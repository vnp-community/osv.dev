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
