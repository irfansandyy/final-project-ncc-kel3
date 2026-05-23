package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port               string
	PostgresDSN        string
	LogLevel           string
	ServiceName        string
	AllowedOrigins     string
	WSPollIntervalMs   int
}

func Load() *Config {
	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		PostgresDSN:      os.Getenv("POSTGRES_DSN"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		ServiceName:      getEnv("SERVICE_NAME", "siem-api"),
		AllowedOrigins:   getEnv("ALLOWED_ORIGINS", "*"),
		WSPollIntervalMs: getEnvInt("WS_POLL_INTERVAL_MS", 500),
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, exists := os.LookupEnv(key); exists {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}
