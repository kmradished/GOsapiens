//go:build dbintegration

package postgres

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

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/discoveryhttp"
	"github.com/kmradished/GOsapiens/internal/likehttp"
	"github.com/kmradished/GOsapiens/internal/webserver"
)

func TestBlockHTTPAndDiscovery(t *testing.T) {
	s := testStore(t)
	newHandler := func(s *Store) http.Handler {
		discovery := application.NewDiscoveryServiceWithStore(s, nil)
		getDiscovery := discoveryhttp.NewGetDiscoveryContextHandler(discovery.DiscoverContext, webserver.CurrentUser)
		putBlock := likehttp.NewPutBlockHandler(s.Block, webserver.CurrentUser, webserver.ValidID)
		return webserver.NewHandlerWithBlock(webserver.Config{CommandTimeout: time.Second},
			slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler(), http.NotFoundHandler(), getDiscovery, putBlock)
	}
	handler := newHandler(s)
	request := func(method, path, actor, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("X-Demo-User", actor)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Header().Get("X-Request-ID") == "" {
			t.Fatal("missing request ID")
		}
		return response
	}
	response := request(http.MethodGet, "/v1/me/discovery?limit=1", "u1", "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"candidate_id":"u6"`) {
		t.Fatalf("initial discovery: %d %s", response.Code, response.Body)
	}
	response = request(http.MethodPut, "/v1/me/blocks/u6", "u1", `{"actor_id":"u4"}`)
	if response.Code != 400 {
		t.Fatalf("actor override: %d %s", response.Code, response.Body)
	}
	response = request(http.MethodPut, "/v1/me/blocks/u6", "u1", "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"changed":true`) {
		t.Fatalf("block: %d %s", response.Code, response.Body)
	}
	// Новые Store и обработчики читают результат из прежней БД.
	handler = newHandler(New(s.Pool))
	response = request(http.MethodPut, "/v1/me/blocks/u6", "u1", "")
	if response.Code != 200 || !bytes.Equal(bytes.TrimSpace(response.Body.Bytes()), []byte(`{"changed":false,"matched":false}`)) {
		t.Fatalf("repeated block: %d %s", response.Code, response.Body)
	}
	response = request(http.MethodGet, "/v1/me/discovery?limit=1", "u1", "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"candidate_id":"u2"`) {
		t.Fatalf("discovery after block: %d %s", response.Code, response.Body)
	}
	response = request(http.MethodPut, "/v1/me/blocks/u4", "u6", "")
	var result map[string]bool
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &result) != nil || !result["changed"] || result["matched"] {
		t.Fatalf("block after match: %d %s", response.Code, response.Body)
	}
	state := readBlockState(t, s, "u4", "u6")
	if state.leftBlocked || !state.rightBlocked || !state.leftLiked || !state.rightLiked || state.active != 0 || state.matches != 1 {
		t.Fatalf("HTTP block changed wrong side: %+v", state)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if s.Pool.Stat().AcquiredConns() != 0 {
		t.Fatal("HTTP handlers leaked a connection")
	}
	if err := s.Pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
}
