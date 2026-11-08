package profileapi_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/profileapi"
)

func newClient(t *testing.T, server *httptest.Server, timeout time.Duration, limit int64) *profileapi.Client {
	t.Helper()
	httpClient := server.Client()
	httpClient.Timeout = timeout
	client, err := profileapi.NewClient(server.URL, httpClient, limit)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestProfileResponse(t *testing.T) {
	const valid = `{"id":"u/2?","active":false}`
	for _, tc := range []struct {
		name string
		body string
		want error
	}{
		{name: "exact limit", body: valid},
		{name: "over limit", body: valid + " ", want: profileapi.ErrResponseTooLarge},
		{name: "malformed", body: "{", want: profileapi.ErrDependency},
		{name: "missing id", body: `{"active":true}`, want: profileapi.ErrDependency},
		{name: "missing active", body: `{"id":"u/2?"}`, want: profileapi.ErrDependency},
		{name: "wrong id", body: `{"id":"u3","active":true}`, want: profileapi.ErrDependency},
		{name: "null", body: "null", want: profileapi.ErrDependency},
		{name: "trailing JSON", body: `{"id":"u/2?","active":true}0`, want: profileapi.ErrDependency},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.EscapedPath() != "/profiles/u%2F2%3F" || r.URL.RawQuery != "" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			client := newClient(t, server, time.Second, int64(len(valid)))

			profile, err := client.GetContext(context.Background(), "u/2?")
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			if tc.want == nil && (profile.ID != "u/2?" || profile.Active) {
				t.Fatalf("unexpected profile %+v", profile)
			}
		})
	}
}

func TestProfileTruncatedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = fmt.Fprint(w, `{"id":"u1","active":true}`)
	}))
	defer server.Close()

	client := newClient(t, server, time.Second, 4096)
	_, err := client.GetContext(context.Background(), "u1")
	if !errors.Is(err, profileapi.ErrDependency) {
		t.Fatalf("error = %v, want dependency failure", err)
	}
}

func TestProfileCancellation(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	watchdog, stopWatchdog := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopWatchdog()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
			close(stopped)
		case <-watchdog.Done():
		}
	}))
	defer server.Close()
	client := newClient(t, server, 10*time.Second, 4096)
	ctx, cancel := context.WithCancel(watchdog)
	defer cancel()
	result := make(chan error, 1)

	go func() {
		_, err := client.GetContext(ctx, "u1")
		result <- err
	}()
	select {
	case <-started:
		cancel()
	case <-watchdog.Done():
		t.Fatal("request did not reach server")
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) || !errors.Is(err, profileapi.ErrDependency) {
			t.Fatalf("error = %v, want wrapped cancellation", err)
		}
	case <-watchdog.Done():
		t.Fatal("client did not stop")
	}
	select {
	case <-stopped:
	case <-watchdog.Done():
		t.Fatal("server did not observe cancellation")
	}
}

func TestProfileServerModes(t *testing.T) {
	profiles := []domain.Profile{{ID: "u1", Active: true}}
	server := httptest.NewServer(profileapi.NewServer(profiles, 2*time.Second, 8192).Handler())
	defer server.Close()
	client := newClient(t, server, 200*time.Millisecond, 4096)

	for _, tc := range []struct {
		mode string
		want error
	}{
		{mode: "ok"},
		{mode: "missing", want: domain.ErrProfileNotFound},
		{mode: "error", want: profileapi.ErrDependency},
		{mode: "slow", want: context.DeadlineExceeded},
		{mode: "large", want: profileapi.ErrResponseTooLarge},
		{mode: "ok"},
	} {
		req, err := http.NewRequest(http.MethodPut, server.URL+"/_mode/"+tc.mode, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("switch to %s: HTTP %d", tc.mode, resp.StatusCode)
		}

		profile, err := client.GetContext(context.Background(), "u1")
		if !errors.Is(err, tc.want) {
			t.Fatalf("%s: error = %v, want %v", tc.mode, err, tc.want)
		}
		if tc.want == nil && (profile.ID != "u1" || !profile.Active) {
			t.Fatalf("%s: unexpected profile %+v", tc.mode, profile)
		}
		if tc.mode == "error" && !strings.Contains(err.Error(), "500") {
			t.Fatalf("want upstream status 500, got %v", err)
		}
	}
}
