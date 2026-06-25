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
