package likehttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestDeleteLikeHandler(t *testing.T) {
	tests := []struct {
		name, user, target, query, body string
		changed                         bool
		storeErr                        error
		status                          int
		code                            string
		calls                           int
	}{
		{name: "unlike", user: "u1", target: "u2", changed: true, status: 200, calls: 1},
		{name: "repeat", user: "u1", target: "u2", status: 200, calls: 1},
		{name: "missing actor", target: "u2", status: 400, code: "invalid_demo_user"},
		{name: "invalid target", user: "u1", target: "!", status: 400, code: "invalid_id"},
		{name: "self unlike", user: "u1", target: "u1", status: 400, code: "invalid_id"},
		{name: "query", user: "u1", target: "u2", query: "?x=1", status: 400, code: "invalid_query"},
		{name: "body", user: "u1", target: "u2", body: "{}", status: 400, code: "invalid_body"},
		{name: "missing profile", user: "u1", target: "u2", storeErr: domain.ErrProfileNotFound, status: 404, code: "profile_not_found", calls: 1},
		{name: "timeout", user: "u1", target: "u2", storeErr: context.DeadlineExceeded, status: 504, code: "database_timeout", calls: 1},
		{name: "retryable", user: "u1", target: "u2", storeErr: domain.ErrTransactionRetryable, status: 503, code: "transaction_retryable", calls: 1},
		{name: "unconfirmed commit", user: "u1", target: "u2", storeErr: errors.Join(domain.ErrCommitUnconfirmed, context.DeadlineExceeded), status: 503, code: "commit_unconfirmed", calls: 1},
		{name: "internal error", user: "u1", target: "u2", storeErr: errors.New("test database error"), status: 500, code: "internal_error", calls: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			mux := http.NewServeMux()
			mux.HandleFunc("DELETE /v1/me/likes/{targetID}", NewDeleteLikeHandler(
				func(_ context.Context, actorID, targetID domain.UserID) (domain.PairChange, error) {
					calls++
					if actorID != domain.UserID(tc.user) || targetID != domain.UserID(tc.target) {
						t.Fatalf("unexpected IDs: %q, %q", actorID, targetID)
					}
					return domain.PairChange{Changed: tc.changed}, tc.storeErr
				},
				func(r *http.Request) (domain.UserID, bool) {
					id := domain.UserID(r.Header.Get("X-Demo-User"))
					return id, id != ""
				},
				func(id domain.UserID) bool { return id == "u1" || id == "u2" },
			))
			req := httptest.NewRequest(http.MethodDelete, "/v1/me/likes/"+tc.target+tc.query, strings.NewReader(tc.body))
			req.Header.Set("X-Demo-User", tc.user)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.status || calls != tc.calls {
				t.Fatalf("got status %d and %d calls, want %d and %d: %s",
					rec.Code, calls, tc.status, tc.calls, rec.Body)
			}
			if tc.code != "" {
				var response errorResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || response.Code != tc.code {
					t.Fatalf("want code %q, got %s", tc.code, rec.Body)
				}
				return
			}
			want := `{"changed":false,"matched":false}`
			if tc.changed {
				want = `{"changed":true,"matched":false}`
			}
			if got := strings.TrimSpace(rec.Body.String()); got != want {
				t.Fatalf("got %s, want %s", got, want)
			}
		})
	}
}
