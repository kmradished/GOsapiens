//go:build dbintegration && likeintegration

package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestBlockBeforeAndAfterLike(t *testing.T) {
	for _, blockFirst := range []bool{true, false} {
		name := "after_like"
		if blockFirst {
			name = "before_like"
		}
		t.Run(name, func(t *testing.T) {
			s := testStore(t)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			if !blockFirst {
				if _, err := s.Like(ctx, "u2", "u1"); err != nil {
					t.Fatal(err)
				}
				decision, err := s.Like(ctx, "u1", "u2")
				if err != nil || !decision.MatchedNow {
					t.Fatalf("reciprocal Like() = %+v, %v", decision, err)
				}
			}
			if change, err := s.Block(ctx, "u1", "u2"); err != nil || !change.Changed || change.Matched {
				t.Fatalf("Block() = %+v, %v", change, err)
			}
			for _, ids := range [][2]domain.UserID{{"u1", "u2"}, {"u2", "u1"}} {
				decision, err := s.Like(ctx, ids[0], ids[1])
				if !errors.Is(err, domain.ErrBlocked) || decision != (domain.LikeDecision{}) {
					t.Fatalf("Like(%q, %q) = %+v, %v", ids[0], ids[1], decision, err)
				}
			}
			state := readBlockState(t, s, "u1", "u2")
			wantMatches := 1
			if blockFirst {
				wantMatches = 0
			}
			if state.leftLiked != !blockFirst || state.rightLiked != !blockFirst || !state.leftBlocked ||
				state.rightBlocked || state.active != 0 || state.matches != wantMatches {
				t.Fatalf("final state: %+v", state)
			}
			if matched, err := s.PairMatched(ctx, "u1", "u2"); err != nil || matched {
				t.Fatalf("PairMatched() = %v, %v", matched, err)
			}
		})
	}
}

func TestConcurrentLikeBlockBothOrders(t *testing.T) {
	for _, blockFirst := range []bool{false, true} {
		name := "like_first"
		if blockFirst {
			name = "block_first"
		}
		t.Run(name, func(t *testing.T) {
			s := testStore(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			// Встречная реакция должна сохраниться при любом порядке.
			if _, err := s.Like(ctx, "u2", "u1"); err != nil {
				t.Fatal(err)
			}
			first, firstPID := namedBlockStore(t, ctx, s, "block_test_first")
			second, secondPID := namedBlockStore(t, ctx, s, "block_test_second")
			if firstPID == secondPID {
				t.Fatal("operations must use different connections")
			}
			gate, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer rollback(gate)
			gatePID := gate.Conn().PgConn().PID()
			if _, err := gate.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext(current_schema())::bigint)`); err != nil {
				t.Fatal(err)
			}
			// Триггер останавливает реальную первую операцию после lockPair.
			// Вторая сессия затем обязана ждать именно эту пару в PostgreSQL.
			_, err = s.Pool.Exec(ctx, `
				CREATE FUNCTION pause_pair_writer() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN
				    IF current_setting('application_name') = 'block_test_first' THEN
				        PERFORM pg_advisory_xact_lock(hashtext(current_schema())::bigint);
				    END IF;
				    RETURN NEW;
				END $$;
				CREATE TRIGGER pause_pair_writer BEFORE UPDATE ON pairs
				FOR EACH ROW EXECUTE FUNCTION pause_pair_writer()`)
			if err != nil {
				t.Fatal(err)
			}
			type outcome struct {
				like  domain.LikeDecision
				block domain.PairChange
				err   error
			}
			result := make(chan outcome, 2)
			var workers sync.WaitGroup
			defer func() {
				cancel()
				rollback(gate)
				workers.Wait()
			}()
			run := func(store *Store, block bool) {
				workers.Add(1)
				go func() {
					defer workers.Done()
					var out outcome
					if block {
						out.block, out.err = store.Block(ctx, "u1", "u2")
					} else {
						out.like, out.err = store.Like(ctx, "u1", "u2")
					}
					result <- out
				}()
			}
			run(first, blockFirst)
			waitBlockedBy(t, ctx, s, firstPID, gatePID)
			run(second, !blockFirst)
			waitBlockedBy(t, ctx, s, secondPID, firstPID)
			if err := gate.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			var blockedLikes, changedBlocks, newMatches int
			for range 2 {
				select {
				case out := <-result:
					if errors.Is(out.err, domain.ErrBlocked) {
						blockedLikes++
						if out.like != (domain.LikeDecision{}) {
							t.Fatalf("denied Like returned success: %+v", out)
						}
					} else if out.err != nil {
						t.Fatal(out.err)
					}
					if out.block.Changed {
						changedBlocks++
					}
					if out.like.MatchedNow {
						newMatches++
						if !out.like.Changed || !out.like.Matched {
							t.Fatalf("incorrect Like response: %+v", out.like)
						}
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
			wantBlocked, wantMatches := 0, 1
			if blockFirst {
				wantBlocked, wantMatches = 1, 0
			}
			state := readBlockState(t, s, "u1", "u2")
			if blockedLikes != wantBlocked || changedBlocks != 1 || newMatches != wantMatches ||
				state.leftLiked != !blockFirst || !state.rightLiked || !state.leftBlocked || state.rightBlocked ||
				state.active != 0 || state.matches != wantMatches {
				t.Fatalf("blocked=%d blocks=%d new matches=%d state=%+v", blockedLikes, changedBlocks, newMatches, state)
			}
		})
	}
}
