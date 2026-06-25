// embed.go — EmbeddedServer allows ai-service to be run inside apps/osv orchestrator.
// This file is ADDITIVE — main.go is NOT modified.
//
// P2-01 FIX: Mount full AI HTTP router (not just /health). Wire provider chain
// with Ollama backend for graceful degradation.
package aiembed

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	httpdelivery "github.com/osv/ai-service/internal/delivery/http"
	"github.com/osv/ai-service/internal/provider"
	"github.com/osv/ai-service/internal/provider/ollama"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// AIServiceEmbeddedConfig holds configuration for embedded ai-service.
type AIServiceEmbeddedConfig struct {
	HTTPPort     int    // REST API port (default: 8086)
	GRPCPort     int    // gRPC port (default: 50052)
	NATSURL      string
	MongoURI     string
	PostgresDSN  string
	AIBaseURL    string // Ollama/OpenAI base URL (default: http://ollama:11434)
	AIModel      string // LLM model name (default: qwen2.5:1.5b)
}

// AIServiceEmbeddedServer wraps ai-service for embedding in apps/osv.
type AIServiceEmbeddedServer struct {
	cfg AIServiceEmbeddedConfig
}

// NewAIServiceEmbeddedServer creates a new embeddable server instance.
func NewAIServiceEmbeddedServer(cfg AIServiceEmbeddedConfig) *AIServiceEmbeddedServer {
	return &AIServiceEmbeddedServer{cfg: cfg}
}

// Name satisfies the orchestrator.Service interface.
func (s *AIServiceEmbeddedServer) Name() string { return "ai-service" }

// Start begins serving and blocks until ctx is cancelled.
func (s *AIServiceEmbeddedServer) Start(ctx context.Context) error {
	if s.cfg.MongoURI != "" {
		os.Setenv("MONGO_URI", s.cfg.MongoURI)
	}
	if s.cfg.PostgresDSN != "" {
		os.Setenv("POSTGRES_DSN", s.cfg.PostgresDSN)
	}
	if s.cfg.NATSURL != "" {
		os.Setenv("NATS_URL", s.cfg.NATSURL)
	}
	return runEmbeddedAIService(ctx, s.cfg)
}

func runEmbeddedAIService(ctx context.Context, cfg AIServiceEmbeddedConfig) error {
	port := cfg.HTTPPort
	if port == 0 {
		port = 8086
	}

	// ── Resolve AI config from struct → env → warn ──────────────────────────
	// [FIX BUG-007] Warn when using localhost fallback to surface misconfiguration in production.
	baseURL := cfg.AIBaseURL
	if baseURL == "" {
		baseURL = os.Getenv("AI_BASE_URL")
	}
	if baseURL == "" {
		baseURL = os.Getenv("OLLAMA_BASE_URL") // legacy name
	}
	if baseURL == "" {
		baseURL = "http://ollama:11434"
		log.Warn().Str("fallback", baseURL).
			Msg("AI_BASE_URL not set, using ollama container default — configure in production")
	}

	modelName := cfg.AIModel
	if modelName == "" {
		modelName = os.Getenv("AI_MODEL")
	}
	if modelName == "" {
		modelName = "qwen2.5:1.5b"
		log.Warn().Str("fallback", modelName).
			Msg("AI_MODEL not set, using default — configure in production")
	}

	// ── Build provider chain with Ollama ────────────────────────────────────
	logger := log.Logger.With().Str("service", "ai-service-embedded").Logger()
	// [FIX BUG-007] Use AI_EMBEDDING_MODEL, fallback OLLAMA_EMBEDDING_MODEL for backward compat
	embeddingModel := firstNonEmptyStr(
		os.Getenv("AI_EMBEDDING_MODEL"),
		os.Getenv("OLLAMA_EMBEDDING_MODEL"), // legacy name — kept for backward compat
	)
	if embeddingModel == "" {
		embeddingModel = "nomic-embed-text"
		log.Warn().Str("fallback", embeddingModel).
			Msg("AI_EMBEDDING_MODEL not set, using default — configure in production")
	}
	ollamaProvider := ollama.New(baseURL, modelName, embeddingModel, 30*time.Second, logger)
	chain := provider.NewChain(logger, ollamaProvider)

	// ── Create AI HTTP handler with nil usecases + chain ────────────────────
	// The handler's isReady() will return false if Ollama is not reachable,
	// causing GetTriageQueue/GetEnrichmentStatus/TriggerEnrichment to return
	// graceful 200 responses instead of panicking on nil use cases.
	httpHandler := httpdelivery.NewAIHTTPHandler(nil, nil, nil, nil, nil, zerolog.Nop()).
		WithChain(chain)

	// ── Mount full AI router ─────────────────────────────────────────────────
	// P2-01: Previously only /health was mounted — this caused all
	// /api/v1/ai/* routes to 404. Now mount the complete router.
	mainMux := http.NewServeMux()
	mainMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ready := chain.HasAvailableProvider()
		if ready {
			fmt.Fprintf(w, `{"status":"ok","service":"ai-service","provider":"ollama"}`)
		} else {
			fmt.Fprintf(w, `{"status":"degraded","service":"ai-service","provider":"none","message":"Ollama not reachable"}`)
		}
	})
	// Mount the full AI router under /api/v1/ai/*
	aiRouter := httpdelivery.NewRouter(httpHandler)
	mainMux.Handle("/", aiRouter)
	mainMux.Handle("/api/v2/ai/", aiRouter) // v2 alias

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("ai-service listen :%d: %w", port, err)
	}
	srv := &http.Server{Handler: mainMux}
	go srv.Serve(ln) //nolint:errcheck

	logger.Info().Int("port", port).Str("ollama_url", baseURL).Msg("ai-service embedded ready (graceful degradation mode)")

	<-ctx.Done()
	return srv.Close()
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
