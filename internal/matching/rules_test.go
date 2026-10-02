package matching

import (
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestDecideCreatesMatch(t *testing.T) {
	got := Decide(
		"u1",
		"u10",
		domain.ReactionLike,
		domain.ReactionLike,
		false,
	)

	want := Result{
		Status: StatusCreated,
		Match: domain.Match{
			FirstUserID:  "u1",
			SecondUserID: "u10",
		},
		FullProfilesAvailable: true,
	}

	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
func TestDecideWaitsForReciprocalLike(t *testing.T) {
	got := Decide(
		"u1",
		"u10",
		domain.ReactionLike,
		"",
		false,
	)

	want := Result{
		Status: StatusWaiting,
	}

	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
func TestDecideReturnsExistingMatch(t *testing.T) {
	got := Decide(
		"u1",
		"u10",
		domain.ReactionLike,
		domain.ReactionLike,
		true,
	)

	want := Result{
		Status: StatusAlreadyExists,
		Match: domain.Match{
			FirstUserID:  "u1",
			SecondUserID: "u10",
		},
		FullProfilesAvailable: true,
	}

	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
func TestDecideWithoutMutualLikes(t *testing.T) {
	tests := []struct {
		name   string
		first  domain.ReactionKind
		second domain.ReactionKind
	}{
		{"only second likes", "", domain.ReactionLike},
		{"like and dislike", domain.ReactionLike, domain.ReactionDislike},
		{"dislike and like", domain.ReactionDislike, domain.ReactionLike},
		{"no reactions", "", ""},
		{"both dislike", domain.ReactionDislike, domain.ReactionDislike},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide("u1", "u10", tt.first, tt.second, false)

			want := Result{
				Status: StatusWaiting,
			}

			if got != want {
				t.Fatalf("got %+v, want %+v", got, want)
			}
		})
	}
}
func TestDecideDoesNotChangeInputReactions(t *testing.T) {
	first := domain.Reaction{
		SenderID:   "u1",
		ReceiverID: "u10",
		Kind:       domain.ReactionLike,
	}

	second := domain.Reaction{
		SenderID:   "u10",
		ReceiverID: "u1",
		Kind:       domain.ReactionLike,
	}

	beforeFirst, beforeSecond := first, second

	Decide(
		first.SenderID,
		first.ReceiverID,
		first.Kind,
		second.Kind,
		false,
	)

	if first != beforeFirst || second != beforeSecond {
		t.Fatal("input reactions changed")
	}
}
