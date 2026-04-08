package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Env string
	Port string
	WSPort string

	DatabaseURL string
	RedisURL    string

	JWTSecret        string
	JWTRefreshSecret string

	StripeSecretKey      string
	StripeWebhookSecret  string
	StripePriceSetup     string
	StripePriceMonthly   string
	StripePriceAnnual    string

	GHLClientID      string
	GHLClientSecret  string
	GHLWebhookSecret string

	AppURL       string
	APIURL       string
	DeviceWSURL  string
	AdminAPIKey  string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:    getEnv("ENV", "development"),
		Port:   getEnv("PORT", "8080"),
		WSPort: getEnv("WS_PORT", "8081"),

		DatabaseURL: requireEnv("DATABASE_URL"),
		RedisURL:    requireEnv("REDIS_URL"),

		JWTSecret:        requireEnv("JWT_SECRET"),
		JWTRefreshSecret: requireEnv("JWT_REFRESH_SECRET"),

		StripeSecretKey:     requireEnv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: requireEnv("STRIPE_WEBHOOK_SECRET"),
		StripePriceSetup:    requireEnv("STRIPE_PRICE_SETUP"),
		StripePriceMonthly:  requireEnv("STRIPE_PRICE_MONTHLY"),
		StripePriceAnnual:   requireEnv("STRIPE_PRICE_ANNUAL"),

		GHLClientID:      requireEnv("GHL_CLIENT_ID"),
		GHLClientSecret:  requireEnv("GHL_CLIENT_SECRET"),
		GHLWebhookSecret: requireEnv("GHL_WEBHOOK_SECRET"),

		AppURL:      getEnv("APP_URL", "http://localhost:3000"),
		APIURL:      getEnv("API_URL", "http://localhost:8080"),
		DeviceWSURL: getEnv("DEVICE_WS_URL", "ws://localhost:8081"),
		AdminAPIKey: requireEnv("ADMIN_API_KEY"),
	}
	return cfg, nil
}

func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		// In development, warn but don't crash on missing optional keys
		fmt.Fprintf(os.Stderr, "WARNING: required env var %s is not set\n", key)
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
