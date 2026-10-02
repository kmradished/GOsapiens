package interests

import "github.com/kmradished/GOsapiens/internal/domain"

type AnonymousCard struct {
	CandidateID  domain.UserID
	CommonMovies []string
	CommonMusic  []string
	CommonBooks  []string
}
