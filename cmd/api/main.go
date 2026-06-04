package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anjanvikas2001/portfolio-api/internal/config"
	"github.com/anjanvikas2001/portfolio-api/internal/handler"
	"github.com/anjanvikas2001/portfolio-api/internal/logger"
	"github.com/anjanvikas2001/portfolio-api/internal/service"
	"github.com/anjanvikas2001/portfolio-api/internal/store"
)

func main() {
	logger.Init()

	if err := config.LoadDotEnv(".env"); err != nil {
		slog.Error("dotenv load failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	pool, err := store.Connect(rootCtx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database connect failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("database connected", slog.String("host", pool.Config().ConnConfig.Host))

	// Contact email: Resend when credentials are present, otherwise a dev
	// fallback that logs the message instead of sending it.
	var mailer service.Mailer
	if cfg.MailerConfigured() {
		mailer = service.NewResendMailer(cfg.ResendAPIKey, cfg.ContactFromEmail, cfg.ContactToEmail)
		slog.Info("contact email via Resend", slog.String("to", cfg.ContactToEmail))
	} else {
		mailer = service.LogMailer{}
		slog.Warn("Resend not configured (RESEND_API_KEY/CONTACT_TO_EMAIL) — contact form will log messages, not email them")
	}

	// R2 presigner: powers both the resume download (GET presign) and the asset
	// upload pipeline (PUT presign + public URLs). Nil when R2 isn't configured,
	// in which case the resume falls back to the stored URL and the admin asset
	// endpoints return 503.
	var presigner *service.R2Presigner
	if cfg.R2Configured() {
		presigner = service.NewR2Presigner(cfg.R2AccessKey, cfg.R2SecretKey, cfg.R2BucketName, cfg.R2Endpoint, cfg.R2PublicBaseURL)
		slog.Info("R2 presigning enabled", slog.String("resume_key", cfg.R2ResumeKey), slog.Bool("public_base_set", cfg.R2PublicBaseURL != ""))
	} else {
		slog.Warn("R2 not configured — resume download falls back to stored resume_url; asset uploads disabled")
	}

	// Docx→markdown conversion shells out to pandoc. The handler reports 503 at
	// call time if the binary is missing, so wiring it unconditionally is safe.
	converter := service.NewPandocConverter()

	// OG image generator (SCRUM-69) renders 1200x630 cards from embedded fonts.
	// Failing to parse the embedded TTFs is a build-time issue, not a runtime
	// one, so we treat a startup error as fatal.
	ogGen, err := service.NewGGGenerator()
	if err != nil {
		slog.Error("og generator init failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	router := handler.NewRouter(handler.Deps{
		Pool:               pool,
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
		JWTSecret:          cfg.JWTSecret,
		AdminPasswordHash:  cfg.AdminPasswordHash,
		CookieSecure:       cfg.CookieSecure,
		Mailer:             mailer,
		Presigner:          presigner,
		ResumeKey:          cfg.R2ResumeKey,
		DocxConverter:      converter,
		OGGenerator:        ogGen,
		SiteURL:            cfg.SiteURL,
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server starting", slog.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		slog.Error("server failed", slog.String("error", err.Error()))
		os.Exit(1)
	case sig := <-quit:
		slog.Info("shutdown signal received", slog.String("signal", sig.String()))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.Info("server stopped")
}
