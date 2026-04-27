package config

import (
	"errors"
	"fmt"
	"os"
)

const (
	defaultPort             = "8181"
	defaultProvisionerToken = "dev-token"
)

type Config struct {
	DatabaseURL      string
	Port             string
	ProvisionerToken string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		Port:             envOrDefault("PROVISIONER_PORT", defaultPort),
		ProvisionerToken: envOrDefault("PROVISIONER_TOKEN", defaultProvisionerToken),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	return cfg, nil
}

func (c Config) HTTPAddr() string {
	return fmt.Sprintf(":%s", c.Port)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
