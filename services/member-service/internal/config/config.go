package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const minJWTSecretLen = 32

// Config is read only from environment variables (NFR-M4).
type Config struct {
	Port        string
	DatabaseURL string
	RabbitMQURL string
	JWTSecret   string
	JWTTTL      time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Port:        getenv("PORT", "8081"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RabbitMQURL: os.Getenv("RABBITMQ_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.RabbitMQURL == "" {
		return Config{}, errors.New("RABBITMQ_URL is required")
	}
	if len(cfg.JWTSecret) < minJWTSecretLen {
		return Config{}, fmt.Errorf("JWT_SECRET is required and must be at least %d characters", minJWTSecretLen)
	}

	ttl, err := time.ParseDuration(getenv("JWT_TTL", "1h"))
	if err != nil || ttl <= 0 {
		return Config{}, errors.New("JWT_TTL must be a positive duration such as 15m or 1h")
	}
	cfg.JWTTTL = ttl

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
