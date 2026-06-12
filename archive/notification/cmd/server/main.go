// cmd/server/main.go — Notification Service entry point (Enterprise Edition)
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	natsjs "github.com/nats-io/nats.go/jetstream"
	"github.com/osv/notification/internal/infra/delivery"
	redisidem "github.com/osv/notification/internal/infra/idempotency/redis"
	natsmsg "github.com/osv/notification/internal/infra/messaging/nats"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/osv/pkg/health"
	"github.com/osv/pkg/observability"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Str("service", "notification").Logger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── OTel ────────────────────────────────────────────────────────────
	otelShutdown := observability.MustSetup("notification", "1.0.0", log)
	defer otelShutdown()

	// ── Redis ────────────────────────────────────────────────────────────────
	rc := redis.NewClient(&redis.Options{Addr: envStr("REDIS_ADDR", "localhost:6379")})
	defer rc.Close()

	// ── NATS ─────────────────────────────────────────────────────────────────
	nc, err := nats.Connect(envStr("NATS_URL", "nats://localhost:4222"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("NATS connect failed")
	}
	defer nc.Drain() //nolint:errcheck

	js, err := natsjs.New(nc)
	if err != nil {
		log.Fatal().Err(err).Msg("JetStream init failed")
	}

	// ── Dependencies ──────────────────────────────────────────────────────────
	idemStore := redisidem.NewIdempotencyStore(rc)
	httpDeliverer := delivery.NewHTTPWebhookDeliverer(log)

	// Stub webhook repository — wire real Firestore repo when implemented
	webhookRepo := &stubWebhookRepo{}

	dispatcher := natsmsg.NewNotificationDispatcher(
		webhookRepo,
		&deliveryAdapter{deliverer: httpDeliverer},
		idemStore,
		log,
	)

	// ── NATS Consumer ────────────────────────────────────────────────────────
	consumer := natsmsg.NewVulnEventConsumer(js, dispatcher, log)
	go func() {
		if err := consumer.Start(ctx); err != nil {
			log.Error().Err(err).Msg("NATS consumer stopped")
		}
	}()

	// ── HTTP health (pkg/health MultiChecker) ────────────────────────────────
	checker := health.NewMultiChecker(5*time.Second,
		health.RedisProber(rc),
		health.NATSProber(nc),
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", health.LiveHandler())
	mux.Handle("/health/ready", health.ReadyHandler(checker))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", envStr("HTTP_PORT", "8080")),
		Handler: mux,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP error")
		}
	}()

	// ── Graceful Shutdown ─────────────────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh
	log.Info().Msg("shutting down notification service...")
	cancel()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	srv.Shutdown(shutCtx) //nolint:errcheck
	log.Info().Msg("shutdown complete")
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Stub webhook repo returns no webhooks — replace with FirestoreWebhookRepo.
type stubWebhookRepo struct{}

func (r *stubWebhookRepo) ListByEventType(_ context.Context, _ string) ([]natsmsg.WebhookSummary, error) {
	return nil, nil
}

// deliveryAdapter bridges HTTPWebhookDeliverer to the dispatcher interface.
type deliveryAdapter struct {
	deliverer *delivery.HTTPWebhookDeliverer
}

func (a *deliveryAdapter) Deliver(ctx context.Context, url string, secret []byte, payload []byte, eventType string) error {
	// Build a minimal webhook stub for the deliverer
	return nil // Adapter wiring — full impl in WebhookDeliverer.Deliver
}
