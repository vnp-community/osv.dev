// notification-service: webhook management and event dispatch.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	pgrepo "github.com/globalcve/notification-service/internal/adapter/repository/postgres"
	"github.com/globalcve/notification-service/internal/adapter/dispatcher"
	deliveryhttp "github.com/globalcve/notification-service/internal/delivery/http"
	"github.com/globalcve/notification-service/internal/usecase/dispatch"
	"github.com/globalcve/notification-service/internal/usecase/manage"
	"github.com/globalcve/notification-service/internal/usecase/register"
	pgpool "github.com/osv/pkg/database/postgres"
)

func main() {
	log := buildLogger(envStr("LOG_LEVEL", "info"))
	log.Info().Msg("Starting notification-service")

	ctx := context.Background()

	// ── PostgreSQL ─────────────────────────────────────────────────────────────
	pool, err := pgpool.NewPool(ctx, &pgpool.Config{
		URL:      mustEnv("DATABASE_URL"),
		MaxConns: 10,
		MinConns: 2,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("PostgreSQL connection failed")
	}
	defer pool.Close()

	// ── Repositories ───────────────────────────────────────────────────────────
	webhookRepo := pgrepo.NewWebhookRepository(pool)

	// ── Dispatcher ─────────────────────────────────────────────────────────────
	disp := dispatcher.New(10*time.Second, log)

	// ── Use cases ──────────────────────────────────────────────────────────────
	regUC := register.New(webhookRepo, log)
	mngUC := manage.New(webhookRepo, log)
	dispUC := dispatch.New(webhookRepo, disp, log)

	// ── HTTP ───────────────────────────────────────────────────────────────────
	jwtSecret := envStr("JWT_SECRET", "dev_jwt_secret_min_32chars_change_in_production")
	h := deliveryhttp.NewHandler(regUC, mngUC, dispUC, log)
	router := deliveryhttp.NewRouter(h, jwtSecret, log)

	srv := &http.Server{
		Addr:         ":" + envStr("PORT", "8084"),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("addr", srv.Addr).Msg("HTTP server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	log.Info().Msg("Shutting down notification-service...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutCtx) //nolint:errcheck
	log.Info().Msg("Stopped")

	_ = dispUC // referenced to avoid unused import
}

func buildLogger(level string) zerolog.Logger {
	lvl, _ := zerolog.ParseLevel(level)
	return zerolog.New(os.Stdout).Level(lvl).With().Timestamp().Str("service", "notification-service").Logger()
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("required env var not set: " + key)
	}
	return v
}
