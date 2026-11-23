//go:build dbintegration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/domain"
)

var _ application.BlockStore = (*Store)(nil)
var _ application.DiscoveryStore = (*Store)(nil)

type blockState struct {
	leftLiked, rightLiked, leftBlocked, rightBlocked bool
	active, matches                                  int
	pairVersion, matchVersion                        string
}

func readBlockState(t *testing.T, s *Store, actorID, targetID domain.UserID) blockState {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key, err := domain.NewPairKey(actorID, targetID)
	if err != nil {
		t.Fatal(err)
	}
	var state blockState
	err = s.Pool.QueryRow(ctx, `
		SELECT p.left_liked, p.right_liked, p.left_blocked, p.right_blocked,
		       p.xmin::text, COALESCE(m.xmin::text, ''),
		       (SELECT count(*) FROM matches WHERE left_id=$1 AND right_id=$2 AND active),
		       (SELECT count(*) FROM matches WHERE left_id=$1 AND right_id=$2)
		FROM pairs p LEFT JOIN matches m USING (left_id, right_id)
		WHERE p.left_id=$1 AND p.right_id=$2`, key.LeftID, key.RightID).Scan(
		&state.leftLiked, &state.rightLiked, &state.leftBlocked, &state.rightBlocked,
		&state.pairVersion, &state.matchVersion, &state.active, &state.matches)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestBlockPreservesReactionsAndOtherBlock(t *testing.T) {
	for _, actorID := range []domain.UserID{"u4", "u6"} {
		t.Run(string(actorID), func(t *testing.T) {
			s := testStore(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			targetID := domain.UserID("u6")
			if actorID == targetID {
				targetID = "u4"
			}
			change, err := s.Block(ctx, actorID, targetID)
			if err != nil || change != (domain.PairChange{Changed: true}) {
				t.Fatalf("Block() = %+v, %v", change, err)
			}
			first := readBlockState(t, s, actorID, targetID)
			if !first.leftLiked || !first.rightLiked || first.active != 0 || first.matches != 1 ||
				first.leftBlocked != (actorID == "u4") || first.rightBlocked != (actorID == "u6") {
				t.Fatalf("after block: %+v", first)
			}
			change, err = s.Block(ctx, actorID, targetID)
			if err != nil || change != (domain.PairChange{}) {
				t.Fatalf("repeat Block() = %+v, %v", change, err)
			}
			if repeat := readBlockState(t, s, actorID, targetID); repeat != first {
				t.Fatalf("repeat wrote data: before=%+v, after=%+v", first, repeat)
			}
			if _, err := s.Block(ctx, targetID, actorID); err != nil {
				t.Fatal(err)
			}
			both := readBlockState(t, s, actorID, targetID)
			if !both.leftBlocked || !both.rightBlocked || !both.leftLiked || !both.rightLiked || both.active != 0 || both.matches != 1 {
				t.Fatalf("opposite block lost data: %+v", both)
			}
		})
	}
}

func TestBlockNewPairAndInvalidInput(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tc := range []struct {
		actorID, targetID domain.UserID
		wantErr           error
	}{
		{"u1", "u1", domain.ErrInvalidPair},
		{"", "u1", domain.ErrInvalidPair},
		{"u1", "", domain.ErrInvalidPair},
		{"u1", "unknown", domain.ErrProfileNotFound},
		{"unknown", "u1", domain.ErrProfileNotFound},
	} {
		change, err := s.Block(ctx, tc.actorID, tc.targetID)
		if !errors.Is(err, tc.wantErr) || change != (domain.PairChange{}) {
			t.Fatalf("Block(%q, %q) = %+v, %v", tc.actorID, tc.targetID, change, err)
		}
	}
	change, err := s.Block(ctx, "u2", "u1")
	if err != nil || change != (domain.PairChange{Changed: true}) {
		t.Fatalf("Block() = %+v, %v", change, err)
	}
	state := readBlockState(t, s, "u1", "u2")
	if state.leftLiked || state.rightLiked || state.leftBlocked || !state.rightBlocked || state.matches != 0 {
		t.Fatalf("new blocked pair: %+v", state)
	}
	var count int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM pairs`).Scan(&count); err != nil || count != 4 {
		t.Fatalf("invalid commands left rows: count=%d, err=%v", count, err)
	}
}

func TestBlockRollbackAfterSavingFlag(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Отказ возникает на втором изменении: флаг блокировки уже записан в tx.
	_, err := s.Pool.Exec(ctx, `
		CREATE FUNCTION reject_match_change() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'test match failure'; END $$;
		CREATE TRIGGER reject_match_change BEFORE UPDATE ON matches
		FOR EACH ROW EXECUTE FUNCTION reject_match_change()`)
	if err != nil {
		t.Fatal(err)
	}
	observer, err := s.Pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Release()
	before := readBlockState(t, s, "u4", "u6")
	change, err := s.Block(ctx, "u6", "u4")
	if err == nil || change != (domain.PairChange{}) {
		t.Fatalf("failed Block() = %+v, %v", change, err)
	}
	var leftLiked, rightLiked, leftBlocked, rightBlocked, active bool
	err = observer.QueryRow(ctx, `
		SELECT left_liked, right_liked, left_blocked, right_blocked, active
		FROM pairs JOIN matches USING (left_id, right_id)
		WHERE left_id='u4' AND right_id='u6'`).Scan(&leftLiked, &rightLiked, &leftBlocked, &rightBlocked, &active)
	if err != nil || !leftLiked || !rightLiked || leftBlocked || rightBlocked || !active {
		t.Fatalf("partial block visible: likes=%v/%v blocks=%v/%v active=%v err=%v",
			leftLiked, rightLiked, leftBlocked, rightBlocked, active, err)
	}
	if after := readBlockState(t, s, "u4", "u6"); after != before {
		t.Fatalf("rollback changed state: before=%+v, after=%+v", before, after)
	}
	if _, err := s.Pool.Exec(ctx, `DROP TRIGGER reject_match_change ON matches`); err != nil {
		t.Fatal(err)
	}
	if change, err := s.Block(ctx, "u6", "u4"); err != nil || !change.Changed {
		t.Fatalf("block after rollback = %+v, %v", change, err)
	}
}
