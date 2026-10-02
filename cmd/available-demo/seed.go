package main

import "github.com/kmradished/GOsapiens/internal/domain"

func demoProfiles() []domain.Profile {
	return []domain.Profile{
		{ID: "u1", Active: true},
		{ID: "u2", Active: true},
		{ID: "u3", Active: false},
		{ID: "u4", Active: true},
		{ID: "u5", Active: true},
		{ID: "u6", Active: true},
	}
}

func demoBlocks() []domain.Block {
	return []domain.Block{
		{BlockerID: "u1", BlockedID: "u5"},
	}
}

func demoViews() []domain.View {
	return []domain.View{
		{ViewerID: "u1", CandidateID: "u4"},
	}
}

func demoReactions() []domain.Reaction {
	return []domain.Reaction{
		{SenderID: "u1", ReceiverID: "u2", Kind: domain.ReactionLike},
	}
}
