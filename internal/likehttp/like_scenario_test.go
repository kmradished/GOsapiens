package likehttp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likehttp"
	"github.com/kmradished/GOsapiens/internal/likes"
	"github.com/kmradished/GOsapiens/internal/matching"
	"github.com/kmradished/GOsapiens/internal/memory"
)

// scenarioStore даёт тесту Update и считает вызовы Save
// запросы выполняются последовательно.
type scenarioStore struct {
	pairs     *memory.PairStore
	saveCalls int
}

func (s *scenarioStore) Load(key domain.PairKey) (domain.PairState, bool, error) {
	return s.pairs.Load(key)
}

func (s *scenarioStore) Update(
	key domain.PairKey,
	change func(domain.PairState) (domain.LikeDecision, error),
) (domain.LikeDecision, error) {
	state, found, err := s.pairs.Load(key)
	if err != nil {
		return domain.LikeDecision{}, err
	}
	if !found {
		state = domain.PairState{}
	}

	decision, err := change(state)
	if err != nil {
		return domain.LikeDecision{}, err
	}
	if !decision.Changed {
		return decision, nil
	}

	s.saveCalls++
	if err := s.pairs.Save(key, decision.State); err != nil {
		return domain.LikeDecision{}, err
	}
	return decision, nil
}

func TestLikeHTTPScenario(t *testing.T) {
	store := &scenarioStore{pairs: memory.NewPairStore()}
	checkCalls := 0

	check := func(context.Context, domain.UserID, domain.UserID) (likes.Result, error) {
		checkCalls++
		return likes.Result{Allowed: true}, nil
	}
	service := application.NewServiceWithUpdate(check, store, matching.DecideLike)

	validID := func(id domain.UserID) bool {
		return id == "u1" || id == "u2"
	}
	currentUser := func(r *http.Request) (domain.UserID, bool) {
		id := domain.UserID(r.Header.Get("X-Demo-User"))
		return id, validID(id)
	}
	isDependencyError := func(error) bool { return false }

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /v1/me/likes/{targetID}",
		likehttp.NewPutLikeHandler(service.LikeContext, currentUser, validID, isDependencyError))
	mux.HandleFunc("GET /v1/me/pairs/{targetID}",
		likehttp.NewGetPairHandler(service.PairMatched, currentUser, validID))

	steps := []struct {
		name      string
		method    string
		path      string
		user      string
		wantJSON  string
		wantSaves int
	}{
		{
			name: "read before any like", method: http.MethodGet,
			path: "/v1/me/pairs/u2", user: "u1",
			wantJSON: `{"target_id":"u2","matched":false}`, wantSaves: 0,
		},
		{
			name: "right user starts", method: http.MethodPut,
			path: "/v1/me/likes/u1", user: "u2",
			wantJSON: `{"matched":false,"changed":true,"matched_now":false}`, wantSaves: 1,
		},
		{
			name: "repeat first like", method: http.MethodPut,
			path: "/v1/me/likes/u1", user: "u2",
			wantJSON: `{"matched":false,"changed":false,"matched_now":false}`, wantSaves: 1,
		},
		{
			name: "read after one like", method: http.MethodGet,
			path: "/v1/me/pairs/u2", user: "u1",
			wantJSON: `{"target_id":"u2","matched":false}`, wantSaves: 1,
		},
		{
			name: "left user responds", method: http.MethodPut,
			path: "/v1/me/likes/u2", user: "u1",
			wantJSON: `{"matched":true,"changed":true,"matched_now":true}`, wantSaves: 2,
		},
		{
			name: "repeat after match", method: http.MethodPut,
			path: "/v1/me/likes/u1", user: "u2",
			wantJSON: `{"matched":true,"changed":false,"matched_now":false}`, wantSaves: 2,
		},
		{
			name: "left user reads match", method: http.MethodGet,
			path: "/v1/me/pairs/u2", user: "u1",
			wantJSON: `{"target_id":"u2","matched":true}`, wantSaves: 2,
		},
		{
			name: "right user reads match", method: http.MethodGet,
			path: "/v1/me/pairs/u1", user: "u2",
			wantJSON: `{"target_id":"u1","matched":true}`, wantSaves: 2,
		},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			checksBefore := checkCalls
			req := httptest.NewRequest(step.method, step.path, nil)
			req.Header.Set("X-Demo-User", step.user)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}

			var got, want map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("invalid response JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(step.wantJSON), &want); err != nil {
				t.Fatalf("invalid expected JSON: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("body = %v, want %v", got, want)
			}
			if store.saveCalls != step.wantSaves {
				t.Errorf("Save called %d times, want %d", store.saveCalls, step.wantSaves)
			}
			if step.method == http.MethodGet && checkCalls != checksBefore {
				t.Error("GET called the like checker")
			}
		})
	}
}
