package integration_test

import (
	"context"
	"testing"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/demodata"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likes"
	"github.com/kmradished/GOsapiens/internal/matching"
	"github.com/kmradished/GOsapiens/internal/memory"
)

func TestConcurrentLikesAcrossServices(t *testing.T) {
	pairs := memory.NewPairStore()
	checker := likes.NewService(memory.NewProfiles(demodata.Profiles()), memory.NewLikeState(nil, nil))
	services := []*application.Service{
		application.NewServiceWithUpdate(checker.Check, pairs, matching.DecideLike),
		application.NewServiceWithUpdate(checker.Check, pairs, matching.DecideLike),
	}
	const requests = 64
	type result struct {
		decision domain.LikeDecision
		err      error
	}
	start := make(chan struct{})
	results := make(chan result, requests)
	for i := 0; i < requests; i++ {
		service := services[(i/2)%len(services)]
		actor, target := domain.UserID("u1"), domain.UserID("u2")
		if i%2 == 1 {
			actor, target = target, actor
		}
		go func() {
			<-start
			decision, err := service.LikeContext(context.Background(), actor, target)
			results <- result{decision, err}
		}()
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
	key := domain.PairKey{LeftID: "u1", RightID: "u2"}
	state, found, err := pairs.Load(key)
	if err != nil || !found || state != (domain.PairState{LeftLiked: true, RightLiked: true}) {
		t.Fatalf("lost reaction: state = %+v, found = %t, err = %v", state, found, err)
	}
	if changed != 2 || matchedNow != 1 || pairs.Writes() != 2 {
		t.Fatalf("changed = %d, matchedNow = %d, writes = %d; want 2, 1, 2", changed, matchedNow, pairs.Writes())
	}
}
