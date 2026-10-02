package memory

import (
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
)

// блокировка в обе стороны
func TestLikeStateIsBlocked(t *testing.T) {
	state := NewLikeState(
		[]domain.Block{
			{BlockerID: "u1", BlockedID: "u2"},
		},
		nil,
	)

	direct, err := state.IsBlocked("u1", "u2")
	if err != nil || !direct {
		t.Fatalf("ожидалась прямая блокировка: %v, %v", direct, err)
	}

	reverse, err := state.IsBlocked("u2", "u1")
	if err != nil || !reverse {
		t.Fatalf("ожидалась обратная блокировка: %v, %v", reverse, err)
	}

	unrelated, err := state.IsBlocked("u1", "u3")
	if err != nil || unrelated {
		t.Fatalf("блокировки быть не должно: %v, %v", unrelated, err)
	}
}

// учитываются лайк и дизлайк
func TestLikeStateHasReaction(t *testing.T) {
	state := NewLikeState(
		nil,
		[]domain.Reaction{
			{
				SenderID:   "u1",
				ReceiverID: "u2",
				Kind:       domain.ReactionLike,
			},
			{
				SenderID:   "u1",
				ReceiverID: "u3",
				Kind:       domain.ReactionDislike,
			},
		},
	)

	liked, err := state.HasReaction("u1", "u2")
	if err != nil || !liked {
		t.Fatalf("ожидался прошлый лайк: %v, %v", liked, err)
	}

	disliked, err := state.HasReaction("u1", "u3")
	if err != nil || !disliked {
		t.Fatalf("ожидался прошлый дизлайк: %v, %v", disliked, err)
	}

	reverse, err := state.HasReaction("u2", "u1")
	if err != nil || reverse {
		t.Fatalf("встречная реакция не должна учитываться: %v, %v", reverse, err)
	}

	missing, err := state.HasReaction("u1", "u4")
	if err != nil || missing {
		t.Fatalf("реакции быть не должно: %v, %v", missing, err)
	}
}

// пустое хранилище
func TestLikeStateEmpty(t *testing.T) {
	state := NewLikeState(nil, nil)

	blocked, err := state.IsBlocked("u1", "u2")
	if err != nil || blocked {
		t.Fatalf("блокировки быть не должно: %v, %v", blocked, err)
	}

	reacted, err := state.HasReaction("u1", "u2")
	if err != nil || reacted {
		t.Fatalf("реакции быть не должно: %v, %v", reacted, err)
	}
}

// исходные списки не влияют на хранилище
func TestLikeStateCopiesInput(t *testing.T) {
	blocks := []domain.Block{
		{BlockerID: "u1", BlockedID: "u2"},
	}
	reactions := []domain.Reaction{
		{
			SenderID:   "u1",
			ReceiverID: "u3",
			Kind:       domain.ReactionLike,
		},
	}

	state := NewLikeState(blocks, reactions)

	blocks[0].BlockedID = "u4"
	reactions[0].ReceiverID = "u4"

	blocked, err := state.IsBlocked("u1", "u2")
	if err != nil || !blocked {
		t.Fatalf("блокировка должна сохраниться: %v, %v", blocked, err)
	}

	reacted, err := state.HasReaction("u1", "u3")
	if err != nil || !reacted {
		t.Fatalf("реакция должна сохраниться: %v, %v", reacted, err)
	}
}