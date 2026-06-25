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
