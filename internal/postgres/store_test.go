package postgres

import (
	"context"
	"testing"
)

func TestOpenInvalidDatabaseURLDoesNotExposePassword(t *testing.T) {
	databaseURL := "postgres://localhost/gosapiens?password=test-password&connect_timeout=invalid"
	_, err := Open(context.Background(), databaseURL)
	if err == nil || err.Error() != "invalid DATABASE_URL" {
		t.Fatal("expected invalid DATABASE_URL without connection details")
	}
}
