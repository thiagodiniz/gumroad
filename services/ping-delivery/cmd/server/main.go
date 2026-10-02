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

	"github.com/antiwork/gumroad/services/ping-delivery/internal/config"
	"github.com/antiwork/gumroad/services/ping-delivery/internal/delivery"
	"github.com/antiwork/gumroad/services/ping-delivery/internal/httpapi"
)

// version is set by the Dockerfile via -ldflags "-X main.version=<git sha>".
var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(2)
	}
	log := newLogger(cfg.LogLevel)
	slog.SetDefault(log)

	deliverer := delivery.New(cfg.MaxRedirects, cfg.OpenTimeout, cfg.ReadTimeout)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.New(deliverer, cfg.InternalToken, log).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      deliverer.Budget() + 5*time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("listening", "addr", cfg.Addr, "env", cfg.Environment, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}
