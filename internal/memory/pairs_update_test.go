package memory_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/matching"
	"github.com/kmradished/GOsapiens/internal/memory"
)

func TestPairStoreUpdateScenario(t *testing.T) {
	store := memory.NewPairStore()
	key := domain.PairKey{LeftID: "u1", RightID: "u2"}
	steps := []struct {
		actor  domain.UserID
		want   domain.LikeDecision
		writes int
	}{
		{"u1", domain.LikeDecision{State: domain.PairState{LeftLiked: true}, Changed: true}, 1},
		{"u1", domain.LikeDecision{State: domain.PairState{LeftLiked: true}}, 1},
		{"u2", domain.LikeDecision{
			State:   domain.PairState{LeftLiked: true, RightLiked: true},
			Changed: true, Matched: true, MatchedNow: true,
		}, 2},
		{"u2", domain.LikeDecision{
			State: domain.PairState{LeftLiked: true, RightLiked: true}, Matched: true,
		}, 2},
	}
	for i, step := range steps {
		got, err := store.Update(key, func(state domain.PairState) (domain.LikeDecision, error) {
			return matching.DecideLike(key, state, step.actor)
		})
		if err != nil || got != step.want || store.Writes() != step.writes {
			t.Fatalf("step %d: decision = %+v, err = %v, writes = %d; want %+v, %d writes",
				i, got, err, store.Writes(), step.want, step.writes)
		}
		state, found, err := store.Load(key)
		if err != nil || !found || state != step.want.State {
			t.Fatalf("step %d: Load = %+v, %t, %v", i, state, found, err)
		}
	}
}

func TestPairStoreUpdateErrorDoesNotSave(t *testing.T) {
	store := memory.NewPairStore()
	key := domain.PairKey{LeftID: "u1", RightID: "u2"}
	want := domain.PairState{LeftLiked: true}
	if err := store.Save(key, want); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("rule failed")
	got, err := store.Update(key, func(state domain.PairState) (domain.LikeDecision, error) {
		return domain.LikeDecision{State: domain.PairState{RightLiked: true}, Changed: true},
			fmt.Errorf("decide: %w", cause)
	})
	if !errors.Is(err, cause) || got != (domain.LikeDecision{}) {
		t.Fatalf("Update = %+v, %v; want zero decision and cause", got, err)
	}
	state, found, err := store.Load(key)
	if err != nil || !found || state != want || store.Writes() != 1 {
		t.Fatalf("failed Update changed data: state = %+v, found = %t, err = %v, writes = %d",
			state, found, err, store.Writes())
	}

	called := false
	got, err = store.Update(domain.PairKey{}, func(domain.PairState) (domain.LikeDecision, error) {
		called = true
		return domain.LikeDecision{}, nil
	})
	if !errors.Is(err, domain.ErrInvalidPair) || got != (domain.LikeDecision{}) || called || store.Writes() != 1 {
		t.Fatalf("invalid Update = %+v, %v; called = %t, writes = %d", got, err, called, store.Writes())
	}
}

func TestPairStoreUpdateNoOpDoesNotCreatePair(t *testing.T) {
	store := memory.NewPairStore()
	key := domain.PairKey{LeftID: "u1", RightID: "u2"}
	_, err := store.Update(key, func(domain.PairState) (domain.LikeDecision, error) {
		return domain.LikeDecision{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	state, found, err := store.Load(key)
	if err != nil || found || state != (domain.PairState{}) || store.Writes() != 0 {
		t.Fatalf("no-op created a pair: %+v, %t, %v, writes = %d", state, found, err, store.Writes())
	}
}

func TestPairStoreLikedIDs(t *testing.T) {
	store := memory.NewPairStore()
	seed := map[domain.PairKey]domain.PairState{
		{LeftID: "u1", RightID: "u2"}: {LeftLiked: true},
		{LeftID: "u1", RightID: "u3"}: {RightLiked: true},
		{LeftID: "u0", RightID: "u1"}: {LeftLiked: true, RightLiked: true},
	}
	for key, state := range seed {
		if err := store.Save(key, state); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := store.LikedIDs("u1")
	want := []domain.UserID{"u0", "u2"}
	if err != nil || !reflect.DeepEqual(ids, want) {
		t.Fatalf("LikedIDs = %v, %v; want %v", ids, err, want)
	}
	ids[0] = "changed outside store"
	ids, err = store.LikedIDs("u1")
	if err != nil || !reflect.DeepEqual(ids, want) {
		t.Fatalf("returned slice changed storage: %v, %v", ids, err)
	}
	ids, err = store.LikedIDs("missing")
	if err != nil || ids == nil || len(ids) != 0 {
		t.Fatalf("empty LikedIDs = %v, %v; want non-nil empty slice", ids, err)
	}
}

func TestPairStoreConcurrentLikes(t *testing.T) {
	store := memory.NewPairStore()
	key := domain.PairKey{LeftID: "u1", RightID: "u2"}
	const requests = 64
	type result struct {
		decision domain.LikeDecision
		err      error
	}
	start := make(chan struct{})
	results := make(chan result, requests)
	for i := 0; i < requests; i++ {
		actor := key.LeftID
		if i%2 == 1 {
			actor = key.RightID
		}
		go func(actor domain.UserID) {
			<-start
			decision, err := store.Update(key, func(state domain.PairState) (domain.LikeDecision, error) {
				return matching.DecideLike(key, state, actor)
			})
			// Чтения идут одновременно с остальными обновлениями.
			if err == nil {
				_, _, err = store.Load(key)
			}
			if err == nil {
				_, err = store.LikedIDs(actor)
			}
			results <- result{decision, err}
		}(actor)
	}
	close(start)

	changed, matchedNow := 0, 0
	for i := 0; i < requests; i++ {
		got := <-results
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.decision.Changed {
			changed++
		}
		if got.decision.MatchedNow {
			matchedNow++
		}
	}
	state, found, err := store.Load(key)
	if err != nil || !found || state != (domain.PairState{LeftLiked: true, RightLiked: true}) {
		t.Fatalf("lost Like: state = %+v, found = %t, err = %v", state, found, err)
	}
	if changed != 2 || matchedNow != 1 || store.Writes() != 2 {
		t.Fatalf("changed = %d, matchedNow = %d, writes = %d; want 2, 1, 2",
			changed, matchedNow, store.Writes())
	}
}
