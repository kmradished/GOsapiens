package domain_test

import (
	"errors"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestNewPairKeyCanonicalizesIDs(t *testing.T) {
	expected := domain.PairKey{
		LeftID:  "u1",
		RightID: "u2",
	}

	forward, err := domain.NewPairKey("u1", "u2")
	if err != nil {
		t.Fatalf("NewPairKey(u1, u2) returned error: %v", err)
	}

	reverse, err := domain.NewPairKey("u2", "u1")
	if err != nil {
		t.Fatalf("NewPairKey(u2, u1) returned error: %v", err)
	}

	if forward != expected {
		t.Fatalf("NewPairKey(u1, u2) = %+v, want %+v", forward, expected)
	}

	if reverse != expected {
		t.Fatalf("NewPairKey(u2, u1) = %+v, want %+v", reverse, expected)
	}
}

func TestNewPairKeyRejectsInvalidIDs(t *testing.T) {
	tests := []struct {
		name     string
		firstID  domain.UserID
		secondID domain.UserID
	}{
		{
			name:     "empty first ID",
			firstID:  "",
			secondID: "u2",
		},
		{
			name:     "empty second ID",
			firstID:  "u1",
			secondID: "",
		},
		{
			name:     "equal IDs",
			firstID:  "u1",
			secondID: "u1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := domain.NewPairKey(test.firstID, test.secondID)
			if !errors.Is(err, domain.ErrInvalidPair) {
				t.Fatalf("NewPairKey(%q, %q) error = %v, want %v",
					test.firstID,
					test.secondID,
					err,
					domain.ErrInvalidPair,
				)
			}
		})
	}
}

func TestPairKeyValid(t *testing.T) {
	key := domain.PairKey{
		LeftID:  "u1",
		RightID: "u2",
	}

	if !key.Valid() {
		t.Fatalf("PairKey%+v.Valid() = false, want true", key)
	}
}

func TestPairKeyInvalid(t *testing.T) {
	keys := []domain.PairKey{
		{},
		{
			LeftID:  "u1",
			RightID: "u1",
		},
		{
			LeftID:  "u2",
			RightID: "u1",
		},
	}

	for _, key := range keys {
		if key.Valid() {
			t.Fatalf("PairKey%+v.Valid() = true, want false", key)
		}
	}
}
