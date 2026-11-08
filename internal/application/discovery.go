package application

import (
	"fmt"
	"slices"

	"github.com/kmradished/GOsapiens/internal/discovery/available"
	"github.com/kmradished/GOsapiens/internal/discovery/interests"
	"github.com/kmradished/GOsapiens/internal/domain"
)

const (
	DefaultDiscoveryLimit = 20
	MaxDiscoveryLimit     = 50
)

type DiscoveryQuery struct {
	Interest string
	Limit    int
	Offset   int
}

func (q DiscoveryQuery) ValidPage() bool {
	return q.Limit >= 1 && q.Limit <= MaxDiscoveryLimit && q.Offset >= 0
}

// LikedIDsReader возвращает список идентификаторов пользователей, которые были лайкнуты данным пользователем
type LikedIDsReader interface {
	LikedIDs(actorID domain.UserID) ([]domain.UserID, error)
}

type DiscoveryService struct {
	profiles   interests.ProfileSource
	exclusions available.ExclusionSource
	likes      LikedIDsReader
}

func NewDiscoveryService(profiles interests.ProfileSource, exclusions available.ExclusionSource, likes LikedIDsReader) *DiscoveryService {
	return &DiscoveryService{profiles: profiles, exclusions: exclusions, likes: likes}
}

// Discover возвращает список кандидатов для данного пользователя на основе его интересов и исключений
func (s *DiscoveryService) Discover(actorID domain.UserID, query DiscoveryQuery) ([]interests.AnonymousCard, error) {
	if !query.ValidPage() {
		return nil, fmt.Errorf("invalid discovery pagination: limit=%d offset=%d", query.Limit, query.Offset)
	}
	current, err := s.profiles.Get(actorID)
	if err != nil {
		return nil, fmt.Errorf("get discovery actor profile: %w", err)
	}
	profiles, err := s.profiles.List()
	if err != nil {
		return nil, fmt.Errorf("list discovery profiles: %w", err)
	}
	blocked, err := s.exclusions.BlockedIDs(actorID)
	if err != nil {
		return nil, fmt.Errorf("get discovery blocks: %w", err)
	}
	viewed, err := s.exclusions.ViewedIDs(actorID)
	if err != nil {
		return nil, fmt.Errorf("get discovery views: %w", err)
	}
	reacted, err := s.exclusions.ReactedIDs(actorID)
	if err != nil {
		return nil, fmt.Errorf("get discovery reactions: %w", err)
	}
	liked, err := s.likes.LikedIDs(actorID)
	if err != nil {
		return nil, fmt.Errorf("get discovery likes: %w", err)
	}

	allReactions := append(slices.Clip(reacted), liked...)
	eligible := available.FilterCandidates(actorID, profiles, blocked, viewed, allReactions)
	eligibleIDs := make(map[domain.UserID]struct{}, len(eligible))
	for _, candidate := range eligible {
		eligibleIDs[candidate.ID] = struct{}{}
	}
	eligibleProfiles := make([]domain.Profile, 0, len(eligible))
	for _, profile := range profiles {
		if _, ok := eligibleIDs[profile.ID]; ok {
			eligibleProfiles = append(eligibleProfiles, profile)
		}
	}

	cards := interests.Discover(current, eligibleProfiles)
	if query.Interest != "" {
		cards = slices.DeleteFunc(cards, func(card interests.AnonymousCard) bool {
			return !slices.Contains(card.CommonMovies, query.Interest) &&
				!slices.Contains(card.CommonMusic, query.Interest) &&
				!slices.Contains(card.CommonBooks, query.Interest)
		})
	}
	if query.Offset >= len(cards) {
		return []interests.AnonymousCard{}, nil
	}
	end := query.Offset + min(query.Limit, len(cards)-query.Offset)
	return cards[query.Offset:end], nil
}
