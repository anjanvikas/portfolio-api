// gen-homepage-og renders the static homepage OG card and uploads it to R2 at
// og/homepage.png. One-shot tool — homepage name/headline change rarely; the
// public URL is pinned via NEXT_PUBLIC_HOMEPAGE_OG_URL on the frontend.
//
// Usage:
//
//	cd backend && go run ./cmd/gen-homepage-og
//	# or
//	make og-homepage
//
// Inputs (env / .env):
//
//	R2_*                — same vars the API uses (must point to a real bucket)
//	SITE_URL            — site origin shown in the card band (default localhost)
//	OG_HOMEPAGE_NAME    — display name (default "Anjan Vikas Reddy")
//	OG_HOMEPAGE_HEADLINE— subtitle (default "engineer, builder, writer")
//	OG_HOMEPAGE_KEY     — R2 object key (default "og/homepage.png")
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/anjanvikas/portfolio-api/internal/config"
	"github.com/anjanvikas/portfolio-api/internal/logger"
	"github.com/anjanvikas/portfolio-api/internal/service"
)

func main() {
	logger.Init()

	if err := config.LoadDotEnv(".env"); err != nil {
		fmt.Fprintln(os.Stderr, "dotenv:", err)
		os.Exit(1)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	if !cfg.R2Configured() {
		fmt.Fprintln(os.Stderr, "R2 is not configured — set R2_ACCESS_KEY / R2_SECRET_KEY / R2_BUCKET_NAME / R2_ENDPOINT in .env")
		os.Exit(1)
	}

	name := getenv("OG_HOMEPAGE_NAME", "Anjan Vikas Reddy")
	headline := getenv("OG_HOMEPAGE_HEADLINE", "engineer, builder, writer")
	key := getenv("OG_HOMEPAGE_KEY", "og/homepage.png")

	gen, err := service.NewGGGenerator()
	if err != nil {
		fmt.Fprintln(os.Stderr, "og generator:", err)
		os.Exit(1)
	}
	png, err := gen.RenderHomepage(name, headline, cfg.SiteURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "render:", err)
		os.Exit(1)
	}

	presigner := service.NewR2Presigner(cfg.R2AccessKey, cfg.R2SecretKey, cfg.R2BucketName, cfg.R2Endpoint, cfg.R2PublicBaseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	publicURL, err := presigner.PutObject(ctx, key, "image/png", png)
	if err != nil {
		fmt.Fprintln(os.Stderr, "upload:", err)
		os.Exit(1)
	}

	slog.Info("homepage OG image uploaded", slog.String("key", key), slog.String("url", publicURL))
	fmt.Println(publicURL)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
