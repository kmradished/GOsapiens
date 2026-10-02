package main

import (
	"fmt"

	"github.com/kmradished/GOsapiens/internal/discovery/interests"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func printCards(currentID domain.UserID, cards []interests.AnonymousCard) {
	fmt.Printf("Recommendations for %s:\n", currentID)
	for _, card := range cards {
		fmt.Printf(
			"candidate %s movies=%v music=%v books=%v\n",
			card.CandidateID,
			card.CommonMovies,
			card.CommonMusic,
			card.CommonBooks,
		)
	}
}
