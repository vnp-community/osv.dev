// Package main is the entrypoint for ai-service.
// Serves CVE enrichment (embeddings, MITRE tagging, severity) and
// DefectDojo finding triage via a unified LLM backend.
package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	httpDelivery "github.com/osv/ai-service/internal/delivery/http"
	aifactory "github.com/osv/ai-service/internal/infra/ai"
	"github.com/osv/ai-service/internal/provider"
	"github.com/osv/ai-service/internal/provider/ollama"
	"github.com/osv/ai-service/internal/provider/openai"
)

func main() {
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg := aifactory.FromEnv()
	// FIX TASK-007: Log warning instead of Fatal so ai-service starts even when
	// Ollama/OpenAI is not yet reachable. Handlers use P2-01 graceful degradation.
	if err := cfg.Validate(); err != nil {
		log.Warn().Err(err).Msg("AI backend config invalid — starting in degraded mode (triage/enrichment will return safe defaults)")
	}

	log.Info().
		Str("backend", string(cfg.Backend)).
		Str("model", cfg.ModelName).
		Msg("ai-service starting")

	// gRPC server with health check
	// AI_GRPC_PORT takes priority over GRPC_PORT
	grpcPort := envOrDefault("AI_GRPC_PORT", envOrDefault("GRPC_PORT", "50052"))

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatal().Err(err).Str("port", grpcPort).Msg("gRPC listen failed")
	}

	s := grpc.NewServer()
	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(s, healthSvc)
	healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthSvc.SetServingStatus("ai.AIService", healthpb.HealthCheckResponse_SERVING)
	// TODO: Register AIServiceServer after proto stubs are generated

	go func() {
		log.Info().Str("port", grpcPort).Msg("ai-service gRPC started")
		if err := s.Serve(lis); err != nil {
			log.Error().Err(err).Msg("gRPC serve error")
		}
	}()

	// ── Redis ─────────────────────────────────────────────────────────────
	// TASK-006 FIX: Wire redis so triage queue and enrichment status are persisted.
	var redisClient *redis.Client
	redisURL := envOrDefault("REDIS_URL", "redis://localhost:6379/0")
	if rdbOpts, err := redis.ParseURL(redisURL); err == nil {
		redisClient = redis.NewClient(rdbOpts)
		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			log.Warn().Err(err).Str("url", redisURL).Msg("ai-service: Redis unreachable, triage queue will be in-memory only")
			redisClient = nil
		} else {
			log.Info().Str("url", redisURL).Msg("ai-service: Redis connected")
		}
	} else {
		log.Warn().Err(err).Str("url", redisURL).Msg("ai-service: invalid REDIS_URL, running without Redis")
	}

	// ── Provider Chain ────────────────────────────────────────────────────
	// TASK-006 FIX: Build provider chain so isReady() works correctly for P2-01.
	var providers []provider.LLMProvider
	switch cfg.Backend {
	case aifactory.BackendOllama:
		providers = append(providers, ollama.New(
			cfg.BaseURL, cfg.ModelName, cfg.ModelName, 30*time.Second, log.Logger,
		))
	case aifactory.BackendOpenAI:
		if cfg.APIKey != "" {
			providers = append(providers, openai.New(
				cfg.APIKey, cfg.BaseURL, cfg.ModelName, cfg.ModelName, 30*time.Second, log.Logger,
			))
		}
	}
	providerChain := provider.NewChain(log.Logger, providers...)

	// ── HTTP Server ───────────────────────────────────────────────────────
	// TASK-006 FIX: Pass redisClient and providerChain instead of nil
	httpHandler := httpDelivery.NewAIHTTPHandler(nil, nil, nil, nil, redisClient, log.Logger).
		WithChain(providerChain)
	httpRouter := httpDelivery.NewRouter(httpHandler)
	httpPort := envOrDefault("AI_HTTP_PORT", envOrDefault("HTTP_PORT", "9103"))
	httpSrv := &http.Server{
		Addr:    ":" + httpPort,
		Handler: httpRouter,
	}

	go func() {
		log.Info().Str("port", httpPort).Msg("ai-service HTTP started")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP serve error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Info().Msg("ai-service shutting down")
	s.GracefulStop()
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
