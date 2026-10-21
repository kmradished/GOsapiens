package integration_test

import (
	"testing"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likes"
	"github.com/kmradished/GOsapiens/internal/matching"
	"github.com/kmradished/GOsapiens/internal/memory"
)

type saveCountingStore struct {
	next  application.PairStore
	saves int
}

func (s *saveCountingStore) Load(
	key domain.PairKey,
) (domain.PairState, bool, error) {
	return s.next.Load(key)
}

func (s *saveCountingStore) Save(
	key domain.PairKey,
	state domain.PairState,
) error {
	s.saves++
	return s.next.Save(key, state)
}

func TestScenario(t *testing.T) {
	profiles := memory.NewProfiles([]domain.Profile{
		{ID: "u1", Active: true},
		{ID: "u2", Active: true},
	})
	checker := likes.NewService(profiles, memory.NewLikeState(nil, nil))
	raw := memory.NewPairStore()
	store := &saveCountingStore{next: raw}
	service := application.NewService(checker, store, matching.DecideLike)
	key, err := domain.NewPairKey("u1", "u2")
	if err != nil {
		t.Fatalf("NewPairKey() error = %v", err)
	}

	steps := []struct {
		actorID   domain.UserID
		targetID  domain.UserID
		want      domain.LikeDecision
		wantSaves int
	}{
		{
			actorID:  "u1",
			targetID: "u2",
			want: domain.LikeDecision{
				State:   domain.PairState{LeftLiked: true},
				Changed: true,
			},
			wantSaves: 1,
		},
		{
			actorID:  "u1",
			targetID: "u2",
			want: domain.LikeDecision{
				State: domain.PairState{LeftLiked: true},
			},
			wantSaves: 1,
		},
		{
			actorID:  "u2",
			targetID: "u1",
			want: domain.LikeDecision{
				State:      domain.PairState{LeftLiked: true, RightLiked: true},
				Changed:    true,
				Matched:    true,
				MatchedNow: true,
			},
			wantSaves: 2,
		},
		{
			actorID:  "u2",
			targetID: "u1",
			want: domain.LikeDecision{
				State:   domain.PairState{LeftLiked: true, RightLiked: true},
				Matched: true,
			},
			wantSaves: 2,
		},
	}

	for i, step := range steps {
		got, err := service.Like(step.actorID, step.targetID)
		if err != nil || got != step.want || store.saves != step.wantSaves {
			t.Fatalf("step %d: Like(%q, %q) = %+v, %v; saves = %d, want %+v and %d saves",
				i+1, step.actorID, step.targetID, got, err,
				store.saves, step.want, step.wantSaves)
		}

		state, found, err := raw.Load(key)
		if err != nil || !found || state != step.want.State {
			t.Fatalf("step %d: stored state = %+v, found = %t, error = %v; want %+v",
				i+1, state, found, err, step.want.State)
		}
	}
}
