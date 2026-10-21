package matching

import (
	"errors"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestDecideLikeTransitions(t *testing.T) {
	key := domain.PairKey{
		LeftID:  "u1",
		RightID: "u2",
	}

	empty := domain.PairState{}
	leftOnly := domain.PairState{LeftLiked: true}
	rightOnly := domain.PairState{RightLiked: true}
	both := domain.PairState{
		LeftLiked:  true,
		RightLiked: true,
	}

	tests := []struct {
		name       string
		state      domain.PairState
		actorID    domain.UserID
		wantState  domain.PairState
		changed    bool
		matched    bool
		matchedNow bool
	}{
		{
			name:      "left starts",
			state:     empty,
			actorID:   key.LeftID,
			wantState: leftOnly,
			changed:   true,
		},
		{
			name:      "right starts",
			state:     empty,
			actorID:   key.RightID,
			wantState: rightOnly,
			changed:   true,
		},
		{
			name:       "right answers",
			state:      leftOnly,
			actorID:    key.RightID,
			wantState:  both,
			changed:    true,
			matched:    true,
			matchedNow: true,
		},
		{
			name:       "left answers",
			state:      rightOnly,
			actorID:    key.LeftID,
			wantState:  both,
			changed:    true,
			matched:    true,
			matchedNow: true,
		},
		{
			name:      "left repeats before match",
			state:     leftOnly,
			actorID:   key.LeftID,
			wantState: leftOnly,
		},
		{
			name:      "right repeats before match",
			state:     rightOnly,
			actorID:   key.RightID,
			wantState: rightOnly,
		},
		{
			name:      "left repeats after match",
			state:     both,
			actorID:   key.LeftID,
			wantState: both,
			matched:   true,
		},
		{
			name:      "right repeats after match",
			state:     both,
			actorID:   key.RightID,
			wantState: both,
			matched:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := tt.state

			got, err := DecideLike(key, tt.state, tt.actorID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			want := domain.LikeDecision{
				State:      tt.wantState,
				Changed:    tt.changed,
				Matched:    tt.matched,
				MatchedNow: tt.matchedNow,
			}

			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}

			if tt.state != before {
				t.Errorf(
					"input changed: got %+v, want %+v",
					tt.state,
					before,
				)
			}
		})
	}
}

func TestDecideLikeRejectsForeignActor(t *testing.T) {
	key := domain.PairKey{
		LeftID:  "u1",
		RightID: "u2",
	}

	tests := []struct {
		name  string
		state domain.PairState
	}{
		{
			name: "empty",
		},
		{
			name:  "left liked",
			state: domain.PairState{LeftLiked: true},
		},
		{
			name:  "right liked",
			state: domain.PairState{RightLiked: true},
		},
		{
			name: "already matched",
			state: domain.PairState{
				LeftLiked:  true,
				RightLiked: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := tt.state

			got, err := DecideLike(key, tt.state, "outsider")

			if !errors.Is(err, ErrActorNotInPair) {
				t.Fatalf(
					"got error %v, want %v",
					err,
					ErrActorNotInPair,
				)
			}

			if got != (domain.LikeDecision{}) {
				t.Errorf("got %+v, want empty decision", got)
			}

			if tt.state != before {
				t.Errorf("input state changed")
			}
		})
	}
}

func TestDecideLikeRejectsInvalidPair(t *testing.T) {
	tests := []struct {
		name string
		key  domain.PairKey
	}{
		{
			name: "empty pair",
		},
		{
			name: "empty left",
			key:  domain.PairKey{RightID: "u2"},
		},
		{
			name: "empty right",
			key:  domain.PairKey{LeftID: "u1"},
		},
		{
			name: "same users",
			key: domain.PairKey{
				LeftID:  "u1",
				RightID: "u1",
			},
		},
		{
			name: "wrong order",
			key: domain.PairKey{
				LeftID:  "u2",
				RightID: "u1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecideLike(
				tt.key,
				domain.PairState{},
				"u1",
			)

			if !errors.Is(err, domain.ErrInvalidPair) {
				t.Fatalf(
					"got error %v, want %v",
					err,
					domain.ErrInvalidPair,
				)
			}

			if got != (domain.LikeDecision{}) {
				t.Errorf("got %+v, want empty decision", got)
			}
		})
	}
}
