package demodata

import "github.com/kmradished/GOsapiens/internal/domain"

// Profiles задаёт одинаковые учебные данные для приложения и Profile API.
func Profiles() []domain.Profile {
	dune := domain.Interests{Movies: []string{"Dune"}}
	return []domain.Profile{
		{ID: "u1", Active: true, Interests: dune},
		{ID: "u6", Active: true, Interests: dune},
		{ID: "u2", Active: true, Interests: dune},
		{ID: "u3", Active: false, Interests: dune},
		{ID: "u5", Active: true, Interests: dune},
		{ID: "u4", Active: true, Interests: dune},
		{ID: "u7", Active: true, Interests: dune},
	}
}

func Blocks() []domain.Block {
	return []domain.Block{{BlockerID: "u7", BlockedID: "u1"}}
}
