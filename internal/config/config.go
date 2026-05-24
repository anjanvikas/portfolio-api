// Package config loads runtime configuration from environment variables.
// Required vars cause Load to return an error listing every missing key.
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port          string
	DatabaseURL   string
	JWTSecret     string
	AdminPassword string
	R2AccessKey   string
	R2SecretKey   string
	R2BucketName  string
	R2Endpoint    string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:          getenvDefault("PORT", "8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		R2AccessKey:   os.Getenv("R2_ACCESS_KEY"),
		R2SecretKey:   os.Getenv("R2_SECRET_KEY"),
		R2BucketName:  os.Getenv("R2_BUCKET_NAME"),
		R2Endpoint:    os.Getenv("R2_ENDPOINT"),
	}

	required := map[string]string{
		"DATABASE_URL":   cfg.DatabaseURL,
		"JWT_SECRET":     cfg.JWTSecret,
		"ADMIN_PASSWORD": cfg.AdminPassword,
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
