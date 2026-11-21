package application

import (
	"context"

	"github.com/kmradished/GOsapiens/internal/domain"
)

// Интерфейсы ЛР4 согласованы до реализации. В PostgreSQL каждая команда
// самостоятельно открывает транзакцию и использует общий lockPair.

type LikeStore interface {
	Like(ctx context.Context, actorID, targetID domain.UserID) (domain.LikeDecision, error)
	PairMatched(ctx context.Context, actorID, targetID domain.UserID) (bool, error)
}

type BlockStore interface {
	Block(ctx context.Context, actorID, targetID domain.UserID) (domain.PairChange, error)
}

type UnlikeStore interface {
	Unlike(ctx context.Context, actorID, targetID domain.UserID) (domain.PairChange, error)
}

type ProfileStore interface {
	ReadProfile(ctx context.Context, actorID, profileID domain.UserID) (domain.ProfileRevision, error)
	UpdateProfile(ctx context.Context, actorID, profileID domain.UserID, edit domain.ProfileEdit) (domain.ProfileRevision, error)
}

type DiscoveryStore interface {
	Profiles(ctx context.Context) ([]domain.Profile, error)
	LikedIDs(ctx context.Context, actorID domain.UserID) ([]domain.UserID, error)
	BlockedIDs(ctx context.Context, actorID domain.UserID) ([]domain.UserID, error)
}
