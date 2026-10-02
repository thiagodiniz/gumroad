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

	"github.com/antiwork/gumroad/services/tax-id-validation/internal/cache"
	"github.com/antiwork/gumroad/services/tax-id-validation/internal/config"
	"github.com/antiwork/gumroad/services/tax-id-validation/internal/httpapi"
	"github.com/antiwork/gumroad/services/tax-id-validation/internal/validator"
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

	store := cache.New(cfg.CacheTTL)
	upstream := &http.Client{Timeout: cfg.UpstreamTimeout}
	vies := &http.Client{Timeout: cfg.ViesTimeout}

	router := &validator.Router{
		ABN:      validator.NewCached(validator.NewABN(upstream, cfg.VatstackURL, cfg.VatstackAPIKey), store),
		MVA:      validator.NewCached(validator.NewMVA(upstream, cfg.VatstackURL, cfg.VatstackAPIKey), store),
		GST:      validator.NewCached(validator.NewGST(upstream, cfg.IrasURL, cfg.IrasClientID, cfg.IrasSecret), store),
		QST:      validator.NewCached(validator.NewQST(upstream, cfg.RevenuQuebecURL), store),
		TaxIDPro: validator.NewCachedCountry(validator.NewTaxIDPro(upstream, cfg.TaxIDProURL, cfg.TaxIDProAPIKey), store),
		TRN:      validator.Trn{},
		KraPin:   validator.KraPin,
		FirsTin:  validator.FirsTin,
		TraTin:   validator.TraTin,
		OmanVat:  validator.OmanVat,
		EUVat:    validator.NewEUVat(vies, cfg.ViesURL, cfg.VatRegistrationNumber, log),
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.New(router, cfg.InternalToken, log).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		// VIES can legitimately take up to ViesTimeout; leave headroom for the response.
		WriteTimeout: cfg.ViesTimeout + 5*time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		t := time.NewTicker(cfg.CacheTTL)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				store.Sweep()
			}
		}
	}()

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
