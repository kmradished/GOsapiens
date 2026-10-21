package memory_test

import (
	"errors"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/memory"
)

func TestProfilesGetReturnsDomainNotFound(t *testing.T) {
	profile, err := memory.NewProfiles(nil).Get("missing")
	if !errors.Is(err, domain.ErrProfileNotFound) {
		t.Fatalf("Get error = %v; want domain.ErrProfileNotFound", err)
	}
	if profile.ID != "" || profile.Active {
		t.Fatalf("Get returned a profile on error: %+v", profile)
	}
	if !errors.Is(err, memory.ErrProfileNotFound) {
		t.Fatalf("Get error lost compatibility with memory.ErrProfileNotFound: %v", err)
	}
}
