package main

import (
	"fmt"
	"log"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likes"
	"github.com/kmradished/GOsapiens/internal/memory"
)

// примеры проверки лайка
func runLikesDemo(profiles *memory.Profiles) {
	state := memory.NewLikeState(demoBlocks(), demoReactions())
	service := likes.NewService(profiles, state)

	fmt.Println("\nпроверка лайков")

	printLikeCheck(service, "u2", "u1")
	printLikeCheck(service, "u1", "u1")
	printLikeCheck(service, "u1", "u3")
	printLikeCheck(service, "u1", "u4")
	printLikeCheck(service, "u1", "u2")
	printLikeCheck(service, "u1", "u5")
}

// вывод результата
func printLikeCheck(service *likes.Service, userID, candidateID string) {
	result, err := service.Check(
		domain.UserID(userID),
		domain.UserID(candidateID),
	)
	if err != nil {
		log.Fatalf("проверка лайка: %v", err)
	}

	fmt.Printf(
		"%s -> %s: разрешён=%t, причина=%s\n",
		userID, candidateID, result.Allowed, result.Reason,
	)
}
