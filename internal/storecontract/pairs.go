package storecontract

import (
	"errors"
	"testing"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/domain"
)

// Check проверяет общий контракт всех реализаций application.PairStore.
func Check(t *testing.T, factory func() application.PairStore) {
	t.Helper()

	t.Run("missing pair", func(t *testing.T) {
		store := factory()
		key := newPairKey(t, "u1", "u2")

		state, found, err := store.Load(key)
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if found {
			t.Fatal("Load() found = true, want false")
		}
		if state != (domain.PairState{}) {
			t.Fatalf("Load() state = %+v, want zero state", state)
		}
	})

	t.Run("save and load", func(t *testing.T) {
		store := factory()
		key := newPairKey(t, "u1", "u2")
		want := domain.PairState{LeftLiked: true}

		if err := store.Save(key, want); err != nil {
			t.Fatalf("Save() error = %v, want nil", err)
		}

		got, found, err := store.Load(key)
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if !found {
			t.Fatal("Load() found = false, want true")
		}
		if got != want {
			t.Fatalf("Load() state = %+v, want %+v", got, want)
		}
	})

	t.Run("replace whole state", func(t *testing.T) {
		store := factory()
		key := newPairKey(t, "u1", "u2")

		if err := store.Save(key, domain.PairState{
			LeftLiked:  true,
			RightLiked: true,
		}); err != nil {
			t.Fatalf("first Save() error = %v, want nil", err)
		}

		want := domain.PairState{RightLiked: true}
		if err := store.Save(key, want); err != nil {
			t.Fatalf("second Save() error = %v, want nil", err)
		}

		got, found, err := store.Load(key)
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if !found {
			t.Fatal("Load() found = false, want true")
		}
		if got != want {
			t.Fatalf("Load() state = %+v, want %+v", got, want)
		}
	})

	t.Run("pairs are independent", func(t *testing.T) {
		store := factory()
		firstKey := newPairKey(t, "u1", "u2")
		secondKey := newPairKey(t, "u1", "u3")
		firstWant := domain.PairState{LeftLiked: true}
		secondWant := domain.PairState{RightLiked: true}

		if err := store.Save(firstKey, firstWant); err != nil {
			t.Fatalf("Save(first pair) error = %v, want nil", err)
		}
		if err := store.Save(secondKey, secondWant); err != nil {
			t.Fatalf("Save(second pair) error = %v, want nil", err)
		}

		assertStoredState(t, store, firstKey, firstWant)
		assertStoredState(t, store, secondKey, secondWant)
	})

	t.Run("invalid key does not change data", func(t *testing.T) {
		store := factory()
		validKey := newPairKey(t, "u1", "u2")
		original := domain.PairState{LeftLiked: true}

		if err := store.Save(validKey, original); err != nil {
			t.Fatalf("Save(valid key) error = %v, want nil", err)
		}

		invalidKeys := []domain.PairKey{
			{},
			{LeftID: "u1", RightID: "u1"},
			{LeftID: "u2", RightID: "u1"},
		}

		for _, key := range invalidKeys {
			if err := store.Save(key, domain.PairState{RightLiked: true}); !errors.Is(err, domain.ErrInvalidPair) {
				t.Fatalf("Save(%+v) error = %v, want %v",
					key, err, domain.ErrInvalidPair)
			}

			_, _, err := store.Load(key)
			if !errors.Is(err, domain.ErrInvalidPair) {
				t.Fatalf("Load(%+v) error = %v, want %v",
					key, err, domain.ErrInvalidPair)
			}

			assertStoredState(t, store, validKey, original)
		}
	})
}

func newPairKey(
	t *testing.T,
	leftID domain.UserID,
	rightID domain.UserID,
) domain.PairKey {
	t.Helper()

	key, err := domain.NewPairKey(leftID, rightID)
	if err != nil {
		t.Fatalf("NewPairKey(%q, %q) error = %v", leftID, rightID, err)
	}

	return key
}

func assertStoredState(
	t *testing.T,
	store application.PairStore,
	key domain.PairKey,
	want domain.PairState,
) {
	t.Helper()

	got, found, err := store.Load(key)
	if err != nil {
		t.Fatalf("Load(%+v) error = %v, want nil", key, err)
	}
	if !found {
		t.Fatalf("Load(%+v) found = false, want true", key)
	}
	if got != want {
		t.Fatalf("Load(%+v) state = %+v, want %+v", key, got, want)
	}
}
