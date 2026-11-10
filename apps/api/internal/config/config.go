package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	LogLevel    slog.Level
}

func Load() (Config, error) {
	return Parse(os.Getenv)
}

func Parse(getenv func(string) string) (Config, error) {
	cfg := Config{HTTPAddr: getenv("HTTP_ADDR"), DatabaseURL: getenv("DATABASE_URL")}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8090"
	}
	if _, _, err := net.SplitHostPort(cfg.HTTPAddr); err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDR must be a host:port address: %w", err)
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	level := getenv("LOG_LEVEL")
	if level == "" {
		level = "info"
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(level)); err != nil {
		return Config{}, fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}
	return cfg, nil
}
