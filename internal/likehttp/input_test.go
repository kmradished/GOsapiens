package likehttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestLikeRoutesInputValidation(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name: "empty input", wantStatus: http.StatusOK,
		},
		{
			name: "empty query marker", query: "?",
			wantStatus: http.StatusOK,
		},
		{
			name: "unexpected body", body: `{"actor":"u2"}`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_body",
		},
		{
			name: "whitespace body", body: " ",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_body",
		},
		{
			name: "unexpected parameter", query: "?actor=u2",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_query",
		},
		{
			name: "repeated parameter", query: "?x=1&x=2",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_query",
		},
		{
			name: "malformed query", query: "?x=%zz",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_query",
		},
	}

	for _, method := range []string{http.MethodGet, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					calls := 0
					readPair := func(domain.UserID, domain.UserID) (bool, error) {
						calls++
						return false, nil
					}
					like := func(context.Context, domain.UserID, domain.UserID) (domain.LikeDecision, error) {
						calls++
						return domain.LikeDecision{}, nil
					}
					currentUser := func(*http.Request) (domain.UserID, bool) {
						return "u1", true
					}
					validID := func(id domain.UserID) bool { return id == "u2" }
					isDependencyError := func(error) bool { return false }

					mux := http.NewServeMux()
					mux.HandleFunc("GET /v1/me/pairs/{targetID}",
						NewGetPairHandler(readPair, currentUser, validID))
					mux.HandleFunc("PUT /v1/me/likes/{targetID}",
						NewPutLikeHandler(like, currentUser, validID, isDependencyError))

					path := "/v1/me/pairs/u2"
					if method == http.MethodPut {
						path = "/v1/me/likes/u2"
					}
					req := httptest.NewRequest(method, path+tc.query, strings.NewReader(tc.body))
					rec := httptest.NewRecorder()

					mux.ServeHTTP(rec, req)

					if rec.Code != tc.wantStatus {
						t.Fatalf("status = %d, want %d; body: %s",
							rec.Code, tc.wantStatus, rec.Body.String())
					}

					wantCalls := 0
					if tc.wantStatus == http.StatusOK {
						wantCalls = 1
					}
					if calls != wantCalls {
						t.Errorf("business operation called %d times, want %d", calls, wantCalls)
					}

					if tc.wantCode != "" {
						var body map[string]any
						if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
							t.Fatalf("invalid error JSON: %v", err)
						}
						if body["code"] != tc.wantCode {
							t.Errorf("error code = %v, want %q", body["code"], tc.wantCode)
						}
					}
				})
			}
		})
	}
}
