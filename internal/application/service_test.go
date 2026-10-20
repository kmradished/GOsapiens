package application

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likes"
)

type fakeLikeChecker struct {
	result likes.Result
	err    error
	calls  int
	actor  domain.UserID
	target domain.UserID
	steps  *[]string
}

func (f *fakeLikeChecker) Check(actor, target domain.UserID) (likes.Result, error) {
	f.calls++
	f.actor, f.target = actor, target
	if f.steps != nil {
		*f.steps = append(*f.steps, "check")
	}
	return f.result, f.err
}

type fakePairStore struct {
	state      domain.PairState
	found      bool
	loadErr    error
	saveErr    error
	loadCalls  int
	saveCalls  int
	loadedKey  domain.PairKey
	savedKey   domain.PairKey
	savedState domain.PairState
	steps      *[]string
}

func (f *fakePairStore) Load(key domain.PairKey) (domain.PairState, bool, error) {
	f.loadCalls++
	f.loadedKey = key
	if f.steps != nil {
		*f.steps = append(*f.steps, "load")
	}
	return f.state, f.found, f.loadErr
}

func (f *fakePairStore) Save(key domain.PairKey, state domain.PairState) error {
	f.saveCalls++
	f.savedKey, f.savedState = key, state
	if f.steps != nil {
		*f.steps = append(*f.steps, "save")
	}
	if f.saveErr != nil {
		return f.saveErr // отказ до изменения сохранённого состояния
	}
	f.state, f.found = state, true
	return nil
}

func TestLikeFirstReactionUsesCanonicalKeyAndEmptyState(t *testing.T) {
	var steps []string
	checker := &fakeLikeChecker{result: likes.Result{Allowed: true}, steps: &steps}
	store := &fakePairStore{
		state: domain.PairState{LeftLiked: true, RightLiked: true},
		found: false,
		steps: &steps,
	}
	wantKey := domain.PairKey{LeftID: "u1", RightID: "u2"}
	want := domain.LikeDecision{
		State:   domain.PairState{RightLiked: true},
		Changed: true,
	}
	decide := func(key domain.PairKey, state domain.PairState, actor domain.UserID) (domain.LikeDecision, error) {
		steps = append(steps, "decide")
		if key != wantKey || state != (domain.PairState{}) || actor != "u2" {
			t.Fatalf("decide received key=%+v state=%+v actor=%q", key, state, actor)
		}
		return want, nil
	}

	got, err := NewService(checker, store, decide).Like("u2", "u1")
	if err != nil || got != want {
		t.Fatalf("Like() = (%+v, %v), want (%+v, nil)", got, err, want)
	}
	if checker.calls != 1 || checker.actor != "u2" || checker.target != "u1" {
		t.Fatalf("checker calls=%d, actor=%q, target=%q", checker.calls, checker.actor, checker.target)
	}
	if store.loadCalls != 1 || store.loadedKey != wantKey || store.saveCalls != 1 ||
		store.savedKey != wantKey || store.savedState != want.State || store.state != want.State {
		t.Fatalf("store calls/state: %+v", store)
	}
	if !reflect.DeepEqual(steps, []string{"check", "load", "decide", "save"}) {
		t.Fatalf("steps = %v", steps)
	}
}

func TestLikeDeniedBeforeLoad(t *testing.T) {
	checker := &fakeLikeChecker{result: likes.Result{Reason: likes.ReasonBlocked}}
	store := &fakePairStore{}
	decideCalled := false
	decide := func(domain.PairKey, domain.PairState, domain.UserID) (domain.LikeDecision, error) {
		decideCalled = true
		return domain.LikeDecision{}, nil
	}

	got, err := NewService(checker, store, decide).Like("u1", "u2")
	var denied *LikeDeniedError
	if got != (domain.LikeDecision{}) || !errors.As(err, &denied) || denied.Reason != likes.ReasonBlocked {
		t.Fatalf("Like() = (%+v, %v), want zero decision and blocked denial", got, err)
	}
	if store.loadCalls != 0 || store.saveCalls != 0 || decideCalled {
		t.Fatal("denied Like must stop before Load, decide and Save")
	}
}

func TestLikeUnchangedDoesNotSave(t *testing.T) {
	old := domain.PairState{LeftLiked: true, RightLiked: true}
	store := &fakePairStore{state: old, found: true}
	want := domain.LikeDecision{State: old, Matched: true}
	decide := func(_ domain.PairKey, state domain.PairState, _ domain.UserID) (domain.LikeDecision, error) {
		if state != old {
			t.Fatalf("decide state = %+v, want %+v", state, old)
		}
		return want, nil
	}

	got, err := NewService(&fakeLikeChecker{result: likes.Result{Allowed: true}}, store, decide).Like("u1", "u2")
	if err != nil || got != want || store.saveCalls != 0 || store.state != old {
		t.Fatalf("Like() = (%+v, %v), Save calls=%d, state=%+v", got, err, store.saveCalls, store.state)
	}
}

func TestLikeLoadErrorStopsBeforeDecisionAndSave(t *testing.T) {
	loadErr := errors.New("load failed")
	store := &fakePairStore{loadErr: loadErr}
	decideCalled := false
	decide := func(domain.PairKey, domain.PairState, domain.UserID) (domain.LikeDecision, error) {
		decideCalled = true
		return domain.LikeDecision{Changed: true}, nil
	}

	got, err := NewService(&fakeLikeChecker{result: likes.Result{Allowed: true}}, store, decide).Like("u1", "u2")
	if got != (domain.LikeDecision{}) || !errors.Is(err, loadErr) || decideCalled || store.saveCalls != 0 {
		t.Fatalf("Like() = (%+v, %v), decide called=%t, Save calls=%d", got, err, decideCalled, store.saveCalls)
	}
}

func TestLikeSaveErrorPreservesStateAndRetrySucceeds(t *testing.T) {
	saveErr := errors.New("save failed")
	old := domain.PairState{LeftLiked: true}
	store := &fakePairStore{state: old, found: true, saveErr: saveErr}
	want := domain.LikeDecision{
		State:      domain.PairState{LeftLiked: true, RightLiked: true},
		Changed:    true,
		Matched:    true,
		MatchedNow: true,
	}
	decide := func(_ domain.PairKey, state domain.PairState, actor domain.UserID) (domain.LikeDecision, error) {
		if state != old || actor != "u2" {
			t.Fatalf("decide state=%+v actor=%q, want old state and u2", state, actor)
		}
		return want, nil
	}
	service := NewService(&fakeLikeChecker{result: likes.Result{Allowed: true}}, store, decide)

	got, err := service.Like("u2", "u1")
	if got != (domain.LikeDecision{}) || !errors.Is(err, saveErr) || store.saveCalls != 1 || store.state != old {
		t.Fatalf("failed Like() = (%+v, %v), Save calls=%d, state=%+v", got, err, store.saveCalls, store.state)
	}

	store.saveErr = nil
	got, err = service.Like("u2", "u1")
	if err != nil || got != want || store.saveCalls != 2 || store.state != want.State {
		t.Fatalf("retry Like() = (%+v, %v), Save calls=%d, state=%+v", got, err, store.saveCalls, store.state)
	}
}

func TestLikeCheckerAndDecisionErrorsReturnZero(t *testing.T) {
	checkerErr := errors.New("checker failed")
	store := &fakePairStore{}
	got, err := NewService(
		&fakeLikeChecker{err: checkerErr}, store,
		func(domain.PairKey, domain.PairState, domain.UserID) (domain.LikeDecision, error) {
			t.Fatal("decide called after checker error")
			return domain.LikeDecision{}, nil
		},
	).Like("u1", "u2")
	if got != (domain.LikeDecision{}) || !errors.Is(err, checkerErr) || store.loadCalls != 0 || store.saveCalls != 0 {
		t.Fatalf("checker error: decision=%+v err=%v store=%+v", got, err, store)
	}

	decideErr := errors.New("decide failed")
	store = &fakePairStore{}
	got, err = NewService(
		&fakeLikeChecker{result: likes.Result{Allowed: true}}, store,
		func(domain.PairKey, domain.PairState, domain.UserID) (domain.LikeDecision, error) {
			return domain.LikeDecision{Changed: true}, decideErr
		},
	).Like("u1", "u2")
	if got != (domain.LikeDecision{}) || !errors.Is(err, decideErr) || store.saveCalls != 0 {
		t.Fatalf("decide error: decision=%+v err=%v Save calls=%d", got, err, store.saveCalls)
	}
}
