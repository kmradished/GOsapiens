package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/discovery/available"
	"github.com/kmradished/GOsapiens/internal/discoveryhttp"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likes"
	"github.com/kmradished/GOsapiens/internal/matching"
	"github.com/kmradished/GOsapiens/internal/memory"
)

type discoveryPairReader struct {
	store *memory.PairStore
	key   domain.PairKey
}

func (r discoveryPairReader) LikedIDs(actorID domain.UserID) ([]domain.UserID, error) {
	state, found, err := r.store.Load(r.key)
	if err != nil || !found {
		return nil, err
	}
	switch {
	case actorID == r.key.LeftID && state.LeftLiked:
		return []domain.UserID{r.key.RightID}, nil
	case actorID == r.key.RightID && state.RightLiked:
		return []domain.UserID{r.key.LeftID}, nil
	default:
		return nil, nil
	}
}

func TestDiscoveryReflectsSavedLikes(t *testing.T) {
	dune := domain.Interests{Movies: []string{"Dune"}}
	profiles := memory.NewProfiles([]domain.Profile{
		{ID: "u1", Active: true, Interests: dune},
		{ID: "u6", Active: true, Interests: dune},
		{ID: "u2", Active: true, Interests: dune},
		{ID: "u3", Active: false, Interests: dune},
		{ID: "u5", Active: true, Interests: dune},
		{ID: "u4", Active: true, Interests: dune},
	})
	key, err := domain.NewPairKey("u1", "u6")
	if err != nil {
		t.Fatal(err)
	}
	store := memory.NewPairStore()
	like := application.NewService(likes.NewService(profiles, memory.NewLikeState(nil, nil)), store, matching.DecideLike)
	discovery := application.NewDiscoveryService(profiles, available.NewMemoryExclusions(nil, nil, nil), discoveryPairReader{store: store, key: key})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/me/discovery", discoveryhttp.NewGetDiscoveryHandler(discovery.Discover, func(r *http.Request) (domain.UserID, bool) {
		actor := domain.UserID(r.Header.Get("X-Demo-User"))
		return actor, actor != ""
	}))

	assertDiscoveryPage(t, mux, "u1", 0, "u6", "u2")
	assertDiscoveryPage(t, mux, "u1", 2, "u5", "u4")
	assertDiscoveryPage(t, mux, "u1", 4)
	if decision, err := like.Like("u1", "u6"); err != nil || !decision.Changed {
		t.Fatalf("first Like() = %+v, %v, want a saved change", decision, err)
	}
	assertDiscoveryPage(t, mux, "u1", 0, "u2", "u5")
	assertDiscoveryPage(t, mux, "u1", 2, "u4")
	assertDiscoveryPage(t, mux, "u6", 0, "u1", "u2")
	if decision, err := like.Like("u6", "u1"); err != nil || !decision.MatchedNow {
		t.Fatalf("reciprocal Like() = %+v, %v, want a new match", decision, err)
	}
	assertDiscoveryPage(t, mux, "u6", 0, "u2", "u5")
}

func assertDiscoveryPage(t *testing.T, mux *http.ServeMux, actor domain.UserID, offset int, want ...domain.UserID) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/me/discovery?interest=Dune&limit=2&offset="+strconv.Itoa(offset), nil)
	req.Header.Set("X-Demo-User", string(actor))
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("GET discovery as %q = %d %s, want 200", actor, res.Code, res.Body)
	}
	var page struct {
		Items []struct {
			CandidateID domain.UserID `json:"candidate_id"`
		} `json:"items"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Items == nil || page.Limit != 2 || page.Offset != offset {
		t.Fatalf("invalid discovery page for %q: %s", actor, res.Body)
	}
	ids := make([]domain.UserID, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.CandidateID)
	}
	if !slices.Equal(ids, want) {
		t.Errorf("discovery for %q at offset %d = %v, want %v", actor, offset, ids, want)
	}
}
