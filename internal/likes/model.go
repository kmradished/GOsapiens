package likes

// причина отказа в лайке
type Reason string

const (
	ReasonNone            Reason = ""                  // лайк разрешён
	ReasonSelfLike        Reason = "self_like"         // лайк себе
	ReasonProfileNotFound Reason = "profile_not_found" // профиль не найден
	ReasonInactiveProfile Reason = "inactive_profile"  // профиль неактивен
	ReasonBlocked         Reason = "blocked"           // блокировка
	ReasonAlreadyReacted  Reason = "already_reacted"   // уже был лайк или дизлайк
)

// результат проверки лайка
type Result struct {
	Allowed bool
	Reason  Reason
}
