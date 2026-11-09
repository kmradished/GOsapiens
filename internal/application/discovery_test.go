package application_test

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/discovery/available"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/memory"
)

type discoveryLikesFunc func(domain.UserID) ([]domain.UserID, error)

func (f discoveryLikesFunc) LikedIDs(actorID domain.UserID) ([]domain.UserID, error) {
	return f(actorID)
}

func TestDiscoveryFiltersBeforePagination(t *testing.T) {
	actor := domain.Profile{
		ID: "u1", Active: true,
		Interests: domain.Interests{Movies: []string{"Dune"}, Music: []string{"Queen"}, Books: []string{"1984"}},
	}
	dune := domain.Interests{Movies: []string{"Dune"}}
	profiles := memory.NewProfiles([]domain.Profile{
		actor,
		{ID: "music", Active: true, Interests: domain.Interests{Music: []string{"Queen", "Dune"}}},
		{ID: "u6", Active: true, Interests: dune},
		{ID: "inactive", Active: false, Interests: dune},
		{ID: "blocked-by-actor", Active: true, Interests: dune},
		{ID: "u2", Active: true, Interests: dune},
		{ID: "blocks-actor", Active: true, Interests: dune},
		{ID: "viewed", Active: true, Interests: dune},
		{ID: "u9", Active: true, Interests: dune},
		{ID: "disliked", Active: true, Interests: dune},
		{ID: "previously-liked", Active: true, Interests: dune},
		{ID: "liked", Active: true, Interests: dune},
		{ID: "u4", Active: true, Interests: dune},
		{ID: "books", Active: true, Interests: domain.Interests{Books: []string{"1984"}}},
		{ID: "unrelated", Active: true, Interests: domain.Interests{Books: []string{"Hamlet"}}},
	})
	exclusions := available.NewMemoryExclusions(
		[]domain.Block{{BlockerID: actor.ID, BlockedID: "blocked-by-actor"}, {BlockerID: "blocks-actor", BlockedID: actor.ID}},
		[]domain.View{{ViewerID: actor.ID, CandidateID: "viewed"}},
		[]domain.Reaction{
			{SenderID: actor.ID, ReceiverID: "disliked", Kind: domain.ReactionDislike},
			{SenderID: actor.ID, ReceiverID: "previously-liked", Kind: domain.ReactionLike},
		},
	)
	likes := discoveryLikesFunc(func(actorID domain.UserID) ([]domain.UserID, error) {
		if actorID != actor.ID {
			t.Errorf("LikedIDs() actor = %q, want %q", actorID, actor.ID)
		}
		return []domain.UserID{"liked"}, nil
	})
	service := application.NewDiscoveryService(profiles, exclusions, likes)

	tests := []struct {
		name  string
		query application.DiscoveryQuery
		want  []domain.UserID
	}{
		{name: "unfiltered preserves profile order", query: application.DiscoveryQuery{Limit: 20}, want: []domain.UserID{"music", "u6", "u2", "u9", "u4", "books"}},
		{name: "first page", query: application.DiscoveryQuery{Interest: "Dune", Limit: 2}, want: []domain.UserID{"u6", "u2"}},
		{name: "second page", query: application.DiscoveryQuery{Interest: "Dune", Limit: 2, Offset: 2}, want: []domain.UserID{"u9", "u4"}},
		{name: "partial page", query: application.DiscoveryQuery{Interest: "Dune", Limit: 2, Offset: 3}, want: []domain.UserID{"u4"}},
		{name: "common music", query: application.DiscoveryQuery{Interest: "Queen", Limit: 2}, want: []domain.UserID{"music"}},
		{name: "common books", query: application.DiscoveryQuery{Interest: "1984", Limit: 2}, want: []domain.UserID{"books"}},
		{name: "case sensitive", query: application.DiscoveryQuery{Interest: "dune", Limit: 2}},
		{name: "end of list", query: application.DiscoveryQuery{Interest: "Dune", Limit: 2, Offset: 4}},
		{name: "offset beyond end without overflow", query: application.DiscoveryQuery{Limit: 50, Offset: math.MaxInt}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cards, err := service.Discover(actor.ID, tt.query)
			if err != nil || cards == nil {
				t.Fatalf("Discover() = %v, %v, want a non-nil page", cards, err)
			}
			ids := make([]domain.UserID, 0, len(cards))
			for _, card := range cards {
				ids = append(ids, card.CandidateID)
			}
			if !slices.Equal(ids, tt.want) {
				t.Errorf("candidate IDs = %v, want %v", ids, tt.want)
			}
		})
	}
}

func TestDiscoveryPropagatesLikesReaderFailure(t *testing.T) {
	wantErr := errors.New("pair store unavailable")
	profiles := memory.NewProfiles([]domain.Profile{{ID: "u1", Active: true}})
	likes := discoveryLikesFunc(func(domain.UserID) ([]domain.UserID, error) {
		return nil, wantErr
	})
	service := application.NewDiscoveryService(profiles, available.NewMemoryExclusions(nil, nil, nil), likes)

	cards, err := service.Discover("u1", application.DiscoveryQuery{Limit: 20})
	if cards != nil || !errors.Is(err, wantErr) {
		t.Fatalf("Discover() = %v, %v, want nil and wrapped %v", cards, err, wantErr)
	}
}
