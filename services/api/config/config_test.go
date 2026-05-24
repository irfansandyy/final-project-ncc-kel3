package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("should_return_defaults_when_env_vars_missing", func(t *testing.T) {
		os.Clearenv()
		cfg := Load()

		if cfg.Port != "8080" {
			t.Errorf("expected port 8080, got %s", cfg.Port)
		}
		if cfg.PostgresDSN != "" {
			t.Errorf("expected empty DSN, got %s", cfg.PostgresDSN)
		}
		if cfg.LogLevel != "info" {
			t.Errorf("expected info log level, got %s", cfg.LogLevel)
		}
		if cfg.ServiceName != "siem-api" {
			t.Errorf("expected siem-api service name, got %s", cfg.ServiceName)
		}
		if cfg.AllowedOrigins != "*" {
			t.Errorf("expected * allowed origins, got %s", cfg.AllowedOrigins)
		}
		if cfg.WSPollIntervalMs != 500 {
			t.Errorf("expected 500 ws poll interval, got %d", cfg.WSPollIntervalMs)
		}
	})

	t.Run("should_return_env_values_when_env_vars_present", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("PORT", "9090")
		os.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/db")
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("SERVICE_NAME", "my-api")
		os.Setenv("ALLOWED_ORIGINS", "http://localhost:3000")
		os.Setenv("WS_POLL_INTERVAL_MS", "1000")

		cfg := Load()

		if cfg.Port != "9090" {
			t.Errorf("expected port 9090, got %s", cfg.Port)
		}
		if cfg.PostgresDSN != "postgres://user:pass@localhost:5432/db" {
			t.Errorf("expected DSN, got %s", cfg.PostgresDSN)
		}
		if cfg.LogLevel != "debug" {
			t.Errorf("expected debug log level, got %s", cfg.LogLevel)
		}
		if cfg.ServiceName != "my-api" {
			t.Errorf("expected my-api service name, got %s", cfg.ServiceName)
		}
		if cfg.AllowedOrigins != "http://localhost:3000" {
			t.Errorf("expected specific origin, got %s", cfg.AllowedOrigins)
		}
		if cfg.WSPollIntervalMs != 1000 {
			t.Errorf("expected 1000 ws poll interval, got %d", cfg.WSPollIntervalMs)
		}
	})

	t.Run("should_return_default_int_when_env_var_is_invalid", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("WS_POLL_INTERVAL_MS", "invalid_int")

		cfg := Load()

		if cfg.WSPollIntervalMs != 500 {
			t.Errorf("expected default 500 ws poll interval for invalid int, got %d", cfg.WSPollIntervalMs)
		}
	})
}
