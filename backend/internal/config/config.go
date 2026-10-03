package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL, HTTPAddr, FrontendOrigin, AppEnv, CookieSameSite, MetricsToken string
	CookieSecure                                                                bool
	SessionDuration                                                             time.Duration
}

func Load() Config {
	return Config{
		DatabaseURL:     env("DATABASE_URL", "postgres://finance:finance@localhost:5432/finance?sslmode=disable"),
		HTTPAddr:        httpAddr(),
		FrontendOrigin:  env("FRONTEND_ORIGIN", "http://localhost:5173"),
		AppEnv:          env("APP_ENV", "development"),
		CookieSecure:    envBool("COOKIE_SECURE", false),
		CookieSameSite:  env("COOKIE_SAME_SITE", "lax"),
		MetricsToken:    env("METRICS_TOKEN", ""),
		SessionDuration: time.Duration(envInt("SESSION_DURATION_DAYS", 30)) * 24 * time.Hour,
	}
}

func httpAddr() string {
	if value := os.Getenv("HTTP_ADDR"); value != "" {
		return value
	}
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	return ":8080"
}
func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
