package main

import (
	"log"

	"github.com/kmradished/GOsapiens/internal/discovery/interests"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/memory"
)

func main() {
	profiles := memory.NewProfiles(demoProfiles())
	service := interests.NewService(profiles)

	currentID := domain.UserID("u1")
	cards, err := service.Recommend(currentID)
	if err != nil {
		log.Fatalf("recommend profiles: %v", err)
	}

	printCards(currentID, cards)
	runLikesDemo(profiles)
}
