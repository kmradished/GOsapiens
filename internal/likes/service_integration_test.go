package likes_test

import (
	"testing"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likes"
	"github.com/kmradished/GOsapiens/internal/memory"
)

var _ application.LikeChecker = (*likes.Service)(nil)

func TestServiceWithMemorySources(t *testing.T) {
	profiles := memory.NewProfiles([]domain.Profile{
		{ID: "u1", Active: true},
		{ID: "u2", Active: true},
	})
	tests := []struct {
		name      string
		blocks    []domain.Block
		reactions []domain.Reaction
		target    domain.UserID
		want      likes.Result
	}{
		{
			name:   "actor blocks target",
			blocks: []domain.Block{{BlockerID: "u1", BlockedID: "u2"}},
			target: "u2", want: likes.Result{Reason: likes.ReasonBlocked},
		},
		{
			name:   "target blocks actor",
			blocks: []domain.Block{{BlockerID: "u2", BlockedID: "u1"}},
			target: "u2", want: likes.Result{Reason: likes.ReasonBlocked},
		},
		{
			name:      "existing like passes admission",
			reactions: []domain.Reaction{{SenderID: "u1", ReceiverID: "u2", Kind: domain.ReactionLike}},
			target:    "u2", want: likes.Result{Allowed: true},
		},
		{
			name:      "existing dislike passes admission",
			reactions: []domain.Reaction{{SenderID: "u1", ReceiverID: "u2", Kind: domain.ReactionDislike}},
			target:    "u2", want: likes.Result{Allowed: true},
		},
		{
			name:   "memory profile missing",
			target: "missing", want: likes.Result{Reason: likes.ReasonProfileNotFound},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := likes.NewService(profiles, memory.NewLikeState(tt.blocks, tt.reactions))
			for attempt := 0; attempt < 2; attempt++ {
				got, err := service.Check("u1", tt.target)
				if err != nil || got != tt.want {
					t.Fatalf("Check attempt %d = %+v, %v; want %+v", attempt+1, got, err, tt.want)
				}
			}
		})
	}
}

// для допуска достаточно проверки блокировок
type blockOnlySource struct{}

func (blockOnlySource) IsBlocked(_, _ domain.UserID) (bool, error) {
	return false, nil
}

func TestServiceNeedsOnlyBlockSource(t *testing.T) {
	profiles := memory.NewProfiles([]domain.Profile{
		{ID: "u1", Active: true},
		{ID: "u2", Active: true},
	})
	got, err := likes.NewService(profiles, blockOnlySource{}).Check("u1", "u2")
	if err != nil || got != (likes.Result{Allowed: true}) {
		t.Fatalf("Check = %+v, %v; want allowed", got, err)
	}
}
