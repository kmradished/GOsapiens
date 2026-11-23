package likehttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/webserver"
)

func TestPutBlock(t *testing.T) {
	for _, tc := range []struct {
		name, actor, target, query, body string
		change                           domain.PairChange
		err                              error
		status                           int
		code                             string
		calls                            int
	}{
		{name: "block", actor: "u1", target: "u2", change: domain.PairChange{Changed: true}, status: 200, calls: 1},
		{name: "repeat", actor: "u2", target: "u1", status: 200, calls: 1},
		{name: "missing actor", target: "u2", status: 400, code: "invalid_demo_user"},
		{name: "invalid actor", actor: "bad/id", target: "u2", status: 400, code: "invalid_demo_user"},
		{name: "invalid target", actor: "u1", target: "bad/id", status: 400, code: "invalid_id"},
		{name: "self", actor: "u1", target: "u1", status: 400, code: "invalid_id"},
		{name: "actor in query", actor: "u1", target: "u2", query: "?actor_id=u6", status: 400, code: "invalid_query"},
		{name: "actor in body", actor: "u1", target: "u2", body: `{"actor_id":"u6"}`, status: 400, code: "invalid_body"},
		{name: "whitespace body", actor: "u1", target: "u2", body: " ", status: 400, code: "invalid_body"},
		{name: "unknown profile", actor: "u1", target: "u2", err: domain.ErrProfileNotFound, status: 404, code: "profile_not_found", calls: 1},
		{name: "forbidden", actor: "u1", target: "u2", err: domain.ErrForbidden, status: 403, code: "forbidden", calls: 1},
		{name: "invalid pair", actor: "u1", target: "u2", err: domain.ErrInvalidPair, status: 400, code: "invalid_id", calls: 1},
		{name: "retryable", actor: "u1", target: "u2", err: domain.ErrTransactionRetryable, status: 503, code: "transaction_retryable", calls: 1},
		{name: "unconfirmed", actor: "u1", target: "u2", err: domain.ErrCommitUnconfirmed, status: 503, code: "commit_unconfirmed", calls: 1},
		{name: "timeout", actor: "u1", target: "u2", err: context.DeadlineExceeded, status: 504, code: "database_timeout", calls: 1},
		{name: "canceled", actor: "u1", target: "u2", err: context.Canceled, status: 504, code: "database_timeout", calls: 1},
		{name: "internal", actor: "u1", target: "u2", err: errors.New("secret SQL details"), status: 500, code: "internal_error", calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/v1/me/blocks/u2"+tc.query, strings.NewReader(tc.body))
			req.Header.Set("X-Demo-User", tc.actor)
			req.SetPathValue("targetID", tc.target)
			// Неизвестная длина тела тоже не должна обходить проверку.
			req.ContentLength = -1
			calls := 0
			block := func(ctx context.Context, actorID, targetID domain.UserID) (domain.PairChange, error) {
				calls++
				if ctx != req.Context() || string(actorID) != tc.actor || string(targetID) != tc.target {
					t.Fatalf("unexpected block context or IDs: %q -> %q", actorID, targetID)
				}
				if tc.err != nil {
					return domain.PairChange{}, fmt.Errorf("wrapped storage error: %w", tc.err)
				}
				return tc.change, nil
			}
			response := httptest.NewRecorder()
			NewPutBlockHandler(block, blockDemoUser, webserver.ValidID).ServeHTTP(response, req)
			if response.Code != tc.status || calls != tc.calls || response.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("status=%d calls=%d body=%s", response.Code, calls, response.Body)
			}
			if tc.code != "" {
				var body errorResponse
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Code != tc.code || body.Message == "" {
					t.Fatalf("error body: %s, %v", response.Body, err)
				}
				if strings.Contains(response.Body.String(), "secret SQL") {
					t.Fatal("internal error leaked")
				}
			} else {
				var body blockResponse
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Changed != tc.change.Changed || body.Matched {
					t.Fatalf("success body: %s, %v", response.Body, err)
				}
			}
		})
	}
}

type failedBlockBody struct{}

func (failedBlockBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (failedBlockBody) Close() error             { return nil }

func TestPutBlockRejectsUnreadableBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/v1/me/blocks/u2", nil)
	req.Header.Set("X-Demo-User", "u1")
	req.SetPathValue("targetID", "u2")
	req.Body = failedBlockBody{}
	block := func(context.Context, domain.UserID, domain.UserID) (domain.PairChange, error) {
		t.Fatal("invalid request reached storage")
		return domain.PairChange{}, nil
	}
	response := httptest.NewRecorder()
	NewPutBlockHandler(block, blockDemoUser, webserver.ValidID).ServeHTTP(response, req)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
}

func blockDemoUser(r *http.Request) (domain.UserID, bool) {
	id := domain.UserID(r.Header.Get("X-Demo-User"))
	return id, webserver.ValidID(id)
}
