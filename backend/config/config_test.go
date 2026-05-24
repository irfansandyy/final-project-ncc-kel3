package config

import (
	"testing"
	"time"
)

func Test_Should_ReturnAllDefaults_When_NoEnvVarsSet(t *testing.T) {
	// Ensure relevant env vars are unset (t.Setenv restores after test).
	envKeys := []string{
		"APP_PORT", "DATABASE_URL", "JWT_SECRET", "JWT_TTL_MINUTES",
		"ALLOWED_ORIGIN", "LLM_BASE_URL", "LLM_MODEL", "LLM_CTX_SIZE",
		"LLM_TIMEOUT_SECONDS", "DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS",
		"DB_CONN_MAX_LIFETIME_MINUTES", "RATE_LIMIT_RPS", "RATE_LIMIT_BURST",
	}
	for _, k := range envKeys {
		t.Setenv(k, "")
	}

	cfg := Load()

	if cfg.AppPort != "8000" {
		t.Errorf("AppPort = %q, want %q", cfg.AppPort, "8000")
	}
	if cfg.DatabaseURL != "postgres://postgres:postgres@localhost:5432/chatdb?sslmode=disable" {
		t.Errorf("DatabaseURL = %q, want default", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != "change-me-in-production" {
		t.Errorf("JWTSecret = %q, want default", cfg.JWTSecret)
	}
	expectedTTL := time.Duration(60*24) * time.Minute
	if cfg.TokenTTL != expectedTTL {
		t.Errorf("TokenTTL = %v, want %v", cfg.TokenTTL, expectedTTL)
	}
	if cfg.AllowedOrigin != "http://localhost:3000" {
		t.Errorf("AllowedOrigin = %q, want default", cfg.AllowedOrigin)
	}
	if cfg.LLMBaseURL != "http://localhost:8081" {
		t.Errorf("LLMBaseURL = %q, want default", cfg.LLMBaseURL)
	}
	if cfg.LLMModel != "hf.co/bartowski/Llama-3.2-3B-Instruct-GGUF:Q6_K" {
		t.Errorf("LLMModel = %q, want default", cfg.LLMModel)
	}
	if cfg.LLMCtxSize != 4096 {
		t.Errorf("LLMCtxSize = %d, want 4096", cfg.LLMCtxSize)
	}
	if cfg.LLMTimeout != 60*time.Second {
		t.Errorf("LLMTimeout = %v, want 60s", cfg.LLMTimeout)
	}
	if cfg.DBMaxOpenConns != 25 {
		t.Errorf("DBMaxOpenConns = %d, want 25", cfg.DBMaxOpenConns)
	}
	if cfg.DBMaxIdleConns != 25 {
		t.Errorf("DBMaxIdleConns = %d, want 25", cfg.DBMaxIdleConns)
	}
	if cfg.DBConnMaxLifetime != 5*time.Minute {
		t.Errorf("DBConnMaxLifetime = %v, want 5m", cfg.DBConnMaxLifetime)
	}
	if cfg.RateLimitRequestsPerS != 5 {
		t.Errorf("RateLimitRequestsPerS = %f, want 5", cfg.RateLimitRequestsPerS)
	}
	if cfg.RateLimitBurstRequests != 10 {
		t.Errorf("RateLimitBurstRequests = %d, want 10", cfg.RateLimitBurstRequests)
	}
}

func Test_Should_UseCustomValues_When_AllEnvVarsSet(t *testing.T) {
	t.Setenv("APP_PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://custom:custom@db:5432/mydb")
	t.Setenv("JWT_SECRET", "super-secret-key")
	t.Setenv("JWT_TTL_MINUTES", "120")
	t.Setenv("ALLOWED_ORIGIN", "https://example.com")
	t.Setenv("LLM_BASE_URL", "http://llm:8080")
	t.Setenv("LLM_MODEL", "custom-model")
	t.Setenv("LLM_CTX_SIZE", "8192")
	t.Setenv("LLM_TIMEOUT_SECONDS", "30")
	t.Setenv("DB_MAX_OPEN_CONNS", "50")
	t.Setenv("DB_MAX_IDLE_CONNS", "10")
	t.Setenv("DB_CONN_MAX_LIFETIME_MINUTES", "15")
	t.Setenv("RATE_LIMIT_RPS", "10.5")
	t.Setenv("RATE_LIMIT_BURST", "20")

	cfg := Load()

	if cfg.AppPort != "9090" {
		t.Errorf("AppPort = %q, want %q", cfg.AppPort, "9090")
	}
	if cfg.DatabaseURL != "postgres://custom:custom@db:5432/mydb" {
		t.Errorf("DatabaseURL = %q, want custom", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != "super-secret-key" {
		t.Errorf("JWTSecret = %q, want custom", cfg.JWTSecret)
	}
	if cfg.TokenTTL != 120*time.Minute {
		t.Errorf("TokenTTL = %v, want 120m", cfg.TokenTTL)
	}
	if cfg.AllowedOrigin != "https://example.com" {
		t.Errorf("AllowedOrigin = %q, want custom", cfg.AllowedOrigin)
	}
	if cfg.LLMBaseURL != "http://llm:8080" {
		t.Errorf("LLMBaseURL = %q, want custom", cfg.LLMBaseURL)
	}
	if cfg.LLMModel != "custom-model" {
		t.Errorf("LLMModel = %q, want custom", cfg.LLMModel)
	}
	if cfg.LLMCtxSize != 8192 {
		t.Errorf("LLMCtxSize = %d, want 8192", cfg.LLMCtxSize)
	}
	if cfg.LLMTimeout != 30*time.Second {
		t.Errorf("LLMTimeout = %v, want 30s", cfg.LLMTimeout)
	}
	if cfg.DBMaxOpenConns != 50 {
		t.Errorf("DBMaxOpenConns = %d, want 50", cfg.DBMaxOpenConns)
	}
	if cfg.DBMaxIdleConns != 10 {
		t.Errorf("DBMaxIdleConns = %d, want 10", cfg.DBMaxIdleConns)
	}
	if cfg.DBConnMaxLifetime != 15*time.Minute {
		t.Errorf("DBConnMaxLifetime = %v, want 15m", cfg.DBConnMaxLifetime)
	}
	if cfg.RateLimitRequestsPerS != 10.5 {
		t.Errorf("RateLimitRequestsPerS = %f, want 10.5", cfg.RateLimitRequestsPerS)
	}
	if cfg.RateLimitBurstRequests != 20 {
		t.Errorf("RateLimitBurstRequests = %d, want 20", cfg.RateLimitBurstRequests)
	}
}

func Test_Should_FallbackToDefault_When_IntEnvVarIsNonNumeric(t *testing.T) {
	t.Setenv("LLM_CTX_SIZE", "not-a-number")
	t.Setenv("DB_MAX_OPEN_CONNS", "abc")
	t.Setenv("DB_MAX_IDLE_CONNS", "12.5") // float string for int field
	t.Setenv("RATE_LIMIT_BURST", "xyz")
	t.Setenv("JWT_TTL_MINUTES", "!!!")
	t.Setenv("LLM_TIMEOUT_SECONDS", "slow")
	t.Setenv("DB_CONN_MAX_LIFETIME_MINUTES", "forever")

	cfg := Load()

	if cfg.LLMCtxSize != 4096 {
		t.Errorf("LLMCtxSize = %d, want default 4096 on invalid input", cfg.LLMCtxSize)
	}
	if cfg.DBMaxOpenConns != 25 {
		t.Errorf("DBMaxOpenConns = %d, want default 25 on invalid input", cfg.DBMaxOpenConns)
	}
	if cfg.DBMaxIdleConns != 25 {
		t.Errorf("DBMaxIdleConns = %d, want default 25 on invalid input", cfg.DBMaxIdleConns)
	}
	if cfg.RateLimitBurstRequests != 10 {
		t.Errorf("RateLimitBurstRequests = %d, want default 10 on invalid input", cfg.RateLimitBurstRequests)
	}
	expectedTTL := time.Duration(60*24) * time.Minute
	if cfg.TokenTTL != expectedTTL {
		t.Errorf("TokenTTL = %v, want default %v on invalid input", cfg.TokenTTL, expectedTTL)
	}
	if cfg.LLMTimeout != 60*time.Second {
		t.Errorf("LLMTimeout = %v, want default 60s on invalid input", cfg.LLMTimeout)
	}
	if cfg.DBConnMaxLifetime != 5*time.Minute {
		t.Errorf("DBConnMaxLifetime = %v, want default 5m on invalid input", cfg.DBConnMaxLifetime)
	}
}

func Test_Should_FallbackToDefault_When_FloatEnvVarIsInvalid(t *testing.T) {
	t.Setenv("RATE_LIMIT_RPS", "not-a-float")

	cfg := Load()

	if cfg.RateLimitRequestsPerS != 5 {
		t.Errorf("RateLimitRequestsPerS = %f, want default 5 on invalid input", cfg.RateLimitRequestsPerS)
	}
}

func Test_Should_UseDefault_When_EnvVarIsEmptyString(t *testing.T) {
	// Explicitly set all to empty to exercise the empty-string branch.
	t.Setenv("APP_PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("JWT_TTL_MINUTES", "")
	t.Setenv("RATE_LIMIT_RPS", "")

	cfg := Load()

	if cfg.AppPort != "8000" {
		t.Errorf("AppPort = %q, want default when empty", cfg.AppPort)
	}
	if cfg.DatabaseURL != "postgres://postgres:postgres@localhost:5432/chatdb?sslmode=disable" {
		t.Errorf("DatabaseURL = %q, want default when empty", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != "change-me-in-production" {
		t.Errorf("JWTSecret = %q, want default when empty", cfg.JWTSecret)
	}
	expectedTTL := time.Duration(60*24) * time.Minute
	if cfg.TokenTTL != expectedTTL {
		t.Errorf("TokenTTL = %v, want default %v when empty", cfg.TokenTTL, expectedTTL)
	}
	if cfg.RateLimitRequestsPerS != 5 {
		t.Errorf("RateLimitRequestsPerS = %f, want default 5 when empty", cfg.RateLimitRequestsPerS)
	}
}

func Test_Should_ParseZero_When_IntEnvVarIsZero(t *testing.T) {
	t.Setenv("LLM_CTX_SIZE", "0")
	t.Setenv("RATE_LIMIT_BURST", "0")

	cfg := Load()

	if cfg.LLMCtxSize != 0 {
		t.Errorf("LLMCtxSize = %d, want 0", cfg.LLMCtxSize)
	}
	if cfg.RateLimitBurstRequests != 0 {
		t.Errorf("RateLimitBurstRequests = %d, want 0", cfg.RateLimitBurstRequests)
	}
}

func Test_Should_ParseZeroFloat_When_FloatEnvVarIsZero(t *testing.T) {
	t.Setenv("RATE_LIMIT_RPS", "0")

	cfg := Load()

	if cfg.RateLimitRequestsPerS != 0 {
		t.Errorf("RateLimitRequestsPerS = %f, want 0", cfg.RateLimitRequestsPerS)
	}
}

func Test_Should_ParseNegative_When_IntEnvVarIsNegative(t *testing.T) {
	t.Setenv("LLM_CTX_SIZE", "-1")

	cfg := Load()

	if cfg.LLMCtxSize != -1 {
		t.Errorf("LLMCtxSize = %d, want -1", cfg.LLMCtxSize)
	}
}

func Test_Should_ParseNegativeFloat_When_FloatEnvVarIsNegative(t *testing.T) {
	t.Setenv("RATE_LIMIT_RPS", "-2.5")

	cfg := Load()

	if cfg.RateLimitRequestsPerS != -2.5 {
		t.Errorf("RateLimitRequestsPerS = %f, want -2.5", cfg.RateLimitRequestsPerS)
	}
}

func Test_Should_PartialOverride_When_SomeEnvVarsSet(t *testing.T) {
	t.Setenv("APP_PORT", "3000")
	t.Setenv("JWT_SECRET", "my-secret")
	// Leave everything else at default (empty).
	t.Setenv("DATABASE_URL", "")
	t.Setenv("LLM_CTX_SIZE", "")

	cfg := Load()

	if cfg.AppPort != "3000" {
		t.Errorf("AppPort = %q, want %q", cfg.AppPort, "3000")
	}
	if cfg.JWTSecret != "my-secret" {
		t.Errorf("JWTSecret = %q, want %q", cfg.JWTSecret, "my-secret")
	}
	// Unchanged fields should still have defaults.
	if cfg.DatabaseURL != "postgres://postgres:postgres@localhost:5432/chatdb?sslmode=disable" {
		t.Errorf("DatabaseURL should remain default")
	}
	if cfg.LLMCtxSize != 4096 {
		t.Errorf("LLMCtxSize should remain default 4096")
	}
}
