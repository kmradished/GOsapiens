package interests_test

import (
	"reflect"
	"testing"

	"github.com/kmradished/GOsapiens/internal/discovery/interests"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestDiscoverReturnsMatchingCandidates(t *testing.T) {
	current := domain.Profile{
		ID:     "u1",
		Active: true,
		Interests: domain.Interests{
			Movies: []string{"Dune", "Interstellar"},
			Music:  []string{"Queen"},
			Books:  []string{"1984"},
		},
	}

	candidates := []domain.Profile{
		{
			ID:     "u2",
			Active: true,
			Interests: domain.Interests{
				Movies: []string{"Dune", "Avatar"},
				Books:  []string{"1984"},
			},
		},
		{
			ID:     "u3",
			Active: true,
			Interests: domain.Interests{
				Music: []string{"Queen"},
			},
		},
	}

	want := []interests.AnonymousCard{
		{
			CandidateID:  "u2",
			CommonMovies: []string{"Dune"},
			CommonMusic:  []string{},
			CommonBooks:  []string{"1984"},
		},
		{
			CandidateID:  "u3",
			CommonMovies: []string{},
			CommonMusic:  []string{"Queen"},
			CommonBooks:  []string{},
		},
	}

	got := interests.Discover(current, candidates)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Discover() = %#v, want %#v", got, want)
	}
}

func TestDiscoverExcludesUnsuitableCandidates(t *testing.T) {
	current := domain.Profile{
		ID:     "u1",
		Active: true,
		Interests: domain.Interests{
			Movies: []string{"Dune"},
		},
	}

	tests := []struct {
		name      string
		candidate domain.Profile
	}{
		{
			name: "current user",
			candidate: domain.Profile{
				ID:     "u1",
				Active: true,
				Interests: domain.Interests{
					Movies: []string{"Dune"},
				},
			},
		},
		{
			name: "inactive profile",
			candidate: domain.Profile{
				ID:     "u2",
				Active: false,
				Interests: domain.Interests{
					Movies: []string{"Dune"},
				},
			},
		},
		{
			name: "no common interests",
			candidate: domain.Profile{
				ID:     "u3",
				Active: true,
				Interests: domain.Interests{
					Movies: []string{"Avatar"},
					Music:  []string{"Muse"},
					Books:  []string{"Hamlet"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := interests.Discover(current, []domain.Profile{tt.candidate})

			if got == nil {
				t.Fatal("Discover() returned nil, want a non-nil empty slice")
			}

			if len(got) != 0 {
				t.Fatalf("Discover() = %#v, want an empty result", got)
			}
		})
	}
}

func TestDiscoverReturnsNonNilEmptyResult(t *testing.T) {
	got := interests.Discover(domain.Profile{ID: "u1"}, nil)

	if got == nil {
		t.Fatal("Discover() returned nil, want a non-nil empty slice")
	}

	if len(got) != 0 {
		t.Fatalf("Discover() = %#v, want an empty result", got)
	}
}

func TestDiscoverDoesNotDuplicateInterestsOrCandidates(t *testing.T) {
	current := domain.Profile{
		ID: "u1",
		Interests: domain.Interests{
			Movies: []string{"Dune", "Dune"},
		},
	}
	candidates := []domain.Profile{
		{
			ID:     "u2",
			Active: true,
			Interests: domain.Interests{
				Movies: []string{"Dune", "Dune"},
			},
		},
	}

	want := []interests.AnonymousCard{
		{
			CandidateID:  "u2",
			CommonMovies: []string{"Dune"},
			CommonMusic:  []string{},
			CommonBooks:  []string{},
		},
	}

	got := interests.Discover(current, candidates)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Discover() = %#v, want %#v", got, want)
	}
}

func TestDiscoverDoesNotModifyInputs(t *testing.T) {
	current := domain.Profile{
		ID: "u1",
		Interests: domain.Interests{
			Movies: []string{"Dune"},
			Music:  []string{"Queen"},
			Books:  []string{"1984"},
		},
	}
	candidates := []domain.Profile{
		{
			ID:     "u2",
			Active: true,
			Interests: domain.Interests{
				Movies: []string{"Dune"},
				Music:  []string{"Queen"},
				Books:  []string{"1984"},
			},
		},
	}

	currentBefore := cloneProfile(current)
	candidatesBefore := cloneProfiles(candidates)

	interests.Discover(current, candidates)

	if !reflect.DeepEqual(current, currentBefore) {
		t.Fatalf("Discover() modified current profile: got %#v, want %#v", current, currentBefore)
	}

	if !reflect.DeepEqual(candidates, candidatesBefore) {
		t.Fatalf("Discover() modified candidates: got %#v, want %#v", candidates, candidatesBefore)
	}
}

func cloneProfiles(profiles []domain.Profile) []domain.Profile {
	result := make([]domain.Profile, len(profiles))
	for i, profile := range profiles {
		result[i] = cloneProfile(profile)
	}
	return result
}

func cloneProfile(profile domain.Profile) domain.Profile {
	clone := profile
	clone.Interests.Movies = append([]string(nil), profile.Interests.Movies...)
	clone.Interests.Music = append([]string(nil), profile.Interests.Music...)
	clone.Interests.Books = append([]string(nil), profile.Interests.Books...)
	return clone
}
