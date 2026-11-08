package discoveryhttp_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/discovery/interests"
	"github.com/kmradished/GOsapiens/internal/discoveryhttp"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func currentDemoUser(*http.Request) (domain.UserID, bool) {
	return "u1", true
}

func TestGetDiscoverySuccess(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  application.DiscoveryQuery
		cards []interests.AnonymousCard
		json  string
	}{
		{
			name: "defaults and anonymous response",
			want: application.DiscoveryQuery{Limit: 20},
			cards: []interests.AnonymousCard{{
				CandidateID: "u2", CommonMovies: []string{"Dune"}, CommonMusic: []string{}, CommonBooks: []string{},
			}},
			json: `{"items":[{"candidate_id":"u2","common_movies":["Dune"],"common_music":[],"common_books":[]}],"limit":20,"offset":0}`,
		},
		{
			name:  "trimmed interest and empty array",
			query: "?interest=%20Dune%09&limit=50&offset=7",
			want:  application.DiscoveryQuery{Interest: "Dune", Limit: 50, Offset: 7},
			json:  `{"items":[],"limit":50,"offset":7}`,
		},
		{
			name:  "minimum limit",
			query: "?limit=1&offset=0",
			want:  application.DiscoveryQuery{Limit: 1},
			json:  `{"items":[],"limit":1,"offset":0}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			handler := discoveryhttp.NewGetDiscoveryHandler(func(actor domain.UserID, query application.DiscoveryQuery) ([]interests.AnonymousCard, error) {
				calls++
				if actor != "u1" || query != tt.want {
					t.Errorf("Discover(%q, %+v), want actor u1 and %+v", actor, query, tt.want)
				}
				return tt.cards, nil
			}, currentDemoUser)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/v1/me/discovery"+tt.query, nil))
			assertDiscoveryHTTP(t, res, http.StatusOK)
			if calls != 1 {
				t.Errorf("Discover() calls = %d, want 1", calls)
			}
			var got, want any
			if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tt.json), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("response = %s, want %s", res.Body, tt.json)
			}
		})
	}
}

func TestGetDiscoveryRejectsInvalidQueryBeforeCallingUseCase(t *testing.T) {
	for _, query := range []string{
		"limit=0", "limit=-1", "limit=51", "limit=1.5", "limit=", "limit=NaN", "limit=1&limit=2",
		"offset=-1", "offset=1.5", "offset=", "offset=NaN", "offset=0&offset=1", "offset=999999999999999999999999",
		"interest=", "interest=%20%09", "interest=Dune&interest=Dune", "interest=Dune&%69nterest=Queen",
		"actor_id=u2", "unexpected=1", "interest=%ZZ", "interest=Dune;limit=2",
	} {
		t.Run(query, func(t *testing.T) {
			handler := discoveryhttp.NewGetDiscoveryHandler(unexpectedDiscovery(t), currentDemoUser)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/v1/me/discovery?"+query, nil))
			assertDiscoveryError(t, res, http.StatusBadRequest, "invalid_query")
		})
	}
}

func TestGetDiscoveryRejectsBodyBeforeCallingUseCase(t *testing.T) {
	for _, test := range []struct {
		name          string
		contentLength int64
		wantRead      int
	}{
		{name: "known length", contentLength: 3},
		{name: "streamed body", contentLength: -1, wantRead: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := discoveryhttp.NewGetDiscoveryHandler(unexpectedDiscovery(t), currentDemoUser)
			body := strings.NewReader("abc")
			req := httptest.NewRequest(http.MethodGet, "/v1/me/discovery", io.NopCloser(body))
			req.ContentLength = test.contentLength
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			assertDiscoveryError(t, res, http.StatusBadRequest, "invalid_body")
			if read := 3 - body.Len(); read != test.wantRead {
				t.Errorf("read %d body bytes, want %d", read, test.wantRead)
			}
		})
	}
}

func TestGetDiscoveryRejectsMissingActorBeforeCallingUseCase(t *testing.T) {
	handler := discoveryhttp.NewGetDiscoveryHandler(unexpectedDiscovery(t), func(*http.Request) (domain.UserID, bool) {
		return "", false
	})
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/v1/me/discovery", nil))
	assertDiscoveryError(t, res, http.StatusBadRequest, "invalid_demo_user")
}

func TestGetDiscoveryMapsUseCaseErrors(t *testing.T) {
	const internalDetail = "private storage diagnostic"
	for _, test := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "wrapped missing profile", err: fmt.Errorf("load actor: %w", domain.ErrProfileNotFound), status: http.StatusNotFound, code: "profile_not_found"},
		{name: "storage error", err: errors.New(internalDetail), status: http.StatusInternalServerError, code: "internal_error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := discoveryhttp.NewGetDiscoveryHandler(func(domain.UserID, application.DiscoveryQuery) ([]interests.AnonymousCard, error) {
				return nil, test.err
			}, currentDemoUser)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/v1/me/discovery", nil))
			assertDiscoveryError(t, res, test.status, test.code)
			if strings.Contains(res.Body.String(), internalDetail) {
				t.Errorf("response exposes internal error: %s", res.Body)
			}
		})
	}
}

func unexpectedDiscovery(t *testing.T) func(domain.UserID, application.DiscoveryQuery) ([]interests.AnonymousCard, error) {
	t.Helper()
	return func(domain.UserID, application.DiscoveryQuery) ([]interests.AnonymousCard, error) {
		t.Error("Discover() called for invalid HTTP input")
		return nil, nil
	}
}

func assertDiscoveryHTTP(t *testing.T, res *httptest.ResponseRecorder, status int) {
	t.Helper()
	if res.Code != status || res.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("response = %d %q %s, want %d application/json", res.Code, res.Header().Get("Content-Type"), res.Body, status)
	}
}

func assertDiscoveryError(t *testing.T, res *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	assertDiscoveryHTTP(t, res, status)
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != code || body.Message == "" {
		t.Errorf("response = %s, want code %q and a message", res.Body, code)
	}
}
