// Package main is the entrypoint for ai-service.
// Serves CVE enrichment (embeddings, MITRE tagging, severity) and
// DefectDojo finding triage via a unified LLM backend.
package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	aifactory "github.com/osv/ai-service/internal/infra/ai"
)

func main() {
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg := aifactory.FromEnv()
	if err := cfg.Validate(); err != nil {
		log.Fatal().Err(err).Msg("invalid AI backend config")
	}

	log.Info().
		Str("backend", string(cfg.Backend)).
		Str("model", cfg.ModelName).
		Msg("ai-service starting")

	// gRPC server would be wired here
	grpcPort := envOrDefault("GRPC_PORT", "50052")
	log.Info().Str("port", grpcPort).Msg("ai-service gRPC listening")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Info().Msg("ai-service shutting down")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
