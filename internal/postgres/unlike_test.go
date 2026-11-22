//go:build dbintegration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestUnlike(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// u6 находится справа в паре u4/u6.
	change, err := s.Unlike(ctx, "u6", "u4")
	if err != nil || change != (domain.PairChange{Changed: true}) {
		t.Fatalf("unlike: got %+v, err %v", change, err)
	}

	var leftLiked, rightLiked, active bool
	err = s.Pool.QueryRow(ctx, `
		SELECT p.left_liked, p.right_liked, m.active
		FROM pairs p JOIN matches m USING (left_id, right_id)
		WHERE p.left_id = 'u4' AND p.right_id = 'u6'`).Scan(&leftLiked, &rightLiked, &active)
	if err != nil {
		t.Fatal(err)
	}
	if !leftLiked || rightLiked || active {
		t.Fatalf("after unlike: left=%t, right=%t, active=%t", leftLiked, rightLiked, active)
	}

	change, err = s.Unlike(ctx, "u6", "u4")
	if err != nil || change != (domain.PairChange{}) {
		t.Fatalf("repeat unlike: got %+v, err %v", change, err)
	}

	change, err = s.Unlike(ctx, "u1", "u2")
	if err != nil || change != (domain.PairChange{}) {
		t.Fatalf("no previous like: got %+v, err %v", change, err)
	}

	_, err = s.Unlike(ctx, "u1", "unknown")
	if !errors.Is(err, domain.ErrProfileNotFound) {
		t.Fatalf("missing profile: got %v", err)
	}
}

func TestUnlikeBlockedPair(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Лайк мог быть поставлен до блокировки.
	_, err := s.Pool.Exec(ctx, `UPDATE pairs SET left_liked = true
		WHERE left_id = 'u1' AND right_id = 'u7'`)
	if err != nil {
		t.Fatal(err)
	}

	change, err := s.Unlike(ctx, "u1", "u7")
	if err != nil || change != (domain.PairChange{Changed: true}) {
		t.Fatalf("unlike with block: got %+v, err %v", change, err)
	}

	var liked, blocked bool
	err = s.Pool.QueryRow(ctx, `SELECT left_liked, right_blocked FROM pairs
		WHERE left_id = 'u1' AND right_id = 'u7'`).Scan(&liked, &blocked)
	if err != nil {
		t.Fatal(err)
	}
	if liked || !blocked {
		t.Fatalf("after unlike: liked=%t, blocked=%t", liked, blocked)
	}
}

func TestUnlikeRollback(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	observer, err := s.Pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Release()

	// Запрос к matches упадёт после того, как Unlike уже сбросил лайк.
	_, err = observer.Exec(ctx, `ALTER TABLE matches
		ADD CONSTRAINT test_match_stays_active CHECK (active)`)
	if err != nil {
		t.Fatal(err)
	}

	change, err := s.Unlike(ctx, "u4", "u6")
	var pgErr *pgconn.PgError
	if change != (domain.PairChange{}) || !errors.As(err, &pgErr) || pgErr.ConstraintName != "test_match_stays_active" {
		t.Fatalf("unlike without commit: got %+v, err %v", change, err)
	}

	var leftLiked, rightLiked, active bool
	err = observer.QueryRow(ctx, `
		SELECT p.left_liked, p.right_liked, m.active
		FROM pairs p JOIN matches m USING (left_id, right_id)
		WHERE p.left_id = 'u4' AND p.right_id = 'u6'`).Scan(&leftLiked, &rightLiked, &active)
	if err != nil {
		t.Fatal(err)
	}
	if !leftLiked || !rightLiked || !active {
		t.Fatalf("after rollback: left=%t, right=%t, active=%t", leftLiked, rightLiked, active)
	}
}
