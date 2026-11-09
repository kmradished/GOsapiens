package likehttp

import (
	"io"
	"net/http"
)

func requestBodyIsEmpty(r *http.Request) bool {
	if r.ContentLength > 0 {
		return false
	}
	if r.Body == nil {
		return true
	}

	data, err := io.ReadAll(io.LimitReader(r.Body, 1))
	return err == nil && len(data) == 0
}

func validateEmptyInput(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.RawQuery != "" {
		writeError(w, http.StatusBadRequest, "invalid_query",
			"This route does not accept query parameters")
		return false
	}

	if !requestBodyIsEmpty(r) {
		writeError(w, http.StatusBadRequest, "invalid_body",
			"Request body must be empty")
		return false
	}

	return true
}
