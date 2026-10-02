package main

import "github.com/kmradished/GOsapiens/internal/domain"

func demoProfiles() []domain.Profile {
	return []domain.Profile{
		{
			ID:       "u1",
			Name:     "Daria",
			PhotoURL: "https://example.com/u1.jpg",
			Active:   true,
			Interests: domain.Interests{
				Movies: []string{"Dune", "Interstellar"},
				Music:  []string{"Queen"},
				Books:  []string{"1984"},
			},
		},
		{
			ID:       "u2",
			Name:     "Polina",
			PhotoURL: "https://example.com/u2.jpg",
			Active:   true,
			Interests: domain.Interests{
				Movies: []string{"Dune", "Avatar"},
				Books:  []string{"1984"},
			},
		},
		{
			ID:       "u3",
			Name:     "Margo",
			PhotoURL: "https://example.com/u3.jpg",
			Active:   false,
			Interests: domain.Interests{
				Music: []string{"Queen"},
			},
		},
		{
			ID:       "u4",
			Name:     "Arisha",
			PhotoURL: "https://example.com/u4.jpg",
			Active:   true,
			Interests: domain.Interests{
				Movies: []string{"Avatar"},
				Music:  []string{"Muse"},
				Books:  []string{"Hamlet"},
			},
		},
	}
}
