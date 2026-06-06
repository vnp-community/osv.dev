// cmd/server/main.go — AI Enrichment Service (Enterprise Edition)
// AI Provider: VertexAI (Gemini 1.5 Flash) → OpenAI (GPT-4o-mini) → Ollama (local)
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/firestore"
	natsgo "github.com/nats-io/nats.go"
	natsjs "github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"

	"github.com/osv/ai-enrichment/internal/application/command/enrich_vulnerability"
	"github.com/osv/ai-enrichment/internal/application/port"
	"github.com/osv/ai-enrichment/internal/domain/service"
	"github.com/osv/ai-enrichment/internal/infra/ai/ollama"
	"github.com/osv/ai-enrichment/internal/infra/ai/openai"
	"github.com/osv/ai-enrichment/internal/infra/ai/vertex"
	fsrepo "github.com/osv/ai-enrichment/internal/infra/persistence/firestore"
	natsmsg "github.com/osv/ai-enrichment/internal/infra/messaging/nats"
	"github.com/osv/pkg/health"
	"github.com/osv/pkg/observability"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Str("service", "ai-enrichment").Logger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── OTel setup ────────────────────────────────────────────────────────────
	otelShutdown := observability.MustSetup("ai-enrichment", "1.0.0", log)
	defer otelShutdown()

	// ── Config from environment ───────────────────────────────────────────────
	projectID := envStr("GCP_PROJECT_ID", "osv-dev")
	region := envStr("VERTEX_REGION", "us-central1")
	fallbackRegion := envStr("VERTEX_FALLBACK_REGION", "us-east4")
	openAIKey := envStr("OPENAI_API_KEY", "")
	ollamaURL := envStr("OLLAMA_URL", "http://localhost:11434")
	ollamaEmbedModel := envStr("OLLAMA_MODEL", "nomic-embed-text")
	ollamaLLMModel := envStr("OLLAMA_LLM_MODEL", "llama3.2")
	primaryProvider := envStr("AI_PROVIDER_PRIMARY", "vertex")
	natsURL := envStr("NATS_URL", "nats://localhost:4222")
	grpcPort := envStr("GRPC_PORT", "50051")
	httpPort := envStr("HTTP_PORT", "8080")

	// ── Firestore ────────────────────────────────────────────────────────────
	fsClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatal().Err(err).Msg("firestore init failed")
	}
	defer fsClient.Close()

	// ── NATS ─────────────────────────────────────────────────────────────────
	nc, err := natsgo.Connect(natsURL, natsgo.MaxReconnects(-1), natsgo.ReconnectWait(2*time.Second))
	if err != nil {
		log.Fatal().Err(err).Msg("NATS connect failed")
	}
	defer nc.Drain() //nolint:errcheck

	js, err := natsjs.New(nc)
	if err != nil {
		log.Fatal().Err(err).Msg("JetStream init failed")
	}

	// ── AI Providers (3-tier chain) ───────────────────────────────────────────
	tracer := otel.Tracer("ai-enrichment")

	var llmProviders []port.LLMProvider
	var embedProviders []port.EmbeddingProvider

	// VertexAI
	vertexCfg := vertex.Config{ProjectID: projectID, Region: region, FallbackRegion: fallbackRegion}
	if vEmbed, err := vertex.NewVertexEmbeddingAdapter(vertexCfg, log); err != nil {
		log.Warn().Err(err).Msg("VertexAI embedding unavailable (no ADC?), skipping")
	} else {
		embedProviders = append(embedProviders, vEmbed)
	}
	if vLLM, err := vertex.NewVertexGeminiAdapter(vertexCfg, "", log); err != nil {
		log.Warn().Err(err).Msg("VertexAI Gemini unavailable, skipping")
	} else {
		llmProviders = append(llmProviders, vLLM)
	}

	// OpenAI
	if openAIKey != "" {
		oa := openai.NewClient(openAIKey, "", "", log)
		llmProviders = append(llmProviders, oa)
		embedProviders = append(embedProviders, oa)
	} else {
		log.Warn().Msg("OPENAI_API_KEY not set, OpenAI provider skipped")
	}

	// Ollama (always available as final fallback)
	ollamaAdapter := ollama.NewOllamaAdapter(ollamaURL, ollamaEmbedModel, ollamaLLMModel, log)
	llmProviders = append(llmProviders, ollamaAdapter)
	embedProviders = append(embedProviders, ollamaAdapter)

	log.Info().
		Int("llm_providers", len(llmProviders)).
		Int("embed_providers", len(embedProviders)).
		Str("primary", primaryProvider).
		Msg("AI provider chain initialized")

	// ── Domain Services ───────────────────────────────────────────────────────
	tracker := service.NewInMemoryUsageTracker()
	enrichmentRepo := fsrepo.NewEnrichmentRepo(fsClient, log)
	publisher := natsmsg.NewAIEnrichmentPublisher(js, log)

	// Use first available providers (chain fallback handled at provider level)
	var llmProvider port.LLMProvider
	if len(llmProviders) > 0 {
		llmProvider = llmProviders[0]
	}
	var embedProvider port.EmbeddingProvider
	if len(embedProviders) > 0 {
		embedProvider = embedProviders[0]
	}

	embeddingSvc := service.NewEmbeddingService(embedProvider, nil, log)
	classifier := service.NewSeverityClassifier(llmProvider, log)

	enrichHandler := enrich_vulnerability.NewHandler(
		embeddingSvc, classifier, llmProvider, enrichmentRepo, publisher, tracer, log,
	)

	// ── NATS Consumer ─────────────────────────────────────────────────────────
	consumer := natsmsg.NewVulnImportedConsumer(js, enrichHandler, log)
	go func() {
		if err := consumer.Start(ctx); err != nil && ctx.Err() == nil {
			log.Error().Err(err).Msg("NATS consumer error")
		}
	}()

	// ── gRPC Server ───────────────────────────────────────────────────────────
	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			recoveryInterceptor(log),
			loggingInterceptor(log),
		),
	)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Fatal().Err(err).Msg("gRPC listen failed")
	}
	go func() {
		log.Info().Str("port", grpcPort).Msg("gRPC starting")
		if err := grpcSrv.Serve(lis); err != nil {
			log.Error().Err(err).Msg("gRPC error")
		}
	}()

	// ── Health Server ─────────────────────────────────────────────────────────
	checker := health.NewMultiChecker(5*time.Second,
		health.NATSProber(nc),
		health.NewFirestoreProber(func(ctx context.Context) error {
			_, err := fsClient.Collection("health").Doc("probe").Get(ctx)
			if err != nil && err.Error() != "rpc error: code = NotFound" {
				return err
			}
			return nil
		}),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", health.LiveHandler())
	mux.Handle("/health/ready", health.ReadyHandler(checker))
	mux.HandleFunc("/metrics/ai", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"total_tokens_today":%d}`, tracker.TotalTokensToday())
	})

	httpSrv := &http.Server{Addr: fmt.Sprintf(":%s", httpPort), Handler: mux}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP error")
		}
	}()

	log.Info().
		Str("grpc_port", grpcPort).
		Str("http_port", httpPort).
		Msg("AI Enrichment Service started (enterprise edition)")

	// ── Graceful Shutdown ─────────────────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh
	log.Info().Msg("shutting down ai-enrichment...")
	cancel()
	grpcSrv.GracefulStop()
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

func recoveryInterceptor(log zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Str("method", info.FullMethod).Msg("panic recovered")
				err = fmt.Errorf("internal error")
			}
		}()
		return handler(ctx, req)
	}
}

func loggingInterceptor(log zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		log.Info().Str("method", info.FullMethod).Dur("ms", time.Since(start)).Err(err).Send()
		return resp, err
	}
}
