package matching

import (
	"errors"
	"fmt"

	"github.com/kmradished/GOsapiens/internal/domain"
)

var ErrInvalidPair = errors.New("user IDs must be non-empty and different")
var ErrProfileAccessDenied = errors.New("full profile is available only after a match")

type DataSource interface {
	HasMatch(firstID, secondID domain.UserID) (bool, error)

	GetReaction(senderID, receiverID domain.UserID) (domain.ReactionKind, error)

	SaveMatch(match domain.Match) error

	GetProfile(id domain.UserID) (domain.Profile, error)
}

type Service struct {
	source DataSource
}

func NewService(source DataSource) *Service {
	return &Service{
		source: source,
	}
}
func (s *Service) Resolve(
	firstID, secondID domain.UserID,
) (Result, error) {
	if firstID == "" || secondID == "" || firstID == secondID {
		return Result{}, ErrInvalidPair
	}

	matchExists, err := s.source.HasMatch(firstID, secondID)
	if err != nil {
		return Result{}, fmt.Errorf("check match: %w", err)
	}

	if matchExists {
		return Decide(firstID, secondID, "", "", true), nil
	}

	firstToSecond, err := s.source.GetReaction(firstID, secondID)
	if err != nil {
		return Result{}, fmt.Errorf(
			"get reaction from %q to %q: %w",
			firstID, secondID, err,
		)
	}

	secondToFirst, err := s.source.GetReaction(secondID, firstID)
	if err != nil {
		return Result{}, fmt.Errorf(
			"get reaction from %q to %q: %w",
			secondID, firstID, err,
		)
	}

	result := Decide(
		firstID,
		secondID,
		firstToSecond,
		secondToFirst,
		false,
	)

	if result.Status == StatusCreated {
		err := s.source.SaveMatch(result.Match)
		if err != nil {
			return Result{}, fmt.Errorf("save match: %w", err)
		}
	}

	return result, nil
}
func (s *Service) FullProfile(
	viewerID, profileID domain.UserID,
) (domain.Profile, error) {
	if viewerID == "" || profileID == "" || viewerID == profileID {
		return domain.Profile{}, ErrInvalidPair
	}

	matchExists, err := s.source.HasMatch(viewerID, profileID)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("check profile access: %w", err)
	}

	if !matchExists {
		return domain.Profile{}, ErrProfileAccessDenied
	}

	profile, err := s.source.GetProfile(profileID)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("get full profile %q: %w", profileID, err)
	}

	return profile, nil
}
