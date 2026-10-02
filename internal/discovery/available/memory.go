package available

import "github.com/kmradished/GOsapiens/internal/domain"

type MemoryExclusions struct {
	blocks    []domain.Block
	views     []domain.View
	reactions []domain.Reaction
}

func NewMemoryExclusions(
	blocks []domain.Block,
	views []domain.View,
	reactions []domain.Reaction,
) *MemoryExclusions {
	return &MemoryExclusions{
		blocks:    blocks,
		views:     views,
		reactions: reactions,
	}
}

func (m *MemoryExclusions) BlockedIDs(userID domain.UserID) ([]domain.UserID, error) {
	result := make([]domain.UserID, 0)

	for _, block := range m.blocks {
		if block.BlockerID == userID {
			result = append(result, block.BlockedID)
		}
		if block.BlockedID == userID {
			result = append(result, block.BlockerID)
		}
	}

	return result, nil
}

func (m *MemoryExclusions) ViewedIDs(userID domain.UserID) ([]domain.UserID, error) {
	result := make([]domain.UserID, 0)

	for _, view := range m.views {
		if view.ViewerID == userID {
			result = append(result, view.CandidateID)
		}
	}

	return result, nil
}

func (m *MemoryExclusions) ReactedIDs(userID domain.UserID) ([]domain.UserID, error) {
	result := make([]domain.UserID, 0)

	for _, reaction := range m.reactions {
		if reaction.SenderID == userID {
			result = append(result, reaction.ReceiverID)
		}
	}

	return result, nil
}
