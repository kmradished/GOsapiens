package memory

import "github.com/kmradished/GOsapiens/internal/domain"

// блокировки и реакции в памяти
type LikeState struct {
	blocks    []domain.Block
	reactions []domain.Reaction
}

// создание хранилища
func NewLikeState(
	blocks []domain.Block,
	reactions []domain.Reaction,
) *LikeState {
	blockCopy := make([]domain.Block, len(blocks))
	copy(blockCopy, blocks)

	reactionCopy := make([]domain.Reaction, len(reactions))
	copy(reactionCopy, reactions)

	return &LikeState{
		blocks:    blockCopy,
		reactions: reactionCopy,
	}
}

// блокировка в любую сторону
func (s *LikeState) IsBlocked(
	userID, candidateID domain.UserID,
) (bool, error) {
	for _, block := range s.blocks {
		if block.BlockerID == userID && block.BlockedID == candidateID {
			return true, nil
		}

		if block.BlockerID == candidateID && block.BlockedID == userID {
			return true, nil
		}
	}

	return false, nil
}

// реакция пользователя на кандидата
func (s *LikeState) HasReaction(
	userID, candidateID domain.UserID,
) (bool, error) {
	for _, reaction := range s.reactions {
		if reaction.SenderID == userID && reaction.ReceiverID == candidateID {
			if reaction.Kind == domain.ReactionLike ||
				reaction.Kind == domain.ReactionDislike {
				return true, nil
			}
		}
	}

	return false, nil
}