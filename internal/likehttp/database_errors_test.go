package likehttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestRoutesDatabaseErrors(t *testing.T) {
	profileFailure := errors.New("test profile dependency")

	tests := []struct {
		name       string
		cause      error
		wantStatus int
		wantCode   string
		putOnly    bool
	}{
		{
			name: "database timeout", cause: context.DeadlineExceeded,
			wantStatus: 504, wantCode: "database_timeout",
		},
		{
			name: "retryable transaction", cause: domain.ErrTransactionRetryable,
			wantStatus: 503, wantCode: "transaction_retryable",
		},
		{
			name: "unconfirmed commit", cause: domain.ErrCommitUnconfirmed,
			wantStatus: 503, wantCode: "commit_unconfirmed",
		},
		{
			name:       "unconfirmed commit with timeout",
			cause:      errors.Join(domain.ErrCommitUnconfirmed, context.DeadlineExceeded),
			wantStatus: 503, wantCode: "commit_unconfirmed",
		},
		{
			name: "unexpected database error", cause: errors.New("SQL failed"),
			wantStatus: 500, wantCode: "internal_error",
		},
		{
			name: "database block", cause: domain.ErrBlocked,
			wantStatus: 403, wantCode: "blocked", putOnly: true,
		},
		{
			name: "invalid pair", cause: domain.ErrInvalidPair,
			wantStatus: 400, wantCode: "invalid_id", putOnly: true,
		},
		{
			name: "profile absent from database", cause: domain.ErrProfileNotFound,
			wantStatus: 404, wantCode: "profile_not_found", putOnly: true,
		},
		{
			name:       "profile timeout",
			cause:      errors.Join(profileFailure, context.DeadlineExceeded),
			wantStatus: 504, wantCode: "dependency_timeout", putOnly: true,
		},
	}

	for _, method := range []string{http.MethodGet, http.MethodPut} {
		for _, tt := range tests {
			if tt.putOnly && method != http.MethodPut {
				continue
			}

			t.Run(method+"/"+tt.name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				calls := 0
				failure := fmt.Errorf("private implementation detail: %w", tt.cause)

				checkCall := func(gotCtx context.Context, actorID, targetID domain.UserID) {
					calls++
					if gotCtx != ctx || actorID != "u1" || targetID != "u2" {
						t.Fatal("operation received wrong context or IDs")
					}
				}

				readPair := func(gotCtx context.Context, actorID, targetID domain.UserID) (bool, error) {
					checkCall(gotCtx, actorID, targetID)
					return true, failure
				}
				like := func(gotCtx context.Context, actorID, targetID domain.UserID) (domain.LikeDecision, error) {
					checkCall(gotCtx, actorID, targetID)
					return domain.LikeDecision{Changed: true, Matched: true}, failure
				}
				currentUser := func(*http.Request) (domain.UserID, bool) {
					return "u1", true
				}
				validID := func(id domain.UserID) bool { return id == "u2" }

				mux := http.NewServeMux()
				mux.HandleFunc("GET /v1/me/pairs/{targetID}",
					NewGetPairHandlerContext(readPair, currentUser, validID))
				mux.HandleFunc("PUT /v1/me/likes/{targetID}",
					NewPutLikeHandler(like, currentUser, validID, func(err error) bool {
						return errors.Is(err, profileFailure)
					}))

				path := "/v1/me/pairs/u2"
				if method == http.MethodPut {
					path = "/v1/me/likes/u2"
				}
				req := httptest.NewRequest(method, path, nil).WithContext(ctx)
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, req)

				if rec.Code != tt.wantStatus {
					t.Fatalf("status=%d, want %d; body=%s",
						rec.Code, tt.wantStatus, rec.Body.String())
				}
				if calls != 1 {
					t.Errorf("operation called %d times, want 1", calls)
				}
				if rec.Header().Get("Content-Type") != "application/json" {
					t.Error("response must be JSON")
				}

				var body map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				message, ok := body["message"].(string)
				if len(body) != 2 || body["code"] != tt.wantCode || !ok || message == "" {
					t.Errorf("unexpected error body: %v", body)
				}
				if strings.Contains(rec.Body.String(), "private implementation detail") {
					t.Error("response exposed internal error details")
				}
			})
		}
	}
}

func TestGetPairHandlerContextSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	calls := 0
	readPair := func(gotCtx context.Context, actorID, targetID domain.UserID) (bool, error) {
		calls++
		if gotCtx != ctx || actorID != "u1" || targetID != "u2" {
			t.Fatal("read received wrong context or IDs")
		}
		return true, nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/me/pairs/{targetID}", NewGetPairHandlerContext(
		readPair,
		func(*http.Request) (domain.UserID, bool) { return "u1", true },
		func(id domain.UserID) bool { return id == "u2" },
	))
	req := httptest.NewRequest(http.MethodGet, "/v1/me/pairs/u2", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body pairResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || calls != 1 || body.TargetID != "u2" || !body.Matched {
		t.Fatalf("unexpected response: status=%d, calls=%d, body=%+v",
			rec.Code, calls, body)
	}
}
