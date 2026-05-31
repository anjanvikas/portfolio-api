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
	R2PublicBaseURL    string
	R2ResumeKey        string
	ResendAPIKey       string
	ContactFromEmail   string
	ContactToEmail     string
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
		R2PublicBaseURL:    os.Getenv("R2_PUBLIC_BASE_URL"),
		R2ResumeKey:        getenvDefault("R2_RESUME_KEY", "resume.pdf"),
		ResendAPIKey:       os.Getenv("RESEND_API_KEY"),
		ContactFromEmail:   getenvDefault("CONTACT_FROM_EMAIL", "Portfolio <onboarding@resend.dev>"),
		ContactToEmail:     os.Getenv("CONTACT_TO_EMAIL"),
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

// R2Configured reports whether real R2 credentials are present. The shipped
// .env.example carries placeholder values (empty keys, an "<account-id>"
// endpoint); when those are in effect the resume download falls back to the
// stored resume_url instead of presigning against a bucket that doesn't exist.
func (c *Config) R2Configured() bool {
	return c.R2AccessKey != "" &&
		c.R2SecretKey != "" &&
		c.R2BucketName != "" &&
		c.R2Endpoint != "" &&
		!strings.Contains(c.R2Endpoint, "<")
}

// MailerConfigured reports whether Resend can actually send. When false the
// contact form uses the LogMailer (messages are logged, not emailed).
func (c *Config) MailerConfigured() bool {
	return c.ResendAPIKey != "" && c.ContactToEmail != ""
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
