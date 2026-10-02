package matching

import "github.com/kmradished/GOsapiens/internal/domain"

func Decide(
	firstID, secondID domain.UserID,
	firstToSecond, secondToFirst domain.ReactionKind,
	matchExists bool,
) Result {
	match := domain.Match{
		FirstUserID:  firstID,
		SecondUserID: secondID,
	}

	if matchExists {
		return Result{
			Status:                StatusAlreadyExists,
			Match:                 match,
			FullProfilesAvailable: true,
		}
	}

	if firstToSecond != domain.ReactionLike ||
		secondToFirst != domain.ReactionLike {
		return Result{
			Status: StatusWaiting,
		}
	}

	return Result{
		Status:                StatusCreated,
		Match:                 match,
		FullProfilesAvailable: true,
	}
}
