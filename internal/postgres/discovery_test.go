//go:build dbintegration

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/discovery/available"
	"github.com/kmradished/GOsapiens/internal/discoveryhttp"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestDiscoveryReaders(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	profiles, err := s.Profiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var ids []domain.UserID
	for _, profile := range profiles {
		ids = append(ids, profile.ID)
	}
	if !slices.Equal(ids, []domain.UserID{"u1", "u6", "u2", "u3", "u5", "u4", "u7"}) {
		t.Fatalf("profile order = %v", ids)
	}
	for _, tc := range []struct {
		actorID domain.UserID
		likes   []domain.UserID
		blocks  []domain.UserID
	}{
		{"u1", nil, []domain.UserID{"u7"}},
		{"u7", nil, []domain.UserID{"u1"}},
		{"u2", []domain.UserID{"u5"}, nil},
		{"u5", nil, nil},
		{"u4", []domain.UserID{"u6"}, nil},
		{"u6", []domain.UserID{"u4"}, nil},
	} {
		liked, err := s.LikedIDs(ctx, tc.actorID)
		if err != nil || liked == nil || !slices.Equal(liked, tc.likes) {
			t.Fatalf("LikedIDs(%q) = %v, %v", tc.actorID, liked, err)
		}
		blocked, err := s.BlockedIDs(ctx, tc.actorID)
		if err != nil || blocked == nil || !slices.Equal(blocked, tc.blocks) {
			t.Fatalf("BlockedIDs(%q) = %v, %v", tc.actorID, blocked, err)
		}
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := s.Profiles(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("Profiles canceled: %v", err)
	}
	if _, err := s.LikedIDs(canceled, "u1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("LikedIDs canceled: %v", err)
	}
	if _, err := s.BlockedIDs(canceled, "u1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("BlockedIDs canceled: %v", err)
	}
}

func TestDiscoveryUsesCurrentDatabaseState(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	exclusions := available.NewMemoryExclusions(nil, nil, nil)
	service := application.NewDiscoveryServiceWithStore(s, exclusions)
	check := func(query application.DiscoveryQuery, want ...domain.UserID) {
		t.Helper()
		cards, err := service.DiscoverContext(ctx, "u1", query)
		if err != nil || cards == nil {
			t.Fatalf("Discover() = %v, %v", cards, err)
		}
		ids := make([]domain.UserID, 0, len(cards))
		for _, card := range cards {
			ids = append(ids, card.CandidateID)
		}
		if !slices.Equal(ids, want) {
			t.Fatalf("Discover(%+v) = %v, want %v", query, ids, want)
		}
	}
	page := application.DiscoveryQuery{Interest: "Dune", Limit: 2}
	check(page, "u6", "u2")
	if _, err := s.Block(ctx, "u6", "u1"); err != nil {
		t.Fatal(err)
	}
	check(page, "u2", "u5")
	if _, err := s.Block(ctx, "u1", "u2"); err != nil {
		t.Fatal(err)
	}
	check(page, "u5", "u4")
	// Меняем фикстуры через другое соединение: снимок не кэшируется в сервисе.
	_, err := s.Pool.Exec(ctx, `
		UPDATE profiles SET active=false WHERE id='u5';
		UPDATE profiles SET music=ARRAY['Queen'], books=ARRAY['1984'] WHERE id='u1';
		UPDATE profiles SET name='Private name', bio='Private bio', photo_url='private.jpg',
		                    music=ARRAY['Queen'], books=ARRAY['1984'] WHERE id='u4'`)
	if err != nil {
		t.Fatal(err)
	}
	check(page, "u4")
	check(application.DiscoveryQuery{Interest: "dune", Limit: 20})
	check(application.DiscoveryQuery{Interest: "Dune", Limit: 2, Offset: 1})
	// Новый профиль и входящий Like не должны скрывать доступного кандидата.
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO profiles (id, name, movies, position) VALUES ('u8', 'u8', ARRAY['Dune'], 8);
		INSERT INTO pairs (left_id, right_id, right_liked) VALUES ('u1', 'u8', true)`)
	if err != nil {
		t.Fatal(err)
	}
	check(page, "u4", "u8")
	if _, err := s.Pool.Exec(ctx, `UPDATE pairs SET left_liked=true WHERE left_id='u1' AND right_id='u8'`); err != nil {
		t.Fatal(err)
	}
	check(page, "u4")
	if cards, err := service.DiscoverContext(ctx, "missing", page); cards != nil || !errors.Is(err, domain.ErrProfileNotFound) {
		t.Fatalf("missing actor = %v, %v", cards, err)
	}
	currentUser := func(*http.Request) (domain.UserID, bool) { return "u1", true }
	handler := discoveryhttp.NewGetDiscoveryContextHandler(service.DiscoverContext, currentUser)
	for _, interest := range []string{"Dune", "Queen", "1984"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/me/discovery?interest="+interest, nil).WithContext(ctx)
		handler.ServeHTTP(response, request)
		var body struct {
			Items []map[string]any `json:"items"`
		}
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &body) != nil || len(body.Items) != 1 {
			t.Fatalf("HTTP discovery: %d %s", response.Code, response.Body)
		}
		if len(body.Items[0]) != 4 || strings.Contains(response.Body.String(), "Private") || strings.Contains(response.Body.String(), "private.jpg") {
			t.Fatalf("profile leaked: %s", response.Body)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/me/discovery?offset=20", nil).WithContext(ctx))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"items":[]`) {
		t.Fatalf("empty page: %d %s", response.Code, response.Body)
	}
}
