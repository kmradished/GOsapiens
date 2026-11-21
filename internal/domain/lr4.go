package domain

import "errors"

var (
	ErrForbidden            = errors.New("operation is forbidden")
	ErrVersionConflict      = errors.New("profile version conflict")
	ErrBlocked              = errors.New("pair is blocked")
	ErrTransactionRetryable = errors.New("transaction can be retried")
	ErrCommitUnconfirmed    = errors.New("transaction commit is unconfirmed")
)

// PairChange — результат блокировки или отзыва собственного лайка.
type PairChange struct {
	Changed bool
	Matched bool
}

// ProfileRevision содержит редактируемые поля собственного профиля.
type ProfileRevision struct {
	ID      UserID
	Name    string
	Bio     string
	Version int64
}

type ProfileEdit struct {
	Name            string
	Bio             string
	ExpectedVersion int64
}
