//go:build dbintegration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestLikeBlocked(t *testing.T) {
	tests := []struct {
		name         string
		leftBlocked  bool
		rightBlocked bool
		alreadyLiked bool
	}{
		{
			name:        "left blocked",
			leftBlocked: true,
		},
		{
			name:         "right blocked",
			rightBlocked: true,
		},
		{
			name:         "repeat after left block",
			leftBlocked:  true,
			alreadyLiked: true,
		},
		{
			name:         "repeat after right block",
			rightBlocked: true,
			alreadyLiked: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := testStore(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := store.Pool.Exec(ctx, `
				INSERT INTO pairs (
					left_id, right_id, left_liked, right_liked,
					left_blocked, right_blocked
				)
				VALUES ('u1', 'u2', $1, false, $2, $3)`,
				tt.alreadyLiked, tt.leftBlocked, tt.rightBlocked,
			)
			if err != nil {
				t.Fatal(err)
			}

			got, err := store.Like(ctx, "u1", "u2")
			if !errors.Is(err, domain.ErrBlocked) {
				t.Fatalf("Like() error = %v, want ErrBlocked", err)
			}
			if got != (domain.LikeDecision{}) {
				t.Errorf("Like() = %+v, want empty decision", got)
			}

			var saved pairRow
			err = store.Pool.QueryRow(ctx, `
				SELECT left_liked, right_liked, left_blocked, right_blocked
				FROM pairs
				WHERE left_id = 'u1' AND right_id = 'u2'
			`).Scan(
				&saved.State.LeftLiked, &saved.State.RightLiked,
				&saved.LeftBlocked, &saved.RightBlocked,
			)
			if err != nil {
				t.Fatal(err)
			}

			want := pairRow{
				State:        domain.PairState{LeftLiked: tt.alreadyLiked},
				LeftBlocked:  tt.leftBlocked,
				RightBlocked: tt.rightBlocked,
			}
			if saved != want {
				t.Errorf("blocked Like changed pair: got %+v, want %+v", saved, want)
			}

			var matches int
			err = store.Pool.QueryRow(ctx, `
				SELECT count(*) FROM matches
				WHERE left_id = 'u1' AND right_id = 'u2'
			`).Scan(&matches)
			if err != nil {
				t.Fatal(err)
			}
			if matches != 0 {
				t.Errorf("blocked Like created %d match rows", matches)
			}
		})
	}
}

func TestLikeRepeatDoesNotUpdateRows(t *testing.T) {
	store := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	readVersions := func() [2]string {
		t.Helper()

		var versions [2]string
		err := store.Pool.QueryRow(ctx, `
			SELECT p.xmin::text, COALESCE(m.xmin::text, '')
			FROM pairs AS p
			LEFT JOIN matches AS m
				ON m.left_id = p.left_id AND m.right_id = p.right_id
			WHERE p.left_id = 'u1' AND p.right_id = 'u2'
		`).Scan(&versions[0], &versions[1])
		if err != nil {
			t.Fatal(err)
		}
		return versions
	}

	steps := []struct {
		name     string
		actorID  domain.UserID
		targetID domain.UserID
		matched  bool
	}{
		{
			name:    "before match",
			actorID: "u1", targetID: "u2",
		},
		{
			name:    "after match",
			actorID: "u2", targetID: "u1",
			matched: true,
		},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			if _, err := store.Like(ctx, step.actorID, step.targetID); err != nil {
				t.Fatal(err)
			}
			before := readVersions()

			got, err := store.Like(ctx, step.actorID, step.targetID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Changed || got.MatchedNow || got.Matched != step.matched {
				t.Errorf("unexpected repeat result: %+v", got)
			}

			after := readVersions()
			if after != before {
				t.Errorf("repeat updated rows: before=%v, after=%v", before, after)
			}
		})
	}
}

func TestLikeReactivatesMatch(t *testing.T) {
	store := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Состояние после отзыва лайка u1:
	// встречный лайк u2 сохранён, прежний мэтч неактивен.
	_, err := store.Pool.Exec(ctx, `
		INSERT INTO pairs (left_id, right_id, left_liked, right_liked)
		VALUES ('u1', 'u2', false, true);

		INSERT INTO matches (left_id, right_id, active)
		VALUES ('u1', 'u2', false);
	`)
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.Like(ctx, "u1", "u2")
	want := domain.LikeDecision{
		State: domain.PairState{
			LeftLiked:  true,
			RightLiked: true,
		},
		Changed:    true,
		Matched:    true,
		MatchedNow: true,
	}
	if err != nil || got != want {
		t.Fatalf("Like() = %+v, %v; want %+v, nil", got, err, want)
	}

	var total, active int
	err = store.Pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE active)
		FROM matches
		WHERE left_id = 'u1' AND right_id = 'u2'
	`).Scan(&total, &active)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || active != 1 {
		t.Errorf("match rows: total=%d, active=%d; want 1 and 1", total, active)
	}

	var saved domain.PairState
	err = store.Pool.QueryRow(ctx, `
		SELECT left_liked, right_liked FROM pairs
		WHERE left_id = 'u1' AND right_id = 'u2'
	`).Scan(&saved.LeftLiked, &saved.RightLiked)
	if err != nil {
		t.Fatal(err)
	}
	if saved != want.State {
		t.Errorf("saved reactions = %+v, want %+v", saved, want.State)
	}
}
