// gen-post-og re-renders the OG card for every published blog post with the
// current renderer, uploads it to R2 at og/posts/<slug>.png and stores the
// public URL (with a ?v=<unix> cache-buster) on blog_post.og_image_url.
//
// Usage:
//
//	cd backend && go run ./cmd/gen-post-og -dry-run -out ./tmp/og   # local PNGs only
//	go run ./cmd/gen-post-og                                         # all posts
//	go run ./cmd/gen-post-og -slug my-post                           # one post
//	# or: make og-posts
//
// Inputs (env / .env): the full API config (config.Load requires DATABASE_URL,
// JWT_SECRET, ADMIN_PASSWORD, R2_*), plus SITE_URL. -dry-run still connects to
// the DB to read posts but never uploads or writes.
//
// Write runs (no -dry-run) print DB host, R2 bucket and SITE_URL, refuse a
// localhost SITE_URL unless -allow-localhost is given, and require -yes.
// Posts are updated without touching updated_at.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/anjanvikas/portfolio-api/internal/config"
	"github.com/anjanvikas/portfolio-api/internal/logger"
	"github.com/anjanvikas/portfolio-api/internal/service"
	"github.com/anjanvikas/portfolio-api/internal/store"
)

func main() {
	logger.Init()
	dry := flag.Bool("dry-run", false, "render to -out only; no upload, no DB write")
	outDir := flag.String("out", "./tmp/og", "output directory for -dry-run")
	slug := flag.String("slug", "", "regenerate only this post")
	yes := flag.Bool("yes", false, "confirm a write run (upload to R2 + update the DB)")
	allowLocal := flag.Bool("allow-localhost", false, "allow a localhost SITE_URL on a write run")
	flag.Parse()

	if err := config.LoadDotEnv(".env"); err != nil {
		fmt.Fprintln(os.Stderr, "dotenv:", err)
		os.Exit(1)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	if !*dry && !cfg.R2Configured() {
		fmt.Fprintln(os.Stderr, "R2 is not configured — set R2_ACCESS_KEY / R2_SECRET_KEY / R2_BUCKET_NAME / R2_ENDPOINT")
		os.Exit(1)
	}

	if !*dry {
		dbHost := "?"
		if u, err := url.Parse(cfg.DatabaseURL); err == nil && u.Host != "" {
			dbHost = u.Host
		}
		fmt.Printf("WRITE RUN\n  db:       %s\n  r2 bucket: %s\n  site url: %s\n", dbHost, cfg.R2BucketName, cfg.SiteURL)
		if err := service.CheckSiteURLForUpload(cfg.SiteURL, *allowLocal); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if !*yes {
			fmt.Fprintln(os.Stderr, "refusing to write without -yes (use -dry-run to preview)")
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "db:", err)
		os.Exit(1)
	}
	defer pool.Close()

	gen, err := service.NewGGGenerator()
	if err != nil {
		fmt.Fprintln(os.Stderr, "og generator:", err)
		os.Exit(1)
	}
	var up ogUploader
	if !*dry {
		up = service.NewR2Presigner(cfg.R2AccessKey, cfg.R2SecretKey, cfg.R2BucketName, cfg.R2Endpoint, cfg.R2PublicBaseURL)
	}

	_, failed, err := regenerate(ctx, store.New(pool), gen, up, regenOptions{
		DryRun: *dry, OutDir: *outDir, Slug: *slug, SiteURL: cfg.SiteURL, RunUnix: time.Now().Unix(),
	}, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if failed > 0 {
		os.Exit(1)
	}
}
