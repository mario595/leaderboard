package config

import (
	"fmt"
	"slices"
)

// Config holds the application settings, loaded from environment variables
type Config struct {
	Addr               string
	Env                string
	LogLevel           string
	DBPath             string
	StravaClientID     string
	StravaClientSecret string
	BaseURL            string
}

var validEnvs = []string{"dev", "prod"}
var validLogLevels = []string{"debug", "info", "warn", "error"}

func getOr(getenv func(string) string, key, def string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return def
}

// Load loads the config using getenv. By default, it uses ADDR=:8080, ENV=dev, LOG_LEVEL=info
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Addr:               getOr(getenv, "ADDR", ":8080"),
		Env:                getOr(getenv, "ENV", "dev"),
		LogLevel:           getOr(getenv, "LOG_LEVEL", "info"),
		DBPath:             getOr(getenv, "DB_PATH", "data/leaderboard.db"),
		StravaClientID:     getOr(getenv, "STRAVA_CLIENT_ID", ""),
		StravaClientSecret: getOr(getenv, "STRAVA_CLIENT_SECRET", ""),
		BaseURL:            getOr(getenv, "BASE_URL", "http://localhost:8080"),
	}

	if !slices.Contains(validEnvs, cfg.Env) {
		return Config{}, fmt.Errorf("invalid ENV %q: must be one of %v", cfg.Env, validEnvs)
	}

	if !slices.Contains(validLogLevels, cfg.LogLevel) {
		return Config{}, fmt.Errorf("invalid LOG_LEVEL %q: must be one of %v", cfg.LogLevel, validLogLevels)
	}

	return cfg, nil
}
