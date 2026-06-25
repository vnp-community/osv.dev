# TASK-AI-001 — LLM Provider Chain (Ollama + OpenAI Failover)

| Field | Value |
|-------|-------|
| **Task ID** | T-AI-001 |
| **Service** | `ai-service` |
| **Status** | ✅ Completed |
| **Solution Ref** | SOL-OVS-005 §6 Ollama Provider, §2.1 Provider Chain |
| **Priority** | 🟡 Medium |
| **Depends On** | — |
| **Estimated** | 4h |

---

## Context

`ai-service` đã tồn tại tại `services/ai-service/`. Task này implement provider abstraction cho LLM inference với:
- **Ollama** (local, privacy-first) → primary
- **OpenAI** (cloud, higher quality) → fallback
- **Provider chain**: thử providers theo thứ tự, failover khi provider fail

---

## Goal

Implement `LLMProvider` interface + 2 provider implementations + `ProviderChain` (failover).

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/ai-service/internal/provider/interface.go` |
| CREATE | `services/ai-service/internal/provider/ollama/provider.go` |
| CREATE | `services/ai-service/internal/provider/openai/provider.go` |
| CREATE | `services/ai-service/internal/provider/chain.go` |
| CREATE | `services/ai-service/internal/provider/chain_test.go` |

---

## Implementation

### File 1: `services/ai-service/internal/provider/interface.go`

```go
package provider

import "context"

// LLMProvider is the abstract interface for AI inference backends
type LLMProvider interface {
    // Name returns the provider identifier (e.g., "ollama", "openai")
    Name() string

    // GenerateEmbedding creates a vector embedding for the given text.
    // The returned slice length matches Dimensions().
    GenerateEmbedding(ctx context.Context, text string) ([]float32, error)

    // Generate creates a text completion for the given prompt.
    // Implementations should request JSON output when appropriate.
    Generate(ctx context.Context, prompt string) (string, error)

    // Dimensions returns the embedding vector size for this provider/model.
    // OpenAI text-embedding-3-small = 1536
    // Ollama nomic-embed-text = 768
    // Ollama llama3 = 4096
    Dimensions() int

    // Available checks if the provider is reachable (health check).
    Available(ctx context.Context) bool
}
```

### File 2: `services/ai-service/internal/provider/ollama/provider.go`

```go
package ollama

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/rs/zerolog"
)

// Provider implements the LLMProvider interface for local Ollama inference
type Provider struct {
    baseURL        string // e.g., "http://ollama:11434"
    llmModel       string // e.g., "llama3"
    embeddingModel string // e.g., "nomic-embed-text"
    client         *http.Client
    logger         zerolog.Logger
}

// New creates an Ollama provider
func New(baseURL, llmModel, embeddingModel string, timeout time.Duration, logger zerolog.Logger) *Provider {
    return &Provider{
        baseURL:        baseURL,
        llmModel:       llmModel,
        embeddingModel: embeddingModel,
        client: &http.Client{
            Timeout: timeout,
        },
        logger: logger,
    }
}

func (p *Provider) Name() string { return "ollama" }

// Dimensions returns embedding size for the configured model
// nomic-embed-text = 768, llama3 = 4096 (used as embedding fallback)
func (p *Provider) Dimensions() int {
    switch p.embeddingModel {
    case "nomic-embed-text", "nomic-embed-text:latest":
        return 768
    default:
        return 4096
    }
}

// GenerateEmbedding calls /api/embeddings on Ollama
func (p *Provider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
    payload := map[string]string{
        "model":  p.embeddingModel,
        "prompt": text,
    }

    body, err := p.post(ctx, "/api/embeddings", payload)
    if err != nil {
        return nil, fmt.Errorf("ollama embedding: %w", err)
    }

    var result struct {
        Embedding []float32 `json:"embedding"`
    }
    if err := json.Unmarshal(body, &result); err != nil {
        return nil, fmt.Errorf("parse embedding response: %w", err)
    }
    if len(result.Embedding) == 0 {
        return nil, fmt.Errorf("empty embedding returned by ollama")
    }

    return result.Embedding, nil
}

// Generate calls /api/generate on Ollama with JSON format enforced
func (p *Provider) Generate(ctx context.Context, prompt string) (string, error) {
    payload := map[string]interface{}{
        "model":  p.llmModel,
        "prompt": prompt,
        "stream": false,
        "format": "json", // Force JSON output for structured responses
        "options": map[string]interface{}{
            "temperature": 0.1, // Low temperature for consistent/deterministic output
            "top_p":       0.9,
        },
    }

    body, err := p.post(ctx, "/api/generate", payload)
    if err != nil {
        return "", fmt.Errorf("ollama generate: %w", err)
    }

    var result struct {
        Response string `json:"response"`
        Done     bool   `json:"done"`
    }
    if err := json.Unmarshal(body, &result); err != nil {
        return "", fmt.Errorf("parse generate response: %w", err)
    }

    return result.Response, nil
}

// Available checks if Ollama is running by hitting /api/tags
func (p *Provider) Available(ctx context.Context) bool {
    ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/tags", nil)
    if err != nil {
        return false
    }

    resp, err := p.client.Do(req)
    if err != nil {
        return false
    }
    defer resp.Body.Close()

    return resp.StatusCode == http.StatusOK
}

// post sends a JSON POST request to the Ollama API
func (p *Provider) post(ctx context.Context, path string, payload interface{}) ([]byte, error) {
    data, err := json.Marshal(payload)
    if err != nil {
        return nil, fmt.Errorf("marshal payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost,
        p.baseURL+path, bytes.NewReader(data))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("http request to %s%s: %w", p.baseURL, path, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
        return nil, fmt.Errorf("ollama returned HTTP %d: %s", resp.StatusCode, string(errBody))
    }

    return io.ReadAll(resp.Body)
}
```

### File 3: `services/ai-service/internal/provider/openai/provider.go`

```go
package openai

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/rs/zerolog"
)

// Provider implements LLMProvider for OpenAI API
type Provider struct {
    apiKey         string
    baseURL        string // default: "https://api.openai.com/v1"
    llmModel       string // e.g., "gpt-4o-mini"
    embeddingModel string // e.g., "text-embedding-3-small"
    client         *http.Client
    logger         zerolog.Logger
}

// New creates an OpenAI provider
func New(apiKey, baseURL, llmModel, embeddingModel string, timeout time.Duration, logger zerolog.Logger) *Provider {
    if baseURL == "" {
        baseURL = "https://api.openai.com/v1"
    }
    return &Provider{
        apiKey:         apiKey,
        baseURL:        baseURL,
        llmModel:       llmModel,
        embeddingModel: embeddingModel,
        client:         &http.Client{Timeout: timeout},
        logger:         logger,
    }
}

func (p *Provider) Name() string { return "openai" }

// Dimensions returns embedding vector size
// text-embedding-3-small = 1536, text-embedding-3-large = 3072
func (p *Provider) Dimensions() int {
    switch p.embeddingModel {
    case "text-embedding-3-large":
        return 3072
    case "text-embedding-ada-002":
        return 1536
    default: // text-embedding-3-small
        return 1536
    }
}

// GenerateEmbedding calls /v1/embeddings on OpenAI
func (p *Provider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
    payload := map[string]interface{}{
        "model": p.embeddingModel,
        "input": text,
    }

    body, err := p.post(ctx, "/embeddings", payload)
    if err != nil {
        return nil, fmt.Errorf("openai embedding: %w", err)
    }

    var result struct {
        Data []struct {
            Embedding []float32 `json:"embedding"`
            Index     int       `json:"index"`
        } `json:"data"`
    }
    if err := json.Unmarshal(body, &result); err != nil {
        return nil, fmt.Errorf("parse embedding response: %w", err)
    }
    if len(result.Data) == 0 {
        return nil, fmt.Errorf("empty embedding data returned by OpenAI")
    }

    return result.Data[0].Embedding, nil
}

// Generate calls /v1/chat/completions on OpenAI
func (p *Provider) Generate(ctx context.Context, prompt string) (string, error) {
    payload := map[string]interface{}{
        "model": p.llmModel,
        "messages": []map[string]string{
            {
                "role":    "system",
                "content": "You are a security expert. Always respond with valid JSON only.",
            },
            {
                "role":    "user",
                "content": prompt,
            },
        },
        "temperature":     0.1,
        "max_tokens":      1000,
        "response_format": map[string]string{"type": "json_object"},
    }

    body, err := p.post(ctx, "/chat/completions", payload)
    if err != nil {
        return "", fmt.Errorf("openai completion: %w", err)
    }

    var result struct {
        Choices []struct {
            Message struct {
                Content string `json:"content"`
            } `json:"message"`
        } `json:"choices"`
    }
    if err := json.Unmarshal(body, &result); err != nil {
        return "", fmt.Errorf("parse completion response: %w", err)
    }
    if len(result.Choices) == 0 {
        return "", fmt.Errorf("no choices returned by OpenAI")
    }

    return result.Choices[0].Message.Content, nil
}

// Available checks if OpenAI API is reachable (simple model list request)
func (p *Provider) Available(ctx context.Context) bool {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/models", nil)
    if err != nil {
        return false
    }
    req.Header.Set("Authorization", "Bearer "+p.apiKey)

    resp, err := p.client.Do(req)
    if err != nil {
        return false
    }
    defer resp.Body.Close()

    return resp.StatusCode == http.StatusOK
}

// post sends a JSON POST request to OpenAI API
func (p *Provider) post(ctx context.Context, path string, payload interface{}) ([]byte, error) {
    data, err := json.Marshal(payload)
    if err != nil {
        return nil, err
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(data))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+p.apiKey)

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("http request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
        return nil, fmt.Errorf("openai returned HTTP %d: %s", resp.StatusCode, string(errBody))
    }

    return io.ReadAll(resp.Body)
}
```

### File 4: `services/ai-service/internal/provider/chain.go`

```go
package provider

import (
    "context"
    "fmt"
    "strings"

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
    var lastErr error
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

        lastErr = fmt.Errorf("provider %s: %w", p.Name(), err)
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
```

### File 5: `services/ai-service/internal/provider/chain_test.go`

```go
package provider

import (
    "context"
    "errors"
    "testing"

    "github.com/rs/zerolog"
)

// mockProvider is a controllable LLM provider for testing
type mockProvider struct {
    name       string
    dims       int
    embedErr   error
    generateErr error
    embedding  []float32
    response   string
    callCount  int
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Dimensions() int { return m.dims }
func (m *mockProvider) Available(_ context.Context) bool { return m.embedErr == nil }

func (m *mockProvider) GenerateEmbedding(_ context.Context, _ string) ([]float32, error) {
    m.callCount++
    if m.embedErr != nil {
        return nil, m.embedErr
    }
    return m.embedding, nil
}

func (m *mockProvider) Generate(_ context.Context, _ string) (string, error) {
    m.callCount++
    if m.generateErr != nil {
        return "", m.generateErr
    }
    return m.response, nil
}

func TestChain_EmbeddingPrimarySuccess(t *testing.T) {
    primary := &mockProvider{
        name:      "primary",
        dims:      768,
        embedding: []float32{0.1, 0.2, 0.3},
    }
    secondary := &mockProvider{name: "secondary", dims: 1536}

    chain := NewChain(zerolog.Nop(), primary, secondary)
    emb, provider, err := chain.GenerateEmbedding(context.Background(), "test text")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if provider != "primary" {
        t.Errorf("expected primary provider, got %s", provider)
    }
    if len(emb) != 3 {
        t.Errorf("embedding length = %d, want 3", len(emb))
    }
    if secondary.callCount != 0 {
        t.Error("secondary provider should NOT be called when primary succeeds")
    }
}

func TestChain_EmbeddingFallback(t *testing.T) {
    primary := &mockProvider{
        name:     "primary",
        embedErr: errors.New("primary unavailable"),
    }
    secondary := &mockProvider{
        name:      "secondary",
        dims:      1536,
        embedding: []float32{0.4, 0.5, 0.6},
    }

    chain := NewChain(zerolog.Nop(), primary, secondary)
    emb, provider, err := chain.GenerateEmbedding(context.Background(), "test")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if provider != "secondary" {
        t.Errorf("expected fallback to secondary, got %s", provider)
    }
    if len(emb) != 3 {
        t.Errorf("embedding length = %d, want 3", len(emb))
    }
    if primary.callCount != 1 {
        t.Error("primary should have been called once (and failed)")
    }
    if secondary.callCount != 1 {
        t.Error("secondary should have been called once")
    }
}

func TestChain_AllProvidersFailEmbedding(t *testing.T) {
    primary := &mockProvider{name: "p1", embedErr: errors.New("p1 fail")}
    secondary := &mockProvider{name: "p2", embedErr: errors.New("p2 fail")}

    chain := NewChain(zerolog.Nop(), primary, secondary)
    _, _, err := chain.GenerateEmbedding(context.Background(), "test")
    if err == nil {
        t.Error("expected error when all providers fail")
    }
}

func TestChain_GenerateFallback(t *testing.T) {
    primary := &mockProvider{
        name:        "ollama",
        generateErr: errors.New("ollama timeout"),
    }
    secondary := &mockProvider{
        name:     "openai",
        response: `{"severity": "High"}`,
    }

    chain := NewChain(zerolog.Nop(), primary, secondary)
    result, provider, err := chain.Generate(context.Background(), "classify this")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if provider != "openai" {
        t.Errorf("expected openai fallback, got %s", provider)
    }
    if result != `{"severity": "High"}` {
        t.Errorf("unexpected result: %s", result)
    }
}

func TestChain_EmptyChain(t *testing.T) {
    chain := NewChain(zerolog.Nop())
    _, _, err := chain.GenerateEmbedding(context.Background(), "test")
    if err == nil {
        t.Error("empty chain should return error")
    }
}
```

---

## Verification

```bash
cd services/ai-service
go build ./internal/provider/...
go test ./internal/provider/... -v
```

**Expected**:
```
--- PASS: TestChain_EmbeddingPrimarySuccess
--- PASS: TestChain_EmbeddingFallback
--- PASS: TestChain_AllProvidersFailEmbedding
--- PASS: TestChain_GenerateFallback
--- PASS: TestChain_EmptyChain
```

### Checklist

- [x] `GenerateEmbedding(ctx, text)` → primary success = no secondary call
- [x] Primary fails → secondary is called (fallback)
- [x] All providers fail → error returned
- [x] `Generate(ctx, prompt)` → same failover logic
- [x] Prometheus metrics registered (no panic at init)
- [x] Ollama uses `format: "json"` in payload
- [x] OpenAI uses `response_format: {"type": "json_object"}`
- [x] OpenAI uses `Authorization: Bearer` header
- [x] Temperature = 0.1 for deterministic output
