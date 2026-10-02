package available

import (
	"fmt"

	"github.com/kmradished/GOsapiens/internal/domain"
)

type ProfileSource interface {
	List() ([]domain.Profile, error)
}

type ExclusionSource interface {
	BlockedIDs(userID domain.UserID) ([]domain.UserID, error)
	ViewedIDs(userID domain.UserID) ([]domain.UserID, error)
	ReactedIDs(userID domain.UserID) ([]domain.UserID, error)
}

type Service struct {
	profiles   ProfileSource
	exclusions ExclusionSource
}

func NewService(profiles ProfileSource, exclusions ExclusionSource) *Service {
	return &Service{
		profiles:   profiles,
		exclusions: exclusions,
	}
}

func (s *Service) Recommend(userID domain.UserID) ([]Candidate, error) {
	profiles, err := s.profiles.List()
	if err != nil {
		return nil, fmt.Errorf("получение профилей: %w", err)
	}

	blockedIDs, err := s.exclusions.BlockedIDs(userID)
	if err != nil {
		return nil, fmt.Errorf("получение блокировок: %w", err)
	}

	viewedIDs, err := s.exclusions.ViewedIDs(userID)
	if err != nil {
		return nil, fmt.Errorf("получение просмотренных анкет: %w", err)
	}

	reactedIDs, err := s.exclusions.ReactedIDs(userID)
	if err != nil {
		return nil, fmt.Errorf("получение реакций: %w", err)
	}

	return FilterCandidates(userID, profiles, blockedIDs, viewedIDs, reactedIDs), nil
}
