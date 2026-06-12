// cmd/server/main.go — Source Sync Service entry point (Enterprise Edition)
// Polls GitHub/OSV sources for new advisories and publishes SourceChangeDetected events.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	natsgo "github.com/nats-io/nats.go"
	natsjs "github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"

	"github.com/osv/pkg/health"
	"github.com/osv/pkg/observability"
	"github.com/osv/source-sync/internal/application/command/sync_source"
	"github.com/osv/source-sync/internal/infra/source"
	"github.com/osv/source-sync/internal/infra/webhook"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Str("service", "source-sync").Logger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── OTel ─────────────────────────────────────────────────────────────────
	otelShutdown := observability.MustSetup("source-sync", "1.0.0", log)
	defer otelShutdown()

	// ── Config ────────────────────────────────────────────────────────────────
	natsURL := envStr("NATS_URL", "nats://localhost:4222")
	httpPort := envStr("HTTP_PORT", "8080")
	syncInterval := envDuration("SYNC_INTERVAL", 15*time.Minute)
	gcsBucket := envStr("GCS_BUCKET", "osv-vulnerabilities")
	githubToken := envStr("GITHUB_TOKEN", "")
	githubWebhookSecret := envStr("GITHUB_WEBHOOK_SECRET", "")
	gitlabWebhookToken := envStr("GITLAB_WEBHOOK_TOKEN", "")

	// ── NATS ─────────────────────────────────────────────────────────────────
	nc, err := natsgo.Connect(natsURL,
		natsgo.MaxReconnects(-1),
		natsgo.ReconnectWait(2*time.Second),
		natsgo.DisconnectErrHandler(func(_ *natsgo.Conn, err error) {
			log.Warn().Err(err).Msg("NATS disconnected")
		}),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("NATS connect failed")
	}
	defer nc.Drain() //nolint:errcheck

	js, err := natsjs.New(nc)
	if err != nil {
		log.Fatal().Err(err).Msg("JetStream init failed")
	}

	// ── Source Adapters ───────────────────────────────────────────────────────
	osvSource := source.NewOSVGCSSource(gcsBucket, log)
	githubSource := source.NewGitHubAdvisorySource(githubToken, log)
	sources := []sync_source.Source{osvSource, githubSource}

	// ── Sync Handler ──────────────────────────────────────────────────────────
	publisher := sync_source.NewNATSPublisher(js, log)
	syncHandler := sync_source.NewHandler(sources, publisher, log)

	// ── Sync Loop ─────────────────────────────────────────────────────────────
	go func() {
		ticker := time.NewTicker(syncInterval)
		defer ticker.Stop()

		// Run immediately on startup
		log.Info().Msg("initial sync starting")
		if err := syncHandler.SyncAll(ctx); err != nil {
			log.Error().Err(err).Msg("initial sync failed")
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				log.Info().Msg("periodic sync starting")
				if err := syncHandler.SyncAll(ctx); err != nil {
					log.Error().Err(err).Msg("periodic sync failed")
				}
			}
		}
	}()

	// ── HTTP Health (pkg/health MultiChecker) ─────────────────────────────────
	checker := health.NewMultiChecker(5*time.Second,
		health.NATSProber(nc),
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", health.LiveHandler())
	mux.Handle("/health/ready", health.ReadyHandler(checker))
	mux.HandleFunc("/metrics/sync", func(w http.ResponseWriter, r *http.Request) {
		stats := syncHandler.Stats()
		fmt.Fprintf(w, `{"last_sync_at":%q,"total_synced":%d,"errors":%d}`,
			stats.LastSyncAt.Format(time.RFC3339),
			stats.TotalSynced,
			stats.Errors,
		)
	})

	// ── Webhook Handler ────────────────────────────────────────────────────────
	// ConfigSourceResolver maps repo URLs → source names (empty for now; extend from sources.yaml)
	resolver := webhook.NewConfigSourceResolver(nil)
	trigger := webhook.NewNATSSyncTrigger(nc, log)
	webhookHandler := webhook.NewHandler(trigger, resolver, webhook.Config{
		GitHubSecret: githubWebhookSecret,
		GitLabToken:  gitlabWebhookToken,
	}, log)
	webhookHandler.RegisterRoutes(mux)
	log.Info().Msg("webhook routes registered: /webhooks/github, /webhooks/gitlab")

	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%s", httpPort),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		log.Info().Str("port", httpPort).Msg("HTTP health server starting")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP error")
		}
	}()

	log.Info().
		Str("nats", natsURL).
		Dur("sync_interval", syncInterval).
		Msg("source-sync service started")

	// ── Graceful Shutdown ─────────────────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh
	log.Info().Msg("shutting down source-sync...")
	cancel()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	httpSrv.Shutdown(shutCtx) //nolint:errcheck
	log.Info().Msg("shutdown complete")
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
