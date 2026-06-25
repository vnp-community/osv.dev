// infra/ai/ollama/ollama_adapter.go — Self-hosted Ollama adapter for development
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// [FIX BUG-007] defaultOllamaURL removed — caller must set AI_BASE_URL env var.
// Using localhost:11434 will fail in container deployments.

// OllamaAdapter implements both EmbeddingProvider and LLMProvider using Ollama.
type OllamaAdapter struct {
	baseURL        string
	embeddingModel string
	llmModel       string
	client         *http.Client
	log            zerolog.Logger
}

// NewOllamaAdapter creates an Ollama adapter for local development/testing.
// [FIX BUG-007] baseURL must not be empty in production — set AI_BASE_URL env var.
// Logs a WARN when baseURL is empty (will fail at runtime for Embed/Generate calls).
func NewOllamaAdapter(baseURL, embeddingModel, llmModel string, log zerolog.Logger) *OllamaAdapter {
	if baseURL == "" {
		slog.Warn("Ollama baseURL is empty — set AI_BASE_URL env var; AI features will fail",
			"hint", "set AI_BASE_URL=http://ollama:11434 or AI_BASE_URL=http://localhost:11434 for local dev")
	}
	return &OllamaAdapter{
		baseURL:        baseURL,
		embeddingModel: embeddingModel,
		llmModel:       llmModel,
		client: &http.Client{
			Timeout: 2 * time.Minute,
		},
		log: log,
	}
}

// ── EmbeddingProvider ─────────────────────────────────────────────────────────

type ollamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed generates an embedding for a single text.
func (a *OllamaAdapter) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody, _ := json.Marshal(ollamaEmbedRequest{Model: a.embeddingModel, Prompt: text})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.baseURL+"/api/embeddings", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	defer resp.Body.Close()

	var result ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode ollama embed response: %w", err)
	}
	return result.Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts sequentially.
func (a *OllamaAdapter) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := a.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed text[%d]: %w", i, err)
		}
		results[i] = emb
	}
	return results, nil
}

// Dimension returns 0 since Ollama model dimensions vary.
func (a *OllamaAdapter) Dimension() int { return 0 }

// ModelName returns the embedding model name.
func (a *OllamaAdapter) ModelName() string { return a.embeddingModel }

// ── LLMProvider ──────────────────────────────────────────────────────────────

type ollamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format,omitempty"` // "json" for structured output
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
}

// GenerateStructured calls Ollama in JSON mode and unmarshals the response into out.
func (a *OllamaAdapter) GenerateStructured(ctx context.Context, prompt string, out interface{}) error {
	text, err := a.generate(ctx, prompt, true)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(text), out)
}

// GenerateText calls Ollama for free-form text generation.
func (a *OllamaAdapter) GenerateText(ctx context.Context, prompt string, maxTokens int) (string, error) {
	return a.generate(ctx, prompt, false)
}

func (a *OllamaAdapter) generate(ctx context.Context, prompt string, jsonMode bool) (string, error) {
	format := ""
	if jsonMode {
		format = "json"
	}
	reqBody, _ := json.Marshal(ollamaGenerateRequest{
		Model: a.llmModel, Prompt: prompt, Stream: false, Format: format,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.baseURL+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama generate: %w", err)
	}
	defer resp.Body.Close()

	var result ollamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode ollama response: %w", err)
	}
	return result.Response, nil
}

// Name returns the provider name.
func (a *OllamaAdapter) Name() string { return "ollama" }

// LLMModelName returns the LLM model name.
// ModelName (line 94) already returns the embedding model name.
func (a *OllamaAdapter) LLMModelName() string { return a.llmModel }
