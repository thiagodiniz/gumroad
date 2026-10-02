// Package config reads the service configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr        string
	Environment string
	LogLevel    string

	// Shared secret the Rails client sends as X-Internal-Token. Empty disables the check,
	// which is only acceptable when a NetworkPolicy restricts callers.
	InternalToken string

	// Ruby's Net::HTTP waited its 60s default; providers answer in well under 10s.
	UpstreamTimeout time.Duration
	ShutdownTimeout time.Duration
	MaxRedirects    int
}

func Load() (Config, error) {
	c := Config{
		Addr:          ":" + getenv("PORT", "8080"),
		Environment:   getenv("APP_ENV", "development"),
		LogLevel:      getenv("LOG_LEVEL", "info"),
		InternalToken: os.Getenv("INTERNAL_AUTH_TOKEN"),
	}

	var err error
	if c.UpstreamTimeout, err = duration("UPSTREAM_TIMEOUT", 10*time.Second); err != nil {
		return c, err
	}
	if c.ShutdownTimeout, err = duration("SHUTDOWN_TIMEOUT", 20*time.Second); err != nil {
		return c, err
	}
	if c.MaxRedirects, err = integer("MAX_REDIRECTS", 4); err != nil {
		return c, err
	}
	if c.Environment == "production" && c.InternalToken == "" {
		return c, errors.New("INTERNAL_AUTH_TOKEN is required when APP_ENV=production")
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func integer(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}

// duration accepts Go durations ("30s", "10m") or a bare number of seconds.
func duration(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}
