package application

import (
	"context"
	"errors"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likes"
)

type storedLikeFake struct {
	decision   domain.LikeDecision
	matched    bool
	err        error
	likeCalls  int
	readCalls  int
	ctx        context.Context
	actorID    domain.UserID
	targetID   domain.UserID
	beforeLike func()
}

func (f *storedLikeFake) Like(
	ctx context.Context,
	actorID, targetID domain.UserID,
) (domain.LikeDecision, error) {
	if f.beforeLike != nil {
		f.beforeLike()
	}
	f.likeCalls++
	f.ctx, f.actorID, f.targetID = ctx, actorID, targetID
	return f.decision, f.err
}

func (f *storedLikeFake) PairMatched(
	ctx context.Context,
	actorID, targetID domain.UserID,
) (bool, error) {
	f.readCalls++
	f.ctx, f.actorID, f.targetID = ctx, actorID, targetID
	return f.matched, f.err
}

func TestServiceWithLikeStoreOrderAndContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checked := false
	check := func(gotCtx context.Context, actorID, targetID domain.UserID) (likes.Result, error) {
		if gotCtx != ctx || actorID != "u2" || targetID != "u1" {
			t.Fatal("Check received wrong context or IDs")
		}
		checked = true
		return likes.Result{Allowed: true}, nil
	}

	want := domain.LikeDecision{
		State:   domain.PairState{RightLiked: true},
		Changed: true,
	}
	store := &storedLikeFake{
		decision: want,
		beforeLike: func() {
			if !checked {
				t.Fatal("store called before profile check")
			}
		},
	}
	service := NewServiceWithLikeStore(check, store)

	got, err := service.LikeContext(ctx, "u2", "u1")
	if err != nil || got != want {
		t.Fatalf("LikeContext() = %+v, %v; want %+v, nil", got, err, want)
	}
	if store.likeCalls != 1 || store.readCalls != 0 ||
		store.ctx != ctx || store.actorID != "u2" || store.targetID != "u1" {
		t.Fatal("store received wrong calls, context or IDs")
	}
}

func TestServiceWithLikeStoreErrors(t *testing.T) {
	profileErr := errors.New("profile service failed")
	databaseErr := errors.New("database failed")

	tests := []struct {
		name      string
		check     likes.Result
		checkErr  error
		storeErr  error
		wantCause error
		wantCalls int
	}{
		{
			name:     "profile error",
			checkErr: profileErr, wantCause: profileErr,
		},
		{
			name:  "profile refusal",
			check: likes.Result{Reason: likes.ReasonInactiveProfile},
		},
		{
			name:     "database error",
			check:    likes.Result{Allowed: true},
			storeErr: databaseErr, wantCause: databaseErr, wantCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := func(context.Context, domain.UserID, domain.UserID) (likes.Result, error) {
				return tt.check, tt.checkErr
			}
			store := &storedLikeFake{
				decision: domain.LikeDecision{Changed: true},
				err:      tt.storeErr,
			}
			service := NewServiceWithLikeStore(check, store)

			got, err := service.LikeContext(context.Background(), "u1", "u2")
			if got != (domain.LikeDecision{}) {
				t.Errorf("failed command returned %+v, want empty decision", got)
			}
			if tt.wantCause != nil {
				if !errors.Is(err, tt.wantCause) {
					t.Errorf("error = %v, want cause %v", err, tt.wantCause)
				}
			} else {
				var denied *LikeDeniedError
				if !errors.As(err, &denied) || denied.Reason != likes.ReasonInactiveProfile {
					t.Errorf("error = %v, want inactive profile refusal", err)
				}
			}
			if store.likeCalls != tt.wantCalls || store.readCalls != 0 {
				t.Errorf("store calls: Like=%d, PairMatched=%d; want %d and 0",
					store.likeCalls, store.readCalls, tt.wantCalls)
			}
		})
	}
}

func TestServiceWithLikeStoreReadsWithoutCheck(t *testing.T) {
	cause := errors.New("read failed")

	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "read error"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			check := func(context.Context, domain.UserID, domain.UserID) (likes.Result, error) {
				t.Fatal("pair reading must not check profiles")
				return likes.Result{}, nil
			}
			store := &storedLikeFake{matched: true}
			if fail {
				store.err = cause
			}
			service := NewServiceWithLikeStore(check, store)

			got, err := service.PairMatchedContext(ctx, "u1", "u2")
			if fail {
				if got || !errors.Is(err, cause) {
					t.Errorf("PairMatchedContext() = %t, %v; want false and cause", got, err)
				}
			} else if err != nil || !got {
				t.Errorf("PairMatchedContext() = %t, %v; want true, nil", got, err)
			}

			if store.readCalls != 1 || store.likeCalls != 0 ||
				store.ctx != ctx || store.actorID != "u1" || store.targetID != "u2" {
				t.Fatal("wrong read calls, context or IDs")
			}
		})
	}
}
