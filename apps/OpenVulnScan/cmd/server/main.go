// Command server — main entry point cho OpenVulnScan monolith.
// Khởi động tất cả service goroutines, đợi signal, graceful shutdown.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/osv/apps/openvulnscan/internal/app"
)

func main() {
	// ── Logging ───────────────────────────────────────────────────────────
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	})
	log.Info().Msg("OpenVulnScan monolith starting...")

	// ── Config ────────────────────────────────────────────────────────────
	cfgPath := "configs/config.yaml"
	if v := os.Getenv("CONFIG_PATH"); v != "" {
		cfgPath = v
	}

	cfg, err := app.LoadConfig(cfgPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", cfgPath).Msg("failed to load config")
	}

	// ── Application ───────────────────────────────────────────────────────
	application, err := app.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create application")
	}

	// Root context với cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Start all service goroutines ──────────────────────────────────────
	if err := application.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start application")
	}

	log.Info().
		Str("http", cfg.Server.HTTPAddr).
		Msg("OpenVulnScan ready — all service goroutines running")

	// ── Wait for shutdown signal ──────────────────────────────────────────
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	log.Info().Msg("shutdown signal received — initiating graceful shutdown...")

	// Cancel context → tất cả goroutines nhận signal stop
	cancel()

	// Wait với timeout
	done := make(chan struct{})
	go func() {
		application.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Msg("all goroutines stopped — shutdown complete")
	case <-time.After(30 * time.Second):
		log.Warn().Msg("shutdown timeout (30s) exceeded — forcing exit")
	}

	application.Shutdown()
	log.Info().Msg("OpenVulnScan stopped")
}
