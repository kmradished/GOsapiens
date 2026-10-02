package memory

import (
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestInteractionsStoresOneMatchForBothDirections(t *testing.T) {
	source := NewInteractions(nil, nil, nil)

	first := domain.Match{
		FirstUserID:  "u1",
		SecondUserID: "u10",
	}

	reversed := domain.Match{
		FirstUserID:  "u10",
		SecondUserID: "u1",
	}

	if err := source.SaveMatch(first); err != nil {
		t.Fatalf("first save: %v", err)
	}

	if err := source.SaveMatch(reversed); err != nil {
		t.Fatalf("repeated save: %v", err)
	}

	if len(source.matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(source.matches))
	}

	pairs := [][2]domain.UserID{
		{"u1", "u10"},
		{"u10", "u1"},
	}

	for _, pair := range pairs {
		exists, err := source.HasMatch(pair[0], pair[1])
		if err != nil || !exists {
			t.Fatalf("match %v: exists %v, error %v", pair, exists, err)
		}
	}
}
