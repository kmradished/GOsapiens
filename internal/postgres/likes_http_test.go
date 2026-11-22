//go:build dbintegration

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likehttp"
	"github.com/kmradished/GOsapiens/internal/likes"
	"github.com/kmradished/GOsapiens/internal/memory"
	"github.com/kmradished/GOsapiens/internal/profileapi"
	"github.com/kmradished/GOsapiens/internal/webserver"
)

type postgresLikeHTTP struct {
	store        *Store
	profiles     *profileapi.Server
	profileCalls atomic.Int64
	app          *httptest.Server
	client       *http.Client
}

func newPostgresLikeHTTP(t *testing.T) *postgresLikeHTTP {
	t.Helper()

	f := &postgresLikeHTTP{
		store: testStore(t),
		profiles: profileapi.NewServer([]domain.Profile{
			{ID: "u1", Active: true},
			{ID: "u2", Active: true},
			{ID: "u3", Active: false},
			{ID: "u7", Active: true},
			{ID: "outside_db", Active: true},
		}, 3*time.Second, 8192),
	}

	profileHandler := f.profiles.Handler()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.profileCalls.Add(1)
		profileHandler.ServeHTTP(w, r)
	}))
	t.Cleanup(upstream.Close)

	profileHTTP := upstream.Client()
	profileHTTP.Timeout = 600 * time.Millisecond
	t.Cleanup(profileHTTP.CloseIdleConnections)

	profileClient, err := profileapi.NewClient(upstream.URL, profileHTTP, 4096)
	if err != nil {
		t.Fatal(err)
	}

	// Пустая legacy-проверка не хранит реальных блокировок.
	// Актуальный запрет проверяет Store.Like внутри транзакции.
	checker := likes.NewService(profileClient, memory.NewLikeState(nil, nil))
	service := application.NewServiceWithLikeStore(checker.Check, f.store)

	putLike := likehttp.NewPutLikeHandler(
		service.LikeContext,
		webserver.CurrentUser,
		webserver.ValidID,
		func(err error) bool { return errors.Is(err, profileapi.ErrDependency) },
	)
	getPair := likehttp.NewGetPairHandlerContext(
		service.PairMatchedContext,
		webserver.CurrentUser,
		webserver.ValidID,
	)
	unusedDiscovery := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected discovery request")
		w.WriteHeader(http.StatusInternalServerError)
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := webserver.NewHandler(
		webserver.Config{CommandTimeout: 700 * time.Millisecond},
		logger, putLike, getPair, unusedDiscovery,
	)
	f.app = httptest.NewServer(handler)
	t.Cleanup(f.app.Close)

	f.client = f.app.Client()
	f.client.Timeout = 3 * time.Second
	t.Cleanup(f.client.CloseIdleConnections)
	return f
}

func (f *postgresLikeHTTP) request(
	t *testing.T,
	method, actorID, targetID string,
	wantStatus int,
) map[string]any {
	t.Helper()

	path := "/v1/me/pairs/"
	if method == http.MethodPut {
		path = "/v1/me/likes/"
	}
	req, err := http.NewRequest(method, f.app.URL+path+url.PathEscape(targetID), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Demo-User", actorID)

	res, err := f.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != wantStatus {
		t.Fatalf("%s %s: status=%d, want %d; body=%s",
			method, path+targetID, res.StatusCode, wantStatus, data)
	}
	if res.Header.Get("Content-Type") != "application/json" {
		t.Fatal("response must be JSON")
	}

	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func (f *postgresLikeHTTP) assertCounts(
	t *testing.T,
	actorID, targetID string,
	wantPairs, wantMatches int,
) {
	t.Helper()

	key, err := domain.NewPairKey(domain.UserID(actorID), domain.UserID(targetID))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var pairs, matches, active int
	err = f.store.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM pairs
			 WHERE left_id = $1 AND right_id = $2),
			(SELECT count(*) FROM matches
			 WHERE left_id = $1 AND right_id = $2),
			(SELECT count(*) FROM matches
			 WHERE left_id = $1 AND right_id = $2 AND active)
	`, key.LeftID, key.RightID).Scan(&pairs, &matches, &active)
	if err != nil {
		t.Fatal(err)
	}
	if pairs != wantPairs || matches != wantMatches || active != wantMatches {
		t.Fatalf("DB rows: pairs=%d, matches=%d, active=%d; want %d, %d, %d",
			pairs, matches, active, wantPairs, wantMatches, wantMatches)
	}
}

func TestLikeHTTPPostgresScenario(t *testing.T) {
	f := newPostgresLikeHTTP(t)

	steps := []struct {
		name        string
		method      string
		actorID     string
		targetID    string
		wantJSON    string
		wantPairs   int
		wantMatches int
	}{
		{
			name: "read unknown target", method: http.MethodGet,
			actorID: "u1", targetID: "unknown",
			wantJSON: `{"target_id":"unknown","matched":false}`,
		},
		{
			name: "read before like", method: http.MethodGet,
			actorID: "u1", targetID: "u2",
			wantJSON: `{"target_id":"u2","matched":false}`,
		},
		{
			name: "right starts", method: http.MethodPut,
			actorID: "u2", targetID: "u1",
			wantJSON:  `{"matched":false,"changed":true,"matched_now":false}`,
			wantPairs: 1,
		},
		{
			name: "repeat first like", method: http.MethodPut,
			actorID: "u2", targetID: "u1",
			wantJSON:  `{"matched":false,"changed":false,"matched_now":false}`,
			wantPairs: 1,
		},
		{
			name: "read one like", method: http.MethodGet,
			actorID: "u1", targetID: "u2",
			wantJSON:  `{"target_id":"u2","matched":false}`,
			wantPairs: 1,
		},
		{
			name: "left responds", method: http.MethodPut,
			actorID: "u1", targetID: "u2",
			wantJSON:  `{"matched":true,"changed":true,"matched_now":true}`,
			wantPairs: 1, wantMatches: 1,
		},
		{
			name: "repeat after match", method: http.MethodPut,
			actorID: "u1", targetID: "u2",
			wantJSON:  `{"matched":true,"changed":false,"matched_now":false}`,
			wantPairs: 1, wantMatches: 1,
		},
		{
			name: "left reads match", method: http.MethodGet,
			actorID: "u1", targetID: "u2",
			wantJSON:  `{"target_id":"u2","matched":true}`,
			wantPairs: 1, wantMatches: 1,
		},
		{
			name: "right reads match", method: http.MethodGet,
			actorID: "u2", targetID: "u1",
			wantJSON:  `{"target_id":"u1","matched":true}`,
			wantPairs: 1, wantMatches: 1,
		},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			before := f.profileCalls.Load()
			got := f.request(t, step.method, step.actorID, step.targetID, 200)

			var want map[string]any
			if err := json.Unmarshal([]byte(step.wantJSON), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("response=%v, want %v", got, want)
			}

			wantProfileCalls := int64(0)
			if step.method == http.MethodPut {
				wantProfileCalls = 2
			}
			if delta := f.profileCalls.Load() - before; delta != wantProfileCalls {
				t.Errorf("profile requests=%d, want %d", delta, wantProfileCalls)
			}

			f.assertCounts(t, step.actorID, step.targetID, step.wantPairs, step.wantMatches)
		})
	}
}

func TestLikeHTTPPostgresFailures(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		targetID   string
		wantStatus int
		wantCode   string
		wantPairs  int
	}{
		{
			name: "missing mode", mode: "missing", targetID: "u2",
			wantStatus: 404, wantCode: "profile_not_found",
		},
		{
			name: "dependency error", mode: "error", targetID: "u2",
			wantStatus: 502, wantCode: "dependency_unavailable",
		},
		{
			name: "dependency timeout", mode: "slow", targetID: "u2",
			wantStatus: 504, wantCode: "dependency_timeout",
		},
		{
			name: "oversized response", mode: "large", targetID: "u2",
			wantStatus: 502, wantCode: "dependency_unavailable",
		},
		{
			name: "unknown profile", mode: "ok", targetID: "unknown",
			wantStatus: 404, wantCode: "profile_not_found",
		},
		{
			name: "profile absent from DB", mode: "ok", targetID: "outside_db",
			wantStatus: 404, wantCode: "profile_not_found",
		},
		{
			name: "inactive profile", mode: "ok", targetID: "u3",
			wantStatus: 409, wantCode: "inactive_profile",
		},
		{
			name: "block stored in DB", mode: "ok", targetID: "u7",
			wantStatus: 403, wantCode: "blocked", wantPairs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newPostgresLikeHTTP(t)
			if err := f.profiles.SetMode(tt.mode); err != nil {
				t.Fatal(err)
			}

			body := f.request(t, http.MethodPut, "u1", tt.targetID, tt.wantStatus)
			message, ok := body["message"].(string)
			if len(body) != 2 || body["code"] != tt.wantCode || !ok || message == "" {
				t.Errorf("unexpected error body: %v", body)
			}
			f.assertCounts(t, "u1", tt.targetID, tt.wantPairs, 0)

			// GET не должен зависеть от текущего режима Profile API.
			before := f.profileCalls.Load()
			body = f.request(t, http.MethodGet, "u1", tt.targetID, 200)
			if body["target_id"] != tt.targetID || body["matched"] != false {
				t.Errorf("unexpected GET response: %v", body)
			}
			if f.profileCalls.Load() != before {
				t.Error("GET called Profile API")
			}
			f.assertCounts(t, "u1", tt.targetID, tt.wantPairs, 0)

			if tt.mode != "ok" {
				if err := f.profiles.SetMode("ok"); err != nil {
					t.Fatal(err)
				}
				body = f.request(t, http.MethodPut, "u1", tt.targetID, 200)
				if body["changed"] != true || body["matched"] != false ||
					body["matched_now"] != false {
					t.Errorf("unexpected recovery response: %v", body)
				}
				f.assertCounts(t, "u1", tt.targetID, 1, 0)
			}
		})
	}
}
