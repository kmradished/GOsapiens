package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/matching"
	"github.com/kmradished/GOsapiens/internal/memory"
)

func main() {
	profiles := []domain.Profile{
		{
			ID:       "u1",
			Name:     "Арина",
			Bio:      "Люблю книги",
			PhotoURL: "arina.jpg",
			Active:   true,
		},
		{
			ID:       "u10",
			Name:     "Даша",
			Bio:      "Люблю кино",
			PhotoURL: "dasha.jpg",
			Active:   true,
		},
	}

	reactions := []domain.Reaction{
		{
			SenderID:   "u1",
			ReceiverID: "u10",
			Kind:       domain.ReactionLike,
		},
		{
			SenderID:   "u10",
			ReceiverID: "u1",
			Kind:       domain.ReactionLike,
		},
	}

	waitingSource := memory.NewInteractions(profiles, reactions[:1], nil)
	waitingService := matching.NewService(waitingSource)

	pending, err := waitingService.Resolve("u1", "u10")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Один лайк:", pending.Status)

	_, err = waitingService.FullProfile("u1", "u10")
	if !errors.Is(err, matching.ErrProfileAccessDenied) {
		log.Fatalf("expected access denied, got %v", err)
	}
	fmt.Println("Без мэтча полный профиль закрыт")

	source := memory.NewInteractions(profiles, reactions, nil)
	service := matching.NewService(source)

	created, err := service.Resolve("u1", "u10")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Взаимные лайки:", created.Status)

	again, err := service.Resolve("u10", "u1")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Повторное обращение:", again.Status)

	pairs := [][2]domain.UserID{
		{"u1", "u10"},
		{"u10", "u1"},
	}

	for _, pair := range pairs {
		profile, err := service.FullProfile(pair[0], pair[1])
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf(
			"%s видит профиль %s: %s, %s, %s\n",
			pair[0], profile.ID, profile.Name, profile.Bio, profile.PhotoURL,
		)
	}
}
