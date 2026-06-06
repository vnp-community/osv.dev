// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command server is the main entry point for the Impact Analysis Service (Enterprise Edition).
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/osv/impact-analysis/internal/application/command/analyze_vulnerability"
	"github.com/osv/impact-analysis/internal/domain/service"
	gogit "github.com/osv/impact-analysis/internal/infra/git"
	"github.com/osv/impact-analysis/internal/infra/messaging/nats"
	"github.com/osv/pkg/health"
	"github.com/osv/pkg/observability"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("impact-analysis: fatal error")
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── OTel ─────────────────────────────────────────────────────────────────
	zlog := zerolog.New(os.Stdout).With().Timestamp().Str("service", "impact-analysis").Logger()
	otelShutdown := observability.MustSetup("impact-analysis", "1.0.0", zlog)
	defer otelShutdown()

	// ── NATS ─────────────────────────────────────────────────────────────────
	natsURL := envOrDefault("NATS_URL", "nats://localhost:4222")
	nc, err := natsgo.Connect(natsURL,
		natsgo.MaxReconnects(-1),
		natsgo.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return fmt.Errorf("nats connect %s: %w", natsURL, err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("nats jetstream: %w", err)
	}

	// ── Git Repo Cache ────────────────────────────────────────────────────────
	cacheDir := envOrDefault("REPO_CACHE_DIR", filepath.Join(os.TempDir(), "osv-repo-cache"))
	repoCache, err := gogit.NewLocalRepoCache(cacheDir)
	if err != nil {
		return fmt.Errorf("repo cache init: %w", err)
	}

	// ── Domain Services ───────────────────────────────────────────────────────
	bisector := service.NewGitBisector(repoCache)
	enumerator := service.NewVersionEnumerator(nil)

	// ── Publisher ─────────────────────────────────────────────────────────────
	publisher := nats.NewEventPublisher(js)

	// ── Command Handler ───────────────────────────────────────────────────────
	handler := analyze_vulnerability.NewHandler(bisector, enumerator, publisher)

	// ── NATS Consumer ─────────────────────────────────────────────────────────
	consumer := nats.NewVulnImportedConsumer(js, handler)

	zlog.Info().Str("nats", natsURL).Msg("impact-analysis: starting")

	go func() {
		if err := consumer.Start(ctx); err != nil && err != context.Canceled {
			zlog.Error().Err(err).Msg("impact-analysis: consumer error")
		}
	}()

	// ── HTTP Health (pkg/health MultiChecker) ─────────────────────────────────
	checker := health.NewMultiChecker(5*time.Second,
		health.NATSProber(nc),
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", health.LiveHandler())
	mux.Handle("/health/ready", health.ReadyHandler(checker))

	httpPort := envOrDefault("HTTP_PORT", "8080")
	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%s", httpPort),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		zlog.Info().Str("port", httpPort).Msg("HTTP health server starting")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zlog.Error().Err(err).Msg("HTTP server error")
		}
	}()

	<-ctx.Done()
	zlog.Info().Msg("impact-analysis: shutting down")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	httpSrv.Shutdown(shutCtx) //nolint:errcheck
	return nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
