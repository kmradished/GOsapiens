package likehttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likes"
)

func TestPutLike(t *testing.T) {
	dependencyFailure := errors.New("profile service failure")
	tests := []struct {
		name       string
		user       string
		target     string
		decision   domain.LikeDecision
		likeErr    error
		wantStatus int
		wantCode   string
		wantCalls  int
	}{
		{
			name: "first like", user: "u1", target: "u2",
			decision:   domain.LikeDecision{Changed: true},
			wantStatus: http.StatusOK, wantCalls: 1,
		},
		{
			name: "repeat before match", user: "u1", target: "u2",
			wantStatus: http.StatusOK, wantCalls: 1,
		},
		{
			name: "reciprocal like", user: "u2", target: "u1",
			decision:   domain.LikeDecision{Changed: true, Matched: true, MatchedNow: true},
			wantStatus: http.StatusOK, wantCalls: 1,
		},
		{
			name: "repeat after match", user: "u2", target: "u1",
			decision:   domain.LikeDecision{Matched: true},
			wantStatus: http.StatusOK, wantCalls: 1,
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
			name: "self like", user: "u1", target: "u1",
			likeErr:    &application.LikeDeniedError{Reason: likes.ReasonSelfLike},
			wantStatus: http.StatusConflict, wantCode: "self_like", wantCalls: 1,
		},
		{
			name: "inactive profile", user: "u1", target: "u2",
			likeErr:    &application.LikeDeniedError{Reason: likes.ReasonInactiveProfile},
			wantStatus: http.StatusConflict, wantCode: "inactive_profile", wantCalls: 1,
		},
		{
			name: "blocked", user: "u1", target: "u2",
			likeErr:    &application.LikeDeniedError{Reason: likes.ReasonBlocked},
			wantStatus: http.StatusForbidden, wantCode: "blocked", wantCalls: 1,
		},
		{
			name: "profile not found refusal", user: "u1", target: "u2",
			likeErr:    &application.LikeDeniedError{Reason: likes.ReasonProfileNotFound},
			wantStatus: http.StatusNotFound, wantCode: "profile_not_found", wantCalls: 1,
		},
		{
			name: "wrapped profile not found", user: "u1", target: "u2",
			likeErr:    fmt.Errorf("check like: %w", domain.ErrProfileNotFound),
			wantStatus: http.StatusNotFound, wantCode: "profile_not_found", wantCalls: 1,
		},
		{
			name: "dependency failure", user: "u1", target: "u2",
			likeErr:    fmt.Errorf("check like: %w", dependencyFailure),
			wantStatus: http.StatusBadGateway, wantCode: "dependency_unavailable", wantCalls: 1,
		},
		{
			name: "dependency timeout", user: "u1", target: "u2",
			likeErr:    errors.Join(dependencyFailure, context.DeadlineExceeded),
			wantStatus: http.StatusGatewayTimeout, wantCode: "dependency_timeout", wantCalls: 1,
		},
		{
			name: "storage failure", user: "u1", target: "u2",
			likeErr:    errors.New("store unavailable"),
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error", wantCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/v1/me/likes/"+tt.target, nil)
			if tt.user != "" {
				req.Header.Set("X-Demo-User", tt.user)
			}
			requestContext, cancel := context.WithCancel(req.Context())
			defer cancel()
			req = req.WithContext(requestContext)

			calls := 0
			like := func(ctx context.Context, actorID, targetID domain.UserID) (domain.LikeDecision, error) {
				calls++
				if ctx != requestContext {
					t.Error("like did not receive the request context")
				}
				if string(actorID) != tt.user || string(targetID) != tt.target {
					t.Errorf("like(%q, %q), want (%q, %q)",
						actorID, targetID, tt.user, tt.target)
				}
				return tt.decision, tt.likeErr
			}

			validID := func(id domain.UserID) bool {
				return id == "u1" || id == "u2"
			}
			currentUser := func(r *http.Request) (domain.UserID, bool) {
				id := domain.UserID(r.Header.Get("X-Demo-User"))
				return id, validID(id)
			}
			isDependencyError := func(err error) bool {
				return errors.Is(err, dependencyFailure)
			}

			mux := http.NewServeMux()
			mux.HandleFunc("PUT /v1/me/likes/{targetID}",
				NewPutLikeHandler(like, currentUser, validID, isDependencyError))
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s",
					rec.Code, tt.wantStatus, rec.Body.String())
			}
			if calls != tt.wantCalls {
				t.Errorf("like called %d times, want %d", calls, tt.wantCalls)
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

			if len(body) != 3 ||
				body["matched"] != tt.decision.Matched ||
				body["changed"] != tt.decision.Changed ||
				body["matched_now"] != tt.decision.MatchedNow {
				t.Errorf("body = %v, want flags from %+v", body, tt.decision)
			}
		})
	}
}
