package config

import (
	"fmt"
	"os"
)

type Config struct {
	Env  string
	Port string

	DatabaseURL string
	RedisURL    string

	JWTSecret        string
	JWTRefreshSecret string

	// Stripe (optional — free mode when empty)
	StripeSecretKey     string
	StripeWebhookSecret string
	StripePriceSetup    string
	StripePriceMonthly  string
	StripePriceAnnual   string

	GHLClientID         string
	GHLClientSecret     string
	GHLWebhookSecret    string // HMAC key for verifying inbound /api/webhooks/inbound POSTs

	// EncryptionKey is a 32-byte hex string used for AES-256-GCM envelope
	// encryption of sensitive at-rest data (currently: GHL OAuth refresh
	// tokens). Set via ENCRYPTION_KEY env var. If empty, the server starts
	// but at-rest encryption falls back to plaintext (logged loudly).
	EncryptionKey string

	// Resend (transactional email — logs to stdout when empty)
	ResendAPIKey string
	FromEmail    string
	// Where ops alerts go (new-paid-customer-needs-a-number, etc.).
	// Defaults to centroneaj@gmail.com so the founder is always paged
	// even if the env var was forgotten in a fresh deploy.
	OpsAlertEmail string

	// Agora Voice (FaceTime Audio bridge — calling is disabled when empty)
	AgoraAppID         string
	AgoraAppCertificate string

	AppURL      string
	DeviceWSURL string
	AdminAPIKey string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:  getEnv("ENV", "development"),
		Port: getEnv("PORT", "8080"),

		DatabaseURL: requireEnv("DATABASE_URL"),
		RedisURL:    requireEnv("REDIS_URL"),

		JWTSecret:        requireEnv("JWT_SECRET"),
		JWTRefreshSecret: requireEnv("JWT_REFRESH_SECRET"),

		// Stripe vars are optional
		StripeSecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		StripePriceSetup:    os.Getenv("STRIPE_PRICE_SETUP"),
		StripePriceMonthly:  os.Getenv("STRIPE_PRICE_MONTHLY"),
		StripePriceAnnual:   os.Getenv("STRIPE_PRICE_ANNUAL"),

		GHLClientID:      requireEnv("GHL_CLIENT_ID"),
		GHLClientSecret:  requireEnv("GHL_CLIENT_SECRET"),
		GHLWebhookSecret: requireEnv("GHL_WEBHOOK_SECRET"),

		EncryptionKey: os.Getenv("ENCRYPTION_KEY"),

		// Resend (optional — logs emails when empty)
		ResendAPIKey:  os.Getenv("RESEND_API_KEY"),
		FromEmail:     getEnv("FROM_EMAIL", "BluTexts <noreply@blutexts.com>"),
		OpsAlertEmail: getEnv("OPS_ALERT_EMAIL", "centroneaj@gmail.com"),

		// Agora (optional — calling feature disabled when either is empty)
		AgoraAppID:          os.Getenv("AGORA_APP_ID"),
		AgoraAppCertificate: os.Getenv("AGORA_APP_CERTIFICATE"),

		AppURL:      getEnv("APP_URL", "http://localhost:3000"),
		DeviceWSURL: getEnv("DEVICE_WS_URL", "ws://localhost:8081"),
		AdminAPIKey: requireEnv("ADMIN_API_KEY"),
	}
	return cfg, nil
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
		fmt.Fprintf(os.Stderr, "WARNING: required env var %s is not set\n", key)
	}
	return v
}
