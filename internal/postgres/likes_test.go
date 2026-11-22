//go:build dbintegration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestLikeSequence(t *testing.T) {
	store := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	steps := []struct {
		name        string
		actorID     domain.UserID
		targetID    domain.UserID
		want        domain.LikeDecision
		wantMatches int
	}{
		{
			name:    "right starts",
			actorID: "u2", targetID: "u1",
			want: domain.LikeDecision{
				State:   domain.PairState{RightLiked: true},
				Changed: true,
			},
		},
		{
			name:    "right repeats",
			actorID: "u2", targetID: "u1",
			want: domain.LikeDecision{
				State: domain.PairState{RightLiked: true},
			},
		},
		{
			name:    "left responds",
			actorID: "u1", targetID: "u2",
			want: domain.LikeDecision{
				State: domain.PairState{
					LeftLiked:  true,
					RightLiked: true,
				},
				Changed:    true,
				Matched:    true,
				MatchedNow: true,
			},
			wantMatches: 1,
		},
		{
			name:    "left repeats after match",
			actorID: "u1", targetID: "u2",
			want: domain.LikeDecision{
				State: domain.PairState{
					LeftLiked:  true,
					RightLiked: true,
				},
				Matched: true,
			},
			wantMatches: 1,
		},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			got, err := store.Like(ctx, step.actorID, step.targetID)
			if err != nil {
				t.Fatalf("Like() error = %v", err)
			}
			if got != step.want {
				t.Fatalf("Like() = %+v, want %+v", got, step.want)
			}

			var saved domain.PairState
			err = store.Pool.QueryRow(ctx, `
				SELECT left_liked, right_liked
				FROM pairs
				WHERE left_id = 'u1' AND right_id = 'u2'
			`).Scan(&saved.LeftLiked, &saved.RightLiked)
			if err != nil {
				t.Fatal(err)
			}
			if saved != step.want.State {
				t.Errorf("saved state = %+v, want %+v", saved, step.want.State)
			}

			var count int
			err = store.Pool.QueryRow(ctx, `
				SELECT count(*)
				FROM matches
				WHERE left_id = 'u1' AND right_id = 'u2'
			`).Scan(&count)
			if err != nil {
				t.Fatal(err)
			}
			if count != step.wantMatches {
				t.Errorf("match rows = %d, want %d", count, step.wantMatches)
			}

			matched, err := store.PairMatched(ctx, step.targetID, step.actorID)
			if err != nil || matched != step.want.Matched {
				t.Errorf("PairMatched() = %t, %v; want %t, nil",
					matched, err, step.want.Matched)
			}
		})
	}
}
