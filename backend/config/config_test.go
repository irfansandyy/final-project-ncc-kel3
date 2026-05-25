package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	os.Setenv("APP_PORT", "9000")
	os.Setenv("LLM_CTX_SIZE", "2048")
	os.Setenv("RATE_LIMIT_RPS", "10.5")

	defer func() {
		os.Unsetenv("APP_PORT")
		os.Unsetenv("LLM_CTX_SIZE")
		os.Unsetenv("RATE_LIMIT_RPS")
	}()

	cfg := Load()

	if cfg.AppPort != "9000" {
		t.Errorf("Expected AppPort 9000, got %s", cfg.AppPort)
	}
	if cfg.LLMCtxSize != 2048 {
		t.Errorf("Expected LLMCtxSize 2048, got %d", cfg.LLMCtxSize)
	}
	if cfg.RateLimitRequestsPerS != 10.5 {
		t.Errorf("Expected RateLimitRequestsPerS 10.5, got %f", cfg.RateLimitRequestsPerS)
	}
}

func TestGetEnv(t *testing.T) {
	os.Clearenv()

	if val := getEnv("TEST_KEY", "fallback"); val != "fallback" {
		t.Errorf("Expected fallback, got %s", val)
	}

	os.Setenv("TEST_KEY", "value")
	if val := getEnv("TEST_KEY", "fallback"); val != "value" {
		t.Errorf("Expected value, got %s", val)
	}
}

func TestGetEnvInt(t *testing.T) {
	os.Clearenv()

	if val := getEnvInt("TEST_INT", 10); val != 10 {
		t.Errorf("Expected 10, got %d", val)
	}

	os.Setenv("TEST_INT", "invalid")
	if val := getEnvInt("TEST_INT", 10); val != 10 {
		t.Errorf("Expected 10, got %d", val)
	}

	os.Setenv("TEST_INT", "20")
	if val := getEnvInt("TEST_INT", 10); val != 20 {
		t.Errorf("Expected 20, got %d", val)
	}
}

func TestGetEnvFloat(t *testing.T) {
	os.Clearenv()

	if val := getEnvFloat("TEST_FLOAT", 1.5); val != 1.5 {
		t.Errorf("Expected 1.5, got %f", val)
	}

	os.Setenv("TEST_FLOAT", "invalid")
	if val := getEnvFloat("TEST_FLOAT", 1.5); val != 1.5 {
		t.Errorf("Expected 1.5, got %f", val)
	}

	os.Setenv("TEST_FLOAT", "2.5")
	if val := getEnvFloat("TEST_FLOAT", 1.5); val != 2.5 {
		t.Errorf("Expected 2.5, got %f", val)
	}
}
