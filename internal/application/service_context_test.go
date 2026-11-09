package application

import (
	"context"
	"errors"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likes"
	"github.com/kmradished/GOsapiens/internal/matching"
)

type updateStoreStub struct {
	update updatePairFunc
}

func (s *updateStoreStub) Load(domain.PairKey) (domain.PairState, bool, error) {
	return domain.PairState{}, false, errors.New("unexpected separate Load")
}

func (s *updateStoreStub) Update(
	key domain.PairKey,
	change func(domain.PairState) (domain.LikeDecision, error),
) (domain.LikeDecision, error) {
	return s.update(key, change)
}

func TestLikeContextUsesUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checked := false
	check := func(gotCtx context.Context, actorID, targetID domain.UserID) (likes.Result, error) {
		checked = true
		if gotCtx != ctx || actorID != "u2" || targetID != "u1" {
			t.Fatalf("Check received unexpected context or IDs: %q, %q", actorID, targetID)
		}
		return likes.Result{Allowed: true}, nil
	}

	updates := 0
	store := &updateStoreStub{
		update: func(
			key domain.PairKey,
			change func(domain.PairState) (domain.LikeDecision, error),
		) (domain.LikeDecision, error) {
			updates++
			if !checked {
				t.Fatal("Update called before Check")
			}
			if key != (domain.PairKey{LeftID: "u1", RightID: "u2"}) {
				t.Fatalf("Update received wrong key: %+v", key)
			}
			return change(domain.PairState{LeftLiked: true})
		},
	}

	service := NewServiceWithUpdate(check, store, matching.DecideLike)
	got, err := service.LikeContext(ctx, "u2", "u1")
	want := domain.LikeDecision{
		State:      domain.PairState{LeftLiked: true, RightLiked: true},
		Changed:    true,
		Matched:    true,
		MatchedNow: true,
	}
	if err != nil || got != want || updates != 1 {
		t.Fatalf("LikeContext() = %+v, %v, updates = %d; want %+v, nil, 1",
			got, err, updates, want)
	}
}

func TestLikeContextDoesNotReportSuccessOnUpdateError(t *testing.T) {
	cause := errors.New("could not save pair")
	store := &updateStoreStub{
		update: func(
			domain.PairKey,
			func(domain.PairState) (domain.LikeDecision, error),
		) (domain.LikeDecision, error) {
			return domain.LikeDecision{Changed: true}, cause
		},
	}

	check := func(context.Context, domain.UserID, domain.UserID) (likes.Result, error) {
		return likes.Result{Allowed: true}, nil
	}

	service := NewServiceWithUpdate(check, store, matching.DecideLike)
	got, err := service.LikeContext(context.Background(), "u1", "u2")
	if !errors.Is(err, cause) || got != (domain.LikeDecision{}) {
		t.Fatalf("LikeContext() = %+v, %v; want empty result and original error", got, err)
	}
}
