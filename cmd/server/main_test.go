package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kmradished/GOsapiens/internal/demodata"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/memory"
	"github.com/kmradished/GOsapiens/internal/profileapi"
	"github.com/kmradished/GOsapiens/internal/webserver"
)

type testApp struct {
	server *httptest.Server
	pairs  *memory.PairStore
}

func newTestApp(t *testing.T, profileHandler http.Handler) testApp {
	t.Helper()
	upstream := httptest.NewServer(profileHandler)
	t.Cleanup(upstream.Close)
	cfg, err := webserver.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.ProfileURL = upstream.URL
	client := webserver.NewHTTPClient(cfg)
	t.Cleanup(client.CloseIdleConnections)
	profiles, err := profileapi.NewClient(cfg.ProfileURL, client, cfg.ProfileMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	pairs := memory.NewPairStore()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := newHandler(cfg, logger, profiles, pairs)
	server := httptest.NewUnstartedServer(handler)
	server.Config = webserver.NewHTTPServer(cfg, handler)
	server.Start()
	server.Client().Timeout = 3 * time.Second
	t.Cleanup(server.Close)
	return testApp{server: server, pairs: pairs}
}

func (app testApp) request(t *testing.T, method, path, actor, body string, result any) int {
	t.Helper()
	req, err := http.NewRequest(method, app.server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if actor != "" {
		req.Header.Set("X-Demo-User", actor)
	}
	res, err := app.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.Header.Get("Content-Type") != "application/json" || res.Header.Get("X-Request-ID") == "" {
		t.Fatalf("missing response headers: %v", res.Header)
	}
	if err := json.NewDecoder(res.Body).Decode(result); err != nil {
		t.Fatal(err)
	}
	return res.StatusCode
}

func (app testApp) page(t *testing.T, offset int, want ...domain.UserID) {
	t.Helper()
	var page struct {
		Items []struct {
			CandidateID domain.UserID `json:"candidate_id"`
		}
		Limit  int
		Offset int
	}
	path := "/v1/me/discovery?interest=Dune&limit=2&offset=" + strconv.Itoa(offset)
	status := app.request(t, http.MethodGet, path, "u1", "", &page)
	ids := make([]domain.UserID, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.CandidateID)
	}
	if status != 200 || page.Items == nil || page.Limit != 2 || page.Offset != offset || !slices.Equal(ids, want) {
		t.Fatalf("page at %d: status = %d, page = %+v, IDs = %v; want %v", offset, status, page, ids, want)
	}
}

type likeReply struct {
	Matched    bool `json:"matched"`
	Changed    bool `json:"changed"`
	MatchedNow bool `json:"matched_now"`
}

func TestHTTPScenario(t *testing.T) {
	profiles := profileapi.NewServer(demodata.Profiles(), 2*time.Second, 8192)
	app := newTestApp(t, profiles.Handler())
	app.page(t, 0, "u6", "u2")
	app.page(t, 2, "u5", "u4")
	app.page(t, 4)
	for _, target := range []string{"u6", "unknown"} {
		var reply struct {
			TargetID string `json:"target_id"`
			Matched  bool
		}
		status := app.request(t, http.MethodGet, "/v1/me/pairs/"+target, "u1", "", &reply)
		if status != 200 || reply.TargetID != target || reply.Matched || app.pairs.Writes() != 0 {
			t.Fatalf("initial pair: status = %d, reply = %+v, writes = %d", status, reply, app.pairs.Writes())
		}
	}

	steps := []struct {
		actor, target string
		want          likeReply
		writes        int
	}{
		{"u1", "u6", likeReply{Changed: true}, 1},
		{"u1", "u6", likeReply{}, 1},
		{"u6", "u1", likeReply{Matched: true, Changed: true, MatchedNow: true}, 2},
		{"u6", "u1", likeReply{Matched: true}, 2},
	}
	for i, step := range steps {
		var reply likeReply
		status := app.request(t, http.MethodPut, "/v1/me/likes/"+step.target, step.actor, "", &reply)
		if status != 200 || reply != step.want || app.pairs.Writes() != step.writes {
			t.Fatalf("step %d: status = %d, reply = %+v, writes = %d; want %+v, %d writes",
				i, status, reply, app.pairs.Writes(), step.want, step.writes)
		}
		for _, pair := range [][2]string{{"u1", "u6"}, {"u6", "u1"}} {
			var state struct{ Matched bool }
			status := app.request(t, http.MethodGet, "/v1/me/pairs/"+pair[1], pair[0], "", &state)
			if status != 200 || state.Matched != step.want.Matched || app.pairs.Writes() != step.writes {
				t.Fatalf("read pair: status = %d, matched = %t, writes = %d", status, state.Matched, app.pairs.Writes())
			}
		}
		if i == 0 {
			app.page(t, 0, "u2", "u5")
			app.page(t, 2, "u4")
		}
	}
}

func TestHTTPRejectsInvalidInputAndDeniedLikes(t *testing.T) {
	profiles := profileapi.NewServer(demodata.Profiles(), 2*time.Second, 8192)
	app := newTestApp(t, profiles.Handler())
	for _, tc := range []struct {
		method, path, actor, body string
		status                    int
		code                      string
	}{
		{http.MethodPut, "/v1/me/likes/u2", "", "", 400, "invalid_demo_user"},
		{http.MethodPut, "/v1/me/likes/u%2F2", "u1", "", 400, "invalid_id"},
		{http.MethodPut, "/v1/me/likes/u2?actor=u6", "u1", "", 400, "invalid_query"},
		{http.MethodPut, "/v1/me/likes/u2", "u1", "{}", 400, "invalid_body"},
		{http.MethodGet, "/v1/me/discovery", "u1", "{}", 400, "invalid_body"},
		{http.MethodGet, "/v1/me/discovery?limit=0", "u1", "", 400, "invalid_query"},
		{http.MethodGet, "/v1/me/discovery?offset=-1", "u1", "", 400, "invalid_query"},
		{http.MethodGet, "/v1/me/discovery?limit=2&limit=3", "u1", "", 400, "invalid_query"},
		{http.MethodGet, "/v1/me/pairs/u1", "u1", "", 400, "invalid_id"},
		{http.MethodPut, "/v1/me/likes/u1", "u1", "", 409, "self_like"},
		{http.MethodPut, "/v1/me/likes/u3", "u1", "", 409, "inactive_profile"},
		{http.MethodPut, "/v1/me/likes/u7", "u1", "", 403, "blocked"},
		{http.MethodPut, "/v1/me/likes/unknown", "u1", "", 404, "profile_not_found"},
	} {
		t.Run(tc.path+"/"+tc.code, func(t *testing.T) {
			var reply struct{ Code string }
			status := app.request(t, tc.method, tc.path, tc.actor, tc.body, &reply)
			if status != tc.status || reply.Code != tc.code || app.pairs.Writes() != 0 {
				t.Fatalf("status = %d, code = %s, writes = %d; want %d, %s, 0 writes",
					status, reply.Code, app.pairs.Writes(), tc.status, tc.code)
			}
		})
	}
}

func TestHTTPDependencyFailuresDoNotSave(t *testing.T) {
	profiles := profileapi.NewServer(demodata.Profiles(), 2*time.Second, 8192)
	app := newTestApp(t, profiles.Handler())
	key := domain.PairKey{LeftID: "u1", RightID: "u2"}
	for _, tc := range []struct {
		mode   string
		status int
		code   string
	}{
		{"missing", 404, "profile_not_found"},
		{"error", 502, "dependency_unavailable"},
		{"slow", 504, "dependency_timeout"},
		{"large", 502, "dependency_unavailable"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			if err := profiles.SetMode(tc.mode); err != nil {
				t.Fatal(err)
			}
			var reply struct{ Code string }
			status := app.request(t, http.MethodPut, "/v1/me/likes/u2", "u1", "", &reply)
			if status != tc.status || reply.Code != tc.code {
				t.Fatalf("status = %d, code = %s; want %d, %s", status, reply.Code, tc.status, tc.code)
			}
			state, found, err := app.pairs.Load(key)
			if err != nil || found || state != (domain.PairState{}) || app.pairs.Writes() != 0 {
				t.Fatalf("failed request saved a pair: %+v, %t, %v, writes = %d", state, found, err, app.pairs.Writes())
			}
			// Выдача читает локальные профили и продолжает содержать u2.
			app.page(t, 0, "u6", "u2")
		})
	}
	if err := profiles.SetMode("ok"); err != nil {
		t.Fatal(err)
	}
	var reply likeReply
	status := app.request(t, http.MethodPut, "/v1/me/likes/u2", "u1", "", &reply)
	if status != 200 || reply != (likeReply{Changed: true}) || app.pairs.Writes() != 1 {
		t.Fatalf("recovered Like: status = %d, reply = %+v, writes = %d", status, reply, app.pairs.Writes())
	}
}

func TestHTTPClientCancellationStopsProfileRequest(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	app := newTestApp(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(stopped)
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, app.server.URL+"/v1/me/likes/u2", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Demo-User", "u1")
	result := make(chan error, 1)
	go func() {
		res, err := app.server.Client().Do(req)
		if err == nil {
			_ = res.Body.Close()
		}
		result <- err
	}()
	select {
	case <-started:
		cancel()
	case <-time.After(3 * time.Second):
		t.Fatal("request did not reach Profile API")
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("client error = %v; want cancellation", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("client did not stop")
	}
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation did not reach Profile API")
	}
	if app.pairs.Writes() != 0 {
		t.Fatalf("canceled request saved a pair: writes = %d", app.pairs.Writes())
	}
}
