// Package service implements the AI provider failover chain.
// Chain: VertexAI (primary) → OpenAI (secondary) → Ollama (local fallback)
// Each provider has its own circuit breaker. Failed providers are bypassed
// until their circuit half-opens.
package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/osv/ai-service/internal/domain/enrichment/port"
	"github.com/osv/shared/pkg/resilience"
	"github.com/rs/zerolog"
)

// ProviderKind identifies an AI provider for metrics.
type ProviderKind string

const (
	ProviderVertex ProviderKind = "vertex"
	ProviderOpenAI ProviderKind = "openai"
	ProviderOllama ProviderKind = "ollama"
)

// AIUsageRecord tracks token usage per provider per call.
type AIUsageRecord struct {
	Provider  ProviderKind
	Operation string
	VulnID    string
	Tokens    int64
	Latency   time.Duration
	Success   bool
}

// UsageTracker records AI usage (for monitoring, not blocking).
type UsageTracker interface {
	Record(rec AIUsageRecord)
	TotalTokensToday() int64
}

// ProviderChain implements LLMProvider and EmbeddingProvider with automatic failover.
type ProviderChain struct {
	providers []chainEntry
	tracker   UsageTracker
	log       zerolog.Logger
}

type chainEntry struct {
	kind    ProviderKind
	llm     port.LLMProvider
	embed   port.EmbeddingProvider
	breaker *resilience.CircuitBreaker
}

// NewProviderChain creates a 3-tier AI provider chain.
// Providers are tried in order; if one fails its circuit opens and the next is tried.
func NewProviderChain(
	vertex port.LLMProvider,       // primary  (nil = skip)
	openai port.LLMProvider,       // secondary (nil = skip)
	ollama port.LLMProvider,       // fallback  (nil = skip)
	vertexEmbed port.EmbeddingProvider,
	openaiEmbed port.EmbeddingProvider,
	ollamaEmbed port.EmbeddingProvider,
	tracker UsageTracker,
	log zerolog.Logger,
) *ProviderChain {
	cbConfig := resilience.CircuitBreakerConfig{
		MaxFailures:         3,
		Timeout:             2 * time.Minute,
		HalfOpenMaxRequests: 1,
	}

	entries := []chainEntry{}
	if vertex != nil {
		entries = append(entries, chainEntry{
			kind:    ProviderVertex,
			llm:     vertex,
			embed:   vertexEmbed,
			breaker: resilience.NewCircuitBreaker("vertex", cbConfig),
		})
	}
	if openai != nil {
		entries = append(entries, chainEntry{
			kind:    ProviderOpenAI,
			llm:     openai,
			embed:   openaiEmbed,
			breaker: resilience.NewCircuitBreaker("openai", cbConfig),
		})
	}
	if ollama != nil {
		entries = append(entries, chainEntry{
			kind:    ProviderOllama,
			llm:     ollama,
			embed:   ollamaEmbed,
			breaker: resilience.NewCircuitBreaker("ollama", cbConfig),
		})
	}

	return &ProviderChain{
		providers: entries,
		tracker:   tracker,
		log:       log,
	}
}

// Complete tries each LLM provider in chain order until one succeeds.
func (c *ProviderChain) Complete(ctx context.Context, prompt string, result interface{}) error {
	return c.tryEach(ctx, func(entry chainEntry) error {
		return entry.llm.GenerateStructured(ctx, prompt, result)
	}, "llm")
}

// GenerateEmbedding tries each embedding provider in chain order.
func (c *ProviderChain) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	var embedding []float32
	err := c.tryEach(ctx, func(entry chainEntry) error {
		if entry.embed == nil {
			return fmt.Errorf("no embedding provider for %s", entry.kind)
		}
		var e error
		embedding, e = entry.embed.GenerateEmbedding(ctx, text)
		return e
	}, "embed")
	return embedding, err
}

func (c *ProviderChain) tryEach(ctx context.Context, fn func(chainEntry) error, operation string) error {
	if len(c.providers) == 0 {
		return fmt.Errorf("no AI providers configured")
	}

	var errs []error
	for _, entry := range c.providers {
		start := time.Now()
		err := entry.breaker.Execute(ctx, func(ctx context.Context) error {
			return fn(entry)
		})

		c.tracker.Record(AIUsageRecord{
			Provider:  entry.kind,
			Operation: operation,
			Latency:   time.Since(start),
			Success:   err == nil,
		})

		if err == nil {
			if len(errs) > 0 {
				c.log.Info().
					Str("provider", string(entry.kind)).
					Str("operation", operation).
					Msg("succeeded on fallback provider")
			}
			return nil
		}

		c.log.Warn().
			Err(err).
			Str("provider", string(entry.kind)).
			Str("circuit_state", entry.breaker.State().String()).
			Str("operation", operation).
			Msg("AI provider failed, trying next")

		errs = append(errs, fmt.Errorf("%s: %w", entry.kind, err))
	}

	return fmt.Errorf("all AI providers failed: %v", errs)
}

// ── InMemoryUsageTracker ──────────────────────────────────────────────────────

// InMemoryUsageTracker tracks token usage in-memory (resets daily).
type InMemoryUsageTracker struct {
	mu      sync.Mutex
	total   int64
	records []AIUsageRecord
	dayKey  string
}

// NewInMemoryUsageTracker creates a simple in-memory tracker.
func NewInMemoryUsageTracker() *InMemoryUsageTracker {
	return &InMemoryUsageTracker{dayKey: dayKey()}
}

func (t *InMemoryUsageTracker) Record(rec AIUsageRecord) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Reset daily
	if k := dayKey(); k != t.dayKey {
		t.total = 0
		t.records = nil
		t.dayKey = k
	}

	t.total += rec.Tokens
	t.records = append(t.records, rec)
}

func (t *InMemoryUsageTracker) TotalTokensToday() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.total
}

func dayKey() string {
	return time.Now().UTC().Format("2006-01-02")
}
