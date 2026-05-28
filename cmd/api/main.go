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

	router := handler.NewRouter(handler.Deps{
		Pool:               pool,
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
		JWTSecret:          cfg.JWTSecret,
		AdminPasswordHash:  cfg.AdminPasswordHash,
		CookieSecure:       cfg.CookieSecure,
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
