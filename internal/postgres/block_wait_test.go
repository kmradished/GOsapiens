//go:build dbintegration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func namedBlockStore(t *testing.T, ctx context.Context, s *Store, name string) (*Store, uint32) {
	t.Helper()
	cfg := s.Pool.Config()
	cfg.MaxConns = 1
	cfg.ConnConfig.RuntimeParams["application_name"] = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := conn.Conn().PgConn().PID()
	conn.Release()
	return New(pool), pid
}

func waitBlockedBy(t *testing.T, ctx context.Context, s *Store, pid, blocker uint32) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := s.Pool.QueryRow(ctx, `SELECT $2::int = ANY(pg_blocking_pids($1))`, pid, blocker).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatalf("backend %d did not wait for %d: %v", pid, blocker, ctx.Err())
		}
	}
}

func TestBlockCanceledWaitReleasesConnection(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	first, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer rollback(first)
	key, err := domain.NewPairKey("u4", "u6")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lockPair(ctx, first, key); err != nil {
		t.Fatal(err)
	}
	second, pid := namedBlockStore(t, ctx, s, "block_canceled_wait")
	waiting, stop := context.WithCancel(ctx)
	defer stop()
	result := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		change, err := second.Block(waiting, "u4", "u6")
		if change != (domain.PairChange{}) {
			err = errors.New("canceled block returned a change")
		}
		result <- err
	}()
	defer func() {
		stop()
		rollback(first)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("canceled block did not finish")
		}
	}()
	waitBlockedBy(t, ctx, s, pid, first.Conn().PgConn().PID())
	stop()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Block() error = %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	state := readBlockState(t, s, "u4", "u6")
	if state.leftBlocked || state.rightBlocked || state.active != 1 || !state.leftLiked || !state.rightLiked {
		t.Fatalf("canceled block changed data: %+v", state)
	}
	rollback(first)
	if change, err := second.Block(ctx, "u4", "u6"); err != nil || !change.Changed {
		t.Fatalf("Block after canceled wait = %+v, %v", change, err)
	}
}
