//go:build dbintegration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestPairMatchedStates(t *testing.T) {
	tests := []struct {
		name     string
		actorID  domain.UserID
		targetID domain.UserID
		setup    string
		want     bool
	}{
		{
			name:    "missing pair",
			actorID: "u1", targetID: "u2",
		},
		{
			name:    "unknown target",
			actorID: "u1", targetID: "unknown",
		},
		{
			name:    "one like",
			actorID: "u2", targetID: "u5",
		},
		{
			name:    "active match",
			actorID: "u4", targetID: "u6",
			want: true,
		},
		{
			name:    "reverse order",
			actorID: "u6", targetID: "u4",
			want: true,
		},
		{
			name:    "inactive match",
			actorID: "u1", targetID: "u2",
			setup: `
				INSERT INTO pairs
					(left_id, right_id, left_liked, right_liked)
				VALUES ('u1', 'u2', true, false);

				INSERT INTO matches (left_id, right_id, active)
				VALUES ('u1', 'u2', false);
			`,
		},
		{
			name:    "blocked pair without match",
			actorID: "u1", targetID: "u7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := testStore(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if tt.setup != "" {
				if _, err := store.Pool.Exec(ctx, tt.setup); err != nil {
					t.Fatal(err)
				}
			}

			got, err := store.PairMatched(ctx, tt.actorID, tt.targetID)
			if err != nil {
				t.Fatalf("PairMatched() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("PairMatched() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestPairMatchedWorksWithReadOnlyConnection(t *testing.T) {
	store := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := store.Pool.Config()
	cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	reader := New(pool)

	matched, err := reader.PairMatched(ctx, "u4", "u6")
	if err != nil || !matched {
		t.Fatalf("read active match = %t, %v; want true, nil", matched, err)
	}

	matched, err = reader.PairMatched(ctx, "u1", "unknown")
	if err != nil || matched {
		t.Fatalf("read missing pair = %t, %v; want false, nil", matched, err)
	}
}

func TestPairMatchedRejectsInvalidPair(t *testing.T) {
	store := &Store{}

	for _, ids := range [][2]domain.UserID{
		{"", "u2"},
		{"u1", ""},
		{"u1", "u1"},
	} {
		got, err := store.PairMatched(context.Background(), ids[0], ids[1])
		if !errors.Is(err, domain.ErrInvalidPair) || got {
			t.Errorf("PairMatched(%q, %q) = %t, %v; want false and ErrInvalidPair",
				ids[0], ids[1], got, err)
		}
	}
}
