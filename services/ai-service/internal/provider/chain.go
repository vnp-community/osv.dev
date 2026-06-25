package provider

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/rs/zerolog"
)

// Metrics
var (
    providerErrors = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "ai_provider_errors_total",
        Help: "Total errors per AI provider",
    }, []string{"provider", "operation"}) // operation: embedding|generate

    providerFallbacks = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "ai_provider_fallbacks_total",
        Help: "Total fallbacks to secondary providers",
    }, []string{"from_provider", "to_provider"})
)

// Chain tries providers in order, falling back on failure
type Chain struct {
    providers []LLMProvider
    logger    zerolog.Logger
}

// NewChain creates a provider chain with the given providers in priority order
// Example: NewChain(ollamaProvider, openAIProvider)
func NewChain(logger zerolog.Logger, providers ...LLMProvider) *Chain {
    return &Chain{providers: providers, logger: logger}
}

// GenerateEmbedding tries each provider in order until one succeeds
func (c *Chain) GenerateEmbedding(ctx context.Context, text string) ([]float32, string, error) {
    var lastErr error

    for i, p := range c.providers {
        embedding, err := p.GenerateEmbedding(ctx, text)
        if err == nil {
            if i > 0 {
                // Log successful fallback
                c.logger.Info().
                    Str("provider", p.Name()).
                    Msg("embedding fallback succeeded")
            }
            return embedding, p.Name(), nil
        }

        providerErrors.WithLabelValues(p.Name(), "embedding").Inc()
        c.logger.Warn().
            Err(err).
            Str("provider", p.Name()).
            Msg("embedding provider failed, trying next")

        // Track fallback metrics
        if i+1 < len(c.providers) {
            providerFallbacks.WithLabelValues(p.Name(), c.providers[i+1].Name()).Inc()
        }

        lastErr = fmt.Errorf("provider %s: %w", p.Name(), err)
    }

    return nil, "", fmt.Errorf("all providers failed for embedding: %w", lastErr)
}

// Generate tries each provider in order until one succeeds
func (c *Chain) Generate(ctx context.Context, prompt string) (string, string, error) {
    var errs []string

    for i, p := range c.providers {
        result, err := p.Generate(ctx, prompt)
        if err == nil {
            if i > 0 {
                c.logger.Info().
                    Str("provider", p.Name()).
                    Msg("generate fallback succeeded")
            }
            return result, p.Name(), nil
        }

        providerErrors.WithLabelValues(p.Name(), "generate").Inc()
        errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
        c.logger.Warn().
            Err(err).
            Str("provider", p.Name()).
            Msg("generate provider failed, trying next")

        if i+1 < len(c.providers) {
            providerFallbacks.WithLabelValues(p.Name(), c.providers[i+1].Name()).Inc()
        }

    }

    return "", "", fmt.Errorf("all providers failed: %s", strings.Join(errs, "; "))
}

// Dimensions returns the dimensions of the first available provider
func (c *Chain) Dimensions() int {
    if len(c.providers) == 0 {
        return 0
    }
    return c.providers[0].Dimensions()
}

// PrimaryProvider returns the name of the first provider in the chain
func (c *Chain) PrimaryProvider() string {
    if len(c.providers) == 0 {
        return ""
    }
    return c.providers[0].Name()
}

// HasAvailableProvider returns true if at least one provider in the chain
// is reachable. Uses a short 2-second timeout to avoid blocking handlers.
// Returns false immediately if the chain is nil or empty.
// — P2-01: used by AIHTTPHandler.isReady() for graceful degradation.
func (c *Chain) HasAvailableProvider() bool {
	if c == nil || len(c.providers) == 0 {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, p := range c.providers {
		if p.Available(ctx) {
			return true
		}
	}
	return false
}
