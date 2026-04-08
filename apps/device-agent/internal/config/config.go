package config

import (
	"fmt"
	"os"
)

type Config struct {
	DeviceToken  string
	DeviceName   string
	APIEndpoint  string // wss://devices.blutexts.com
	PollInterval int    // milliseconds, default 500
	LogLevel     string
}

func Load() (*Config, error) {
	cfg := &Config{
		DeviceToken:  os.Getenv("DEVICE_TOKEN"),
		DeviceName:   os.Getenv("DEVICE_NAME"),
		APIEndpoint:  getEnv("API_ENDPOINT", "wss://devices.blutexts.com"),
		PollInterval: 500,
		LogLevel:     getEnv("LOG_LEVEL", "info"),
	}

	if cfg.DeviceToken == "" {
		return nil, fmt.Errorf("DEVICE_TOKEN is required — get this from your BlueSend admin panel")
	}
	if cfg.DeviceName == "" {
		hostname, _ := os.Hostname()
		cfg.DeviceName = hostname
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
