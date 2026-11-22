package discoveryhttp_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/discovery/interests"
	"github.com/kmradished/GOsapiens/internal/discoveryhttp"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestDiscoveryContextAndDatabaseErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{context.DeadlineExceeded, 504, "database_timeout"},
		{context.Canceled, 504, "database_timeout"},
		{domain.ErrTransactionRetryable, 503, "transaction_retryable"},
	} {
		t.Run(tc.code+tc.err.Error(), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			handler := discoveryhttp.NewGetDiscoveryContextHandler(func(got context.Context, actorID domain.UserID, query application.DiscoveryQuery) ([]interests.AnonymousCard, error) {
				if got != ctx || actorID != "u1" || query.Limit != 20 {
					t.Fatal("request context or query was lost")
				}
				return nil, fmt.Errorf("read database: %w", tc.err)
			}, currentDemoUser)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/me/discovery", nil).WithContext(ctx))
			if response.Code != tc.status || !strings.Contains(response.Body.String(), `"code":"`+tc.code+`"`) {
				t.Fatalf("response = %d %s", response.Code, response.Body)
			}
		})
	}
}
