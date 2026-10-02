package likes

import "github.com/kmradished/GOsapiens/internal/domain"

// проверка правил лайка
func Check(
	user domain.Profile,
	candidate domain.Profile,
	blocked bool,
	alreadyReacted bool,
) Result {
	if user.ID == candidate.ID {
		return Result{Reason: ReasonSelfLike}
	}

	if !user.Active || !candidate.Active {
		return Result{Reason: ReasonInactiveProfile}
	}

	if blocked {
		return Result{Reason: ReasonBlocked}
	}

	if alreadyReacted {
		return Result{Reason: ReasonAlreadyReacted}
	}

	return Result{Allowed: true}
}
