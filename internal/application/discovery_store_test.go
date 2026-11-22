package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/discovery/available"
	"github.com/kmradished/GOsapiens/internal/domain"
)

type discoveryStore struct {
	ctx      context.Context
	profiles []domain.Profile
	liked    []domain.UserID
	blocked  []domain.UserID
	failAt   string
	err      error
	t        *testing.T
}

func (s *discoveryStore) check(ctx context.Context, operation string) error {
	s.t.Helper()
	if ctx != s.ctx {
		s.t.Fatal("discovery did not pass request context")
	}
	if s.failAt == operation {
		return s.err
	}
	return nil
}

func (s *discoveryStore) Profiles(ctx context.Context) ([]domain.Profile, error) {
	return s.profiles, s.check(ctx, "profiles")
}

func (s *discoveryStore) LikedIDs(ctx context.Context, actorID domain.UserID) ([]domain.UserID, error) {
	if actorID != "u1" {
		s.t.Fatalf("unexpected actor: %q", actorID)
	}
	return s.liked, s.check(ctx, "likes")
}

func (s *discoveryStore) BlockedIDs(ctx context.Context, actorID domain.UserID) ([]domain.UserID, error) {
	if actorID != "u1" {
		s.t.Fatalf("unexpected actor: %q", actorID)
	}
	return s.blocked, s.check(ctx, "blocks")
}

func TestDiscoveryStoreKeepsRules(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dune := domain.Interests{Movies: []string{"Dune"}}
	store := &discoveryStore{ctx: ctx, t: t, liked: []domain.UserID{"liked"}, blocked: []domain.UserID{"blocked"}}
	for _, id := range []domain.UserID{"u1", "blocked", "viewed", "disliked", "liked", "u6", "music", "u2", "inactive"} {
		profile := domain.Profile{ID: id, Active: id != "inactive", Interests: dune}
		if id == "music" {
			profile.Interests = domain.Interests{Music: []string{"Queen"}}
		}
		store.profiles = append(store.profiles, profile)
	}
	exclusions := available.NewMemoryExclusions(nil,
		[]domain.View{{ViewerID: "u1", CandidateID: "viewed"}},
		[]domain.Reaction{{SenderID: "u1", ReceiverID: "disliked", Kind: domain.ReactionDislike}})
	service := application.NewDiscoveryServiceWithStore(store, exclusions)
	cards, err := service.DiscoverContext(ctx, "u1", application.DiscoveryQuery{Interest: "Dune", Limit: 1, Offset: 1})
	if err != nil || len(cards) != 1 || cards[0].CandidateID != "u2" {
		t.Fatalf("filtered page = %v, %v", cards, err)
	}
	store.liked = append(store.liked, "u6")
	cards, err = service.DiscoverContext(ctx, "u1", application.DiscoveryQuery{Limit: 20})
	if err != nil || len(cards) != 1 || cards[0].CandidateID != "u2" {
		t.Fatalf("current likes = %v, %v", cards, err)
	}
	if !slices.Equal(store.profiles[0].Interests.Movies, []string{"Dune"}) {
		t.Fatal("discovery changed source profiles")
	}
}

func TestDiscoveryStoreErrors(t *testing.T) {
	for _, operation := range []string{"profiles", "blocks", "likes"} {
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			wantErr := errors.New("database read failure")
			store := &discoveryStore{ctx: ctx, t: t, failAt: operation, err: wantErr,
				profiles: []domain.Profile{{ID: "u1"}}}
			service := application.NewDiscoveryServiceWithStore(store, nil)
			cards, err := service.DiscoverContext(ctx, "u1", application.DiscoveryQuery{Limit: 20})
			if cards != nil || !errors.Is(err, wantErr) {
				t.Fatalf("Discover() = %v, %v", cards, err)
			}
		})
	}
	service := application.NewDiscoveryServiceWithStore(&discoveryStore{ctx: context.Background(), t: t}, nil)
	cards, err := service.DiscoverContext(context.Background(), "u1", application.DiscoveryQuery{Limit: 20})
	if cards != nil || !errors.Is(err, domain.ErrProfileNotFound) {
		t.Fatalf("missing actor = %v, %v", cards, err)
	}
}
