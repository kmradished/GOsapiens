//go:build dbintegration

package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmradished/GOsapiens/internal/domain"
)

type concurrentLikeResult struct {
	decision domain.LikeDecision
	err      error
}

func TestLikeConcurrentOppositeAndRepeats(t *testing.T) {
	store := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	key := domain.PairKey{LeftID: "u1", RightID: "u2"}

	// Временно удерживаем пару, чтобы обе команды начали ждать её в БД.
	gate, err := store.Pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rollback(gate)

	if _, err := lockPair(ctx, gate, key); err != nil {
		t.Fatal(err)
	}

	var clients [2]*Store
	var pids [2]uint32

	for i := range clients {
		cfg := store.Pool.Config()
		cfg.MaxConns = 1

		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		clients[i] = New(pool)

		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		pids[i] = conn.Conn().PgConn().PID()
		conn.Release()
	}

	if pids[0] == pids[1] {
		t.Fatal("expected two different PostgreSQL connections")
	}

	results := make(chan concurrentLikeResult, 2)
	var workers sync.WaitGroup

	// При завершении теста освобождаем блокировку и ждём работников.
	defer func() {
		cancel()
		rollback(gate)

		done := make(chan struct{})
		go func() {
			workers.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("Like workers did not finish")
		}
	}()

	launchLikes := func() {
		actors := [2]domain.UserID{"u1", "u2"}
		targets := [2]domain.UserID{"u2", "u1"}

		for i := range clients {
			workers.Add(1)
			go func(client *Store, actorID, targetID domain.UserID) {
				defer workers.Done()
				decision, err := client.Like(ctx, actorID, targetID)
				results <- concurrentLikeResult{decision: decision, err: err}
			}(clients[i], actors[i], targets[i])
		}
	}

	takeResult := func() concurrentLikeResult {
		t.Helper()
		select {
		case result := <-results:
			return result
		case <-ctx.Done():
			t.Fatalf("waiting for Like result: %v", ctx.Err())
			return concurrentLikeResult{}
		}
	}

	launchLikes()
	waitForLikeConnections(t, ctx, store, pids)

	// Убираем тестовую блокировку. Начальная вставка пары откатывается.
	// Две команды теперь сами должны безопасно создать и изменить пару.
	if err := gate.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	matched, matchedNow := 0, 0
	for i := 0; i < 2; i++ {
		result := takeResult()
		if result.err != nil {
			t.Fatal(result.err)
		}
		if !result.decision.Changed {
			t.Fatalf("new Like did not change state: %+v", result.decision)
		}
		if result.decision.Matched {
			matched++
		}
		if result.decision.MatchedNow {
			matchedNow++
		}
	}
	if matched != 1 || matchedNow != 1 {
		t.Fatalf("matched=%d, matched_now=%d; want 1 and 1", matched, matchedNow)
	}

	var saved domain.PairState
	err = store.Pool.QueryRow(ctx, `
		SELECT left_liked, right_liked FROM pairs
		WHERE left_id = 'u1' AND right_id = 'u2'
	`).Scan(&saved.LeftLiked, &saved.RightLiked)
	if err != nil {
		t.Fatal(err)
	}
	if saved != (domain.PairState{LeftLiked: true, RightLiked: true}) {
		t.Fatalf("concurrent Like lost a reaction: %+v", saved)
	}

	var total, active int
	err = store.Pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE active)
		FROM matches WHERE left_id = 'u1' AND right_id = 'u2'
	`).Scan(&total, &active)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || active != 1 {
		t.Fatalf("matches: total=%d, active=%d; want 1 and 1", total, active)
	}

	readVersions := func() [2]string {
		t.Helper()
		var versions [2]string
		err := store.Pool.QueryRow(ctx, `
			SELECT p.xmin::text, m.xmin::text
			FROM pairs AS p
			JOIN matches AS m
				ON m.left_id = p.left_id AND m.right_id = p.right_id
			WHERE p.left_id = 'u1' AND p.right_id = 'u2'
		`).Scan(&versions[0], &versions[1])
		if err != nil {
			t.Fatal(err)
		}
		return versions
	}

	before := readVersions()

	// Теперь одновременно повторяем оба уже сохранённых лайка.
	launchLikes()
	for i := 0; i < 2; i++ {
		result := takeResult()
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.decision.Changed ||
			result.decision.MatchedNow ||
			!result.decision.Matched {
			t.Errorf("unexpected concurrent repeat: %+v", result.decision)
		}
	}

	after := readVersions()
	if after != before {
		t.Errorf("concurrent repeats updated rows: before=%v, after=%v", before, after)
	}

	for _, client := range clients {
		got, err := client.PairMatched(ctx, "u2", "u1")
		if err != nil || !got {
			t.Errorf("PairMatched() = %t, %v; want true, nil", got, err)
		}
	}
}

func waitForLikeConnections(
	t *testing.T,
	ctx context.Context,
	store *Store,
	pids [2]uint32,
) {
	t.Helper()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		var bothWaiting bool
		err := store.Pool.QueryRow(ctx, `
			SELECT cardinality(pg_blocking_pids($1)) > 0
			   AND cardinality(pg_blocking_pids($2)) > 0
		`, pids[0], pids[1]).Scan(&bothWaiting)
		if err != nil {
			t.Fatal(err)
		}
		if bothWaiting {
			return
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatalf("both Like connections did not wait for the pair: %v", ctx.Err())
		}
	}
}
