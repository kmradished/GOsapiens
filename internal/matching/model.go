package matching

import "github.com/kmradished/GOsapiens/internal/domain"

type Status string

const (
	StatusCreated       Status = "match_created"
	StatusWaiting       Status = "waiting_for_like"
	StatusAlreadyExists Status = "match_already_exists"
)

type Result struct {
	Status                Status
	Match                 domain.Match
	FullProfilesAvailable bool
}
