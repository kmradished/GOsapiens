package main

import (
	"fmt"
	"log"

	"github.com/kmradished/GOsapiens/internal/discovery/available"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/memory"
)

func main() {
	profiles := memory.NewProfiles(demoProfiles())
	exclusions := available.NewMemoryExclusions(
		demoBlocks(),
		demoViews(),
		demoReactions(),
	)
	service := available.NewService(profiles, exclusions)

	currentID := domain.UserID("u1")
	candidates, err := service.Recommend(currentID)
	if err != nil {
		log.Fatalf("get available candidates: %v", err)
	}

	fmt.Printf("Available candidates for %s:\n", currentID)
	for _, candidate := range candidates {
		fmt.Printf("candidate %s\n", candidate.ID)
	}
}
