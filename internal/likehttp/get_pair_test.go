package likehttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestGetPair(t *testing.T) {
	tests := []struct {
		name       string
		user       string
		target     string
		matched    bool
		readErr    error
		wantStatus int
		wantCode   string
		wantReads  int
	}{
		{
			name: "no match", user: "u1", target: "u2",
			wantStatus: http.StatusOK, wantReads: 1,
		},
		{
			name: "mutual likes", user: "u1", target: "u2", matched: true,
			wantStatus: http.StatusOK, wantReads: 1,
		},
		{
			name: "missing demo user", target: "u2",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_demo_user",
		},
		{
			name: "invalid demo user", user: "bad", target: "u2",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_demo_user",
		},
		{
			name: "invalid target", user: "u1", target: "bad",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_id",
		},
		{
			name: "pair with self", user: "u1", target: "u1",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_id",
		},
		{
			name: "read failure", user: "u1", target: "u2",
			readErr:    errors.New("store unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
			wantReads:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reads := 0
			readPair := func(actorID, targetID domain.UserID) (bool, error) {
				reads++
				if string(actorID) != tt.user || string(targetID) != tt.target {
					t.Errorf("readPair(%q, %q), want (%q, %q)",
						actorID, targetID, tt.user, tt.target)
				}
				return tt.matched, tt.readErr
			}

			validID := func(id domain.UserID) bool {
				return id == "u1" || id == "u2"
			}
			currentUser := func(r *http.Request) (domain.UserID, bool) {
				id := domain.UserID(r.Header.Get("X-Demo-User"))
				return id, validID(id)
			}

			mux := http.NewServeMux()
			mux.HandleFunc("GET /v1/me/pairs/{targetID}",
				NewGetPairHandler(readPair, currentUser, validID))

			req := httptest.NewRequest(http.MethodGet, "/v1/me/pairs/"+tt.target, nil)
			if tt.user != "" {
				req.Header.Set("X-Demo-User", tt.user)
			}
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s",
					rec.Code, tt.wantStatus, rec.Body.String())
			}
			if reads != tt.wantReads {
				t.Errorf("readPair called %d times, want %d", reads, tt.wantReads)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}

			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON response: %v", err)
			}

			if tt.wantCode != "" {
				message, ok := body["message"].(string)
				if body["code"] != tt.wantCode || !ok || message == "" {
					t.Errorf("error body = %v, want code %q and a message", body, tt.wantCode)
				}
				return
			}

			if body["target_id"] != tt.target || body["matched"] != tt.matched {
				t.Errorf("body = %v, want target_id=%q and matched=%t",
					body, tt.target, tt.matched)
			}
		})
	}
}
