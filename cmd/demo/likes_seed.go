package main

import "github.com/kmradished/GOsapiens/internal/domain"

// тестовые блокировки
func demoBlocks() []domain.Block {
	return []domain.Block{
		{BlockerID: "u4", BlockedID: "u1"},
	}
}

// тестовые реакции
func demoReactions() []domain.Reaction {
	return []domain.Reaction{
		{
			SenderID:   "u1",
			ReceiverID: "u2",
			Kind:       domain.ReactionLike,
		},
	}
}
