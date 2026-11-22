package likehttp

import (
	"context"
	"errors"
	"net/http"

	"github.com/kmradished/GOsapiens/internal/domain"
)

func writeDatabaseError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, domain.ErrCommitUnconfirmed):
		writeError(w, http.StatusServiceUnavailable, "commit_unconfirmed",
			"Transaction outcome is unknown; read pair state")
	case errors.Is(err, domain.ErrTransactionRetryable):
		writeError(w, http.StatusServiceUnavailable, "transaction_retryable",
			"Transaction could not complete")
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, "database_timeout",
			"Database operation timed out")
	default:
		return false
	}
	return true
}
