// Package config loads runtime configuration from environment variables.
// Required vars cause Load to return an error listing every missing key.
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port               string
	DatabaseURL        string
	JWTSecret          string
	AdminPasswordHash  string
	R2AccessKey        string
	R2SecretKey        string
	R2BucketName       string
	R2Endpoint         string
	CORSAllowedOrigins []string
	CookieSecure       bool
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:               getenvDefault("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		AdminPasswordHash:  os.Getenv("ADMIN_PASSWORD"),
		R2AccessKey:        os.Getenv("R2_ACCESS_KEY"),
		R2SecretKey:        os.Getenv("R2_SECRET_KEY"),
		R2BucketName:       os.Getenv("R2_BUCKET_NAME"),
		R2Endpoint:         os.Getenv("R2_ENDPOINT"),
		CORSAllowedOrigins: parseCSV(getenvDefault("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		CookieSecure:       parseBool(getenvDefault("COOKIE_SECURE", "false")),
	}

	required := map[string]string{
		"DATABASE_URL":   cfg.DatabaseURL,
		"JWT_SECRET":     cfg.JWTSecret,
		"ADMIN_PASSWORD": cfg.AdminPasswordHash,
		"R2_ACCESS_KEY":  cfg.R2AccessKey,
		"R2_SECRET_KEY":  cfg.R2SecretKey,
		"R2_BUCKET_NAME": cfg.R2BucketName,
		"R2_ENDPOINT":    cfg.R2Endpoint,
	}

	var missing []string
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
