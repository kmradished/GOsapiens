// Package memory provides in-memory data sources for local runs and tests
package memory

import (
	"errors"
	"fmt"
	"slices"

	"github.com/kmradished/GOsapiens/internal/domain"
)

// ErrProfileNotFound indicates that a profile does not exist in the source
var ErrProfileNotFound = errors.New("profile not found")

// Profiles stores profiles in memory and preserves their insertion order
type Profiles struct {
	items map[domain.UserID]domain.Profile
	order []domain.UserID
}

// NewProfiles creates an in-memory profile source from seed data
func NewProfiles(seed []domain.Profile) *Profiles {
	profiles := &Profiles{
		// order to save order, items for fast search of matchng UserID and Profile
		items: make(map[domain.UserID]domain.Profile, len(seed)),
		order: make([]domain.UserID, 0, len(seed)),
	}

	for _, profile := range seed {
		if _, exists := profiles.items[profile.ID]; !exists {
			profiles.order = append(profiles.order, profile.ID)
		}

		profiles.items[profile.ID] = cloneProfile(profile)
	}

	return profiles
}

// Get returns an isolated copy of a profile by ID
func (p *Profiles) Get(id domain.UserID) (domain.Profile, error) {
	profile, exists := p.items[id]
	if !exists {
		return domain.Profile{}, fmt.Errorf("profile %q: %w", id, ErrProfileNotFound)
	}

	return cloneProfile(profile), nil
}

// List returns isolated profile copies in insertion order
func (p *Profiles) List() ([]domain.Profile, error) {
	result := make([]domain.Profile, 0, len(p.order))
	for _, id := range p.order {
		result = append(result, cloneProfile(p.items[id]))
	}

	return result, nil
}

func cloneProfile(profile domain.Profile) domain.Profile {
	clone := profile
	clone.Interests.Movies = slices.Clone(profile.Interests.Movies)
	clone.Interests.Music = slices.Clone(profile.Interests.Music)
	clone.Interests.Books = slices.Clone(profile.Interests.Books)
	return clone
}
