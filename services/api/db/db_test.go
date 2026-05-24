package db

import (
	"testing"
)

func TestConnect(t *testing.T) {
	t.Run("should_return_error_when_connection_fails", func(t *testing.T) {
		// An invalid DSN or unavailable database will cause Ping to fail.
		_, err := Connect("postgres://invalid:invalid@localhost:5432/invalid")
		if err == nil {
			t.Errorf("expected error when connecting to an invalid database, got nil")
		}
	})

	t.Run("should_return_error_when_dsn_is_invalid", func(t *testing.T) {
		_, err := Connect("not-a-dsn")
		if err == nil {
			t.Errorf("expected error for malformed DSN, got nil")
		}
	})
}
