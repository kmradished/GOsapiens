package webserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kmradished/GOsapiens/internal/webserver"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestDemoUserRejectsInvalidHeaderBeforeHandler(t *testing.T) {
	for _, values := range [][]string{
		nil, {""}, {" u1"}, {"u1 "}, {"u/1"}, {"u\t1"}, {strings.Repeat("u", 65)}, {"u1", "u2"},
	} {
		t.Run(strings.Join(values, ","), func(t *testing.T) {
			called := false
			business := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
			handler := webserver.NewHandler(webserver.Config{CommandTimeout: time.Second}, quietLogger(), business, business, business)
			req := httptest.NewRequest(http.MethodPut, "/v1/me/likes/u2", nil)
			for _, value := range values {
				req.Header.Add("X-Demo-User", value)
			}
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			var body struct{ Code string }
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if called || res.Code != 400 || body.Code != "invalid_demo_user" || res.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("invalid header reached handler: called = %t, status = %d, body = %s", called, res.Code, res.Body)
			}
		})
	}
}

func TestBusinessRoutesReceiveUserAndRequestContext(t *testing.T) {
	for _, tc := range []struct{ method, path string }{
		{http.MethodPut, "/v1/me/likes/u2"},
		{http.MethodGet, "/v1/me/pairs/u2"},
		{http.MethodGet, "/v1/me/discovery"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			called := false
			business := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				actor, ok := webserver.CurrentUser(r)
				if !ok || actor != "u1" {
					t.Fatalf("CurrentUser = %q, %t", actor, ok)
				}
				deadline, ok := r.Context().Deadline()
				if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > time.Second {
					t.Fatal("request has no command budget")
				}
				if tc.path != "/v1/me/discovery" && r.PathValue("targetID") != "u2" {
					t.Fatalf("targetID = %q", r.PathValue("targetID"))
				}
				cancel()
				if r.Context().Err() != context.Canceled {
					t.Fatal("client cancellation did not reach business handler")
				}
				w.WriteHeader(http.StatusNoContent)
			})
			handler := webserver.NewHandler(webserver.Config{CommandTimeout: time.Second}, quietLogger(), business, business, business)
			req := httptest.NewRequest(tc.method, tc.path, nil).WithContext(parent)
			req.Header.Set("X-Demo-User", "u1")
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if !called || res.Code != http.StatusNoContent {
				t.Fatalf("route not connected: called = %t, status = %d", called, res.Code)
			}
		})
	}
}

func TestRequestIDAndLog(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	business := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	handler := webserver.NewHandler(webserver.Config{CommandTimeout: time.Second}, logger, business, business, business)
	decoder := json.NewDecoder(&logs)
	previous := ""
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPut, "/v1/me/likes/u2", nil)
		req.Header.Set("X-Demo-User", "u1")
		req.Header.Set("X-Request-ID", "client-value")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		id := res.Header().Get("X-Request-ID")
		if id == "" || id == previous || id == "client-value" {
			t.Fatalf("server did not create a new request ID: %q", id)
		}
		previous = id
		var entry struct {
			RequestID string `json:"request_id"`
			Method    string `json:"method"`
			Path      string `json:"path"`
			Status    int    `json:"status"`
			Duration  int64  `json:"duration"`
		}
		if err := decoder.Decode(&entry); err != nil {
			t.Fatal(err)
		}
		if entry.RequestID != id || entry.Method != http.MethodPut || entry.Path != "/v1/me/likes/u2" || entry.Status != 202 || entry.Duration < 0 {
			t.Fatalf("incorrect request log: %+v", entry)
		}
	}
}

func TestHealthAndUnknownRoutes(t *testing.T) {
	business := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected business call")
	})
	handler := webserver.NewHandler(webserver.Config{CommandTimeout: time.Second}, quietLogger(), business, business, business)
	for _, tc := range []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, "/healthz", 200},
		{http.MethodGet, "/unknown", 404},
		{http.MethodPost, "/v1/me/likes/u2", 405},
	} {
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
		if res.Code != tc.status || res.Header().Get("X-Request-ID") == "" {
			t.Fatalf("%s %s: status = %d, headers = %v", tc.method, tc.path, res.Code, res.Header())
		}
	}
}
