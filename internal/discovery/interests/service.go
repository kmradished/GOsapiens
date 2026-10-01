package interests

import (
	"fmt"
	"github.com/kmradished/GOsapiens/internal/domain"
)

//ProfileSource provides the profile data required by Service
type ProfileSource interface {
	Get(id domain.UserID) (domain.Profile, error)
	List() ([]domain.Profile, error)
}

//Service coordinates profile loading and interest-based discovery
type Service struct {
	source ProfileSource
}

//NewService creates a Service with provided profile source
func NewService(source ProfileSource) *Service {
	return &Service{source: source}
}

//Recommend returns anonymous cards with common interests for the current user
func (s *Service) Recommend(currentID domain.UserID) ([]AnonymousCard, error) {
	current, err := s.source.Get(currentID)
	if err != nil {
		return nil, fmt.Errorf("get current profile %q: %w", currentID, err)
	}

	candidates, err := s.source.List()
	if err != nil {
		return nil, fmt.Errorf("list candidate profiles: %w", err)
	}

	return Discover(current, candidates), nil
}
