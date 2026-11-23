//go:build dbintegration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/domain"
)

var _ application.ProfileStore = (*Store)(nil)

func TestProfileReadAndUpdate(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	initial := domain.ProfileRevision{ID: "u1", Name: "u1", Version: 1}
	assertProfile(t, ctx, s, initial)
	for _, tc := range []struct {
		actorID, profileID domain.UserID
		wantErr            error
	}{
		{"u2", "u1", domain.ErrForbidden},
		{"missing", "missing", domain.ErrProfileNotFound},
		{"u2", "missing", domain.ErrProfileNotFound},
	} {
		read, err := s.ReadProfile(ctx, tc.actorID, tc.profileID)
		if !errors.Is(err, tc.wantErr) || read != (domain.ProfileRevision{}) {
			t.Fatalf("read %q as %q: got %+v / %v, want %v", tc.profileID, tc.actorID, read, err, tc.wantErr)
		}
		update, err := s.UpdateProfile(ctx, tc.actorID, tc.profileID, domain.ProfileEdit{
			Name: "Отказ", ExpectedVersion: 1,
		})
		if !errors.Is(err, tc.wantErr) || update != (domain.ProfileRevision{}) {
			t.Fatalf("update %q as %q: got %+v / %v, want %v", tc.profileID, tc.actorID, update, err, tc.wantErr)
		}
	}
	assertProfile(t, ctx, s, initial)

	firstEdit := domain.ProfileEdit{Name: "Полина", Bio: "Люблю кино", ExpectedVersion: initial.Version}
	first, err := s.UpdateProfile(ctx, "u1", "u1", firstEdit)
	want := domain.ProfileRevision{ID: "u1", Name: firstEdit.Name, Bio: firstEdit.Bio, Version: 2}
	if err != nil || first != want {
		t.Fatalf("first update: got %+v / %v, want %+v", first, err, want)
	}
	stale, err := s.UpdateProfile(ctx, "u1", "u1", domain.ProfileEdit{Name: "Старая правка", ExpectedVersion: initial.Version})
	if !errors.Is(err, domain.ErrVersionConflict) || stale != (domain.ProfileRevision{}) {
		t.Fatalf("stale update: got %+v / %v, want version conflict", stale, err)
	}
	assertProfile(t, ctx, s, want)

	firstEdit.ExpectedVersion = want.Version
	want.Version++
	sameValues, err := s.UpdateProfile(ctx, "u1", "u1", firstEdit)
	if err != nil || sameValues != want {
		t.Fatalf("update with unchanged values: got %+v / %v, want %+v", sameValues, err, want)
	}
	assertProfile(t, ctx, s, want)
}

func TestConcurrentProfileUpdates(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	blocker, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer rollback(blocker)
	var version int64
	if err := blocker.QueryRow(ctx, "SELECT version FROM profiles WHERE id = 'u1' FOR UPDATE").Scan(&version); err != nil {
		t.Fatal(err)
	}

	type result struct {
		edit    domain.ProfileEdit
		profile domain.ProfileRevision
		err     error
	}
	results := make(chan result, 2)
	for _, name := range []string{"Правка A", "Правка B"} {
		go func() {
			edit := domain.ProfileEdit{Name: name, Bio: name, ExpectedVersion: version}
			profile, err := s.UpdateProfile(ctx, "u1", "u1", edit)
			results <- result{edit: edit, profile: profile, err: err}
		}()
	}

	// Обе транзакции уже начали SELECT FOR UPDATE и ждут строку в БД.
	// RowShareLock подтверждает вход в запрос через два разных соединения.
	pid := blocker.Conn().PgConn().PID()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting int
		err := s.Pool.QueryRow(ctx, `
			SELECT count(DISTINCT pid) FROM pg_locks
			WHERE relation = 'profiles'::regclass AND mode = 'RowShareLock'
			  AND granted AND pid <> $1
		`, pid).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting == 2 {
			break
		}
		select {
		case unexpected := <-results:
			t.Fatalf("update completed before the lock was released: %+v", unexpected)
		case <-ctx.Done():
			t.Fatal("both updates did not reach PostgreSQL")
		case <-ticker.C:
		}
	}
	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var winner domain.ProfileRevision
	conflicts := 0
	for range 2 {
		select {
		case result := <-results:
			switch {
			case result.err == nil && winner.ID == "":
				winner = domain.ProfileRevision{ID: "u1", Name: result.edit.Name, Bio: result.edit.Bio, Version: version + 1}
				if result.profile != winner {
					t.Fatalf("winner: got %+v, want %+v", result.profile, winner)
				}
			case errors.Is(result.err, domain.ErrVersionConflict) && result.profile == (domain.ProfileRevision{}):
				conflicts++
			default:
				t.Fatalf("unexpected update result: %+v", result)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	if winner.ID == "" || conflicts != 1 {
		t.Fatalf("want one saved update and one conflict, got winner %+v and %d conflicts", winner, conflicts)
	}
	assertProfile(t, ctx, s, winner)
}

func TestProfileUpdateRollsBackOnCommitError(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Только тестовая схема. Дубликат имени обнаруживается при COMMIT.
	if _, err := s.Pool.Exec(ctx, "ALTER TABLE profiles ADD UNIQUE (name) DEFERRABLE INITIALLY DEFERRED"); err != nil {
		t.Fatal(err)
	}
	observer, err := s.Pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Release()

	initial := domain.ProfileRevision{ID: "u1", Name: "u1", Version: 1}
	profile, err := s.UpdateProfile(ctx, "u1", "u1", domain.ProfileEdit{
		Name: "u2", Bio: "Должно откатиться", ExpectedVersion: initial.Version,
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || profile != (domain.ProfileRevision{}) {
		t.Fatalf("update: got %+v / %v, want a unique violation at commit", profile, err)
	}
	var stored domain.ProfileRevision
	err = observer.QueryRow(ctx, "SELECT id, name, bio, version FROM profiles WHERE id = $1", initial.ID).
		Scan(&stored.ID, &stored.Name, &stored.Bio, &stored.Version)
	if err != nil || stored != initial {
		t.Fatalf("read after rollback on another connection: got %+v / %v, want %+v", stored, err, initial)
	}

	retry, err := s.UpdateProfile(ctx, "u1", "u1", domain.ProfileEdit{Name: "Полина", ExpectedVersion: initial.Version})
	want := domain.ProfileRevision{ID: "u1", Name: "Полина", Version: 2}
	if err != nil || retry != want {
		t.Fatalf("retry: got %+v / %v, want %+v", retry, err, want)
	}
	assertProfile(t, ctx, s, want)
}

func assertProfile(t *testing.T, ctx context.Context, s *Store, want domain.ProfileRevision) {
	t.Helper()
	got, err := s.ReadProfile(ctx, want.ID, want.ID)
	if err != nil || got != want {
		t.Fatalf("read profile: got %+v / %v, want %+v", got, err, want)
	}
}
