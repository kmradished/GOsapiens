package memory

import "github.com/kmradished/GOsapiens/internal/domain"

type Interactions struct {
	profiles  *Profiles
	reactions map[[2]domain.UserID]domain.ReactionKind
	matches   map[[2]domain.UserID]domain.Match
}

func NewInteractions(
	profiles []domain.Profile,
	reactions []domain.Reaction,
	matches []domain.Match,
) *Interactions {
	store := &Interactions{
		profiles:  NewProfiles(profiles),
		reactions: make(map[[2]domain.UserID]domain.ReactionKind),
		matches:   make(map[[2]domain.UserID]domain.Match),
	}

	for _, reaction := range reactions {
		key := [2]domain.UserID{
			reaction.SenderID,
			reaction.ReceiverID,
		}
		store.reactions[key] = reaction.Kind
	}

	for _, match := range matches {
		key := matchKey(match.FirstUserID, match.SecondUserID)
		store.matches[key] = match
	}

	return store
}

func (s *Interactions) GetReaction(
	senderID, receiverID domain.UserID,
) (domain.ReactionKind, error) {
	key := [2]domain.UserID{senderID, receiverID}
	return s.reactions[key], nil
}

func (s *Interactions) HasMatch(
	firstID, secondID domain.UserID,
) (bool, error) {
	key := matchKey(firstID, secondID)
	_, exists := s.matches[key]
	return exists, nil
}

func (s *Interactions) SaveMatch(match domain.Match) error {
	key := matchKey(match.FirstUserID, match.SecondUserID)

	if _, exists := s.matches[key]; exists {
		return nil
	}

	s.matches[key] = match
	return nil
}

func (s *Interactions) GetProfile(
	id domain.UserID,
) (domain.Profile, error) {
	return s.profiles.Get(id)
}

func matchKey(firstID, secondID domain.UserID) [2]domain.UserID {
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}

	return [2]domain.UserID{firstID, secondID}
}
