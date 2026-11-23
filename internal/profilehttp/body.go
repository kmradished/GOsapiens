package profilehttp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/kmradished/GOsapiens/internal/domain"
)

const maxBodyBytes = 4096

type editRequest struct {
	Name    *string
	Bio     *string
	Version *int64
}

func decodeEdit(w http.ResponseWriter, r *http.Request) (domain.ProfileEdit, bool) {
	if r.Body == nil {
		return domain.ProfileEdit{}, false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil || !utf8.Valid(body) {
		return domain.ProfileEdit{}, false
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return domain.ProfileEdit{}, false
	}
	var request editRequest
	fields := map[string]any{"name": &request.Name, "bio": &request.Bio, "version": &request.Version}
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		field, allowed := fields[key]
		if err != nil || !ok || !allowed || decoder.Decode(field) != nil {
			return domain.ProfileEdit{}, false
		}
		delete(fields, key)
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		return domain.ProfileEdit{}, false
	}
	if _, err := decoder.Token(); err != io.EOF {
		return domain.ProfileEdit{}, false
	}
	if request.Name == nil || request.Bio == nil || request.Version == nil || *request.Version <= 0 {
		return domain.ProfileEdit{}, false
	}

	name := strings.TrimSpace(*request.Name)
	bio := *request.Bio
	if name == "" || utf8.RuneCountInString(name) > 100 || utf8.RuneCountInString(bio) > 2000 ||
		strings.ContainsRune(name, 0) || strings.ContainsRune(bio, 0) {
		return domain.ProfileEdit{}, false
	}
	return domain.ProfileEdit{Name: name, Bio: bio, ExpectedVersion: *request.Version}, true
}
