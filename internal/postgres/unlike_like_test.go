//go:build dbintegration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestConcurrentLikeUnlikeBothOrders(t *testing.T) {
	for _, name := range []string{"like_first", "unlike_first"} {
		t.Run(name, func(t *testing.T) {
			likeFirst := name == "like_first"
			store := testStore(t)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()

			// В фикстурах u2 уже лайкнул u5.
			first, firstPID := namedBlockStore(t, ctx, store, "unlike_test_first")
			second, secondPID := namedBlockStore(t, ctx, store, "unlike_test_second")
			gate, err := store.Pool.BeginTx(ctx, pgx.TxOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer rollback(gate)
			key, err := domain.NewPairKey("u2", "u5")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := lockPair(ctx, gate, key); err != nil {
				t.Fatal(err)
			}

			type result struct {
				like   domain.LikeDecision
				unlike domain.PairChange
				err    error
			}
			likes := make(chan result, 1)
			unlikes := make(chan result, 1)
			startLike := func(s *Store) {
				go func() {
					decision, err := s.Like(ctx, "u5", "u2")
					likes <- result{like: decision, err: err}
				}()
			}
			startUnlike := func(s *Store) {
				go func() {
					change, err := s.Unlike(ctx, "u2", "u5")
					unlikes <- result{unlike: change, err: err}
				}()
			}
			if likeFirst {
				startLike(first)
			} else {
				startUnlike(first)
			}
			waitBlockedBy(t, ctx, store, firstPID, gate.Conn().PgConn().PID())
			if likeFirst {
				startUnlike(second)
			} else {
				startLike(second)
			}
			waitBlockedBy(t, ctx, store, secondPID, firstPID)
			if err := gate.Commit(ctx); err != nil {
				t.Fatal(err)
			}

			for range 2 {
				select {
				case got := <-likes:
					if got.err != nil || !got.like.Changed || got.like.Matched != likeFirst || got.like.MatchedNow != likeFirst {
						t.Fatalf("Like() = %+v, %v", got.like, got.err)
					}
				case got := <-unlikes:
					if got.err != nil || got.unlike != (domain.PairChange{Changed: true}) {
						t.Fatalf("Unlike() = %+v, %v", got.unlike, got.err)
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}

			wantMatches := 0
			if likeFirst {
				wantMatches = 1
			}
			state := readBlockState(t, store, "u2", "u5")
			if state.leftLiked || !state.rightLiked || state.active != 0 || state.matches != wantMatches {
				t.Fatalf("state after Like and Unlike = %+v", state)
			}
		})
	}
}
