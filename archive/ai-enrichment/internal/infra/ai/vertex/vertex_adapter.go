// Package vertex implements production AI adapters for Google Cloud Vertex AI.
// Supports text-embedding-004 for embeddings and Gemini 1.5 Flash/Pro for LLM.
package vertex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	embeddingModel    = "text-embedding-004"
	embeddingDim      = 768
	geminiFlashModel  = "gemini-1.5-flash-002"
	geminiProModel    = "gemini-1.5-pro-002"
	maxBatchEmbedding = 250 // VertexAI max batch size

	vertexBaseURL = "https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models"
)

// Config holds VertexAI configuration.
type Config struct {
	ProjectID      string
	Region         string // e.g. "us-central1"
	FallbackRegion string // e.g. "us-east4"
}

// VertexEmbeddingAdapter implements EmbeddingProvider backed by VertexAI text-embedding-004.
type VertexEmbeddingAdapter struct {
	config     Config
	httpClient *http.Client
	log        zerolog.Logger
}

// NewVertexEmbeddingAdapter creates a production VertexAI embedding adapter.
// Uses Application Default Credentials (ADC) — set GOOGLE_APPLICATION_CREDENTIALS or run on GCP.
func NewVertexEmbeddingAdapter(config Config, log zerolog.Logger) (*VertexEmbeddingAdapter, error) {
	ts, err := google.DefaultTokenSource(context.Background(),
		"https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		return nil, fmt.Errorf("VertexAI credentials: %w", err)
	}

	return &VertexEmbeddingAdapter{
		config: config,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &oauth2.Transport{Source: ts, Base: http.DefaultTransport},
		},
		log: log,
	}, nil
}

// GenerateEmbedding generates a single text embedding.
func (a *VertexEmbeddingAdapter) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	results, err := a.GenerateEmbeddingBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return results[0], nil
}

// GenerateEmbeddingBatch generates embeddings for multiple texts (max 250 per call).
func (a *VertexEmbeddingAdapter) GenerateEmbeddingBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if len(texts) > maxBatchEmbedding {
		return nil, fmt.Errorf("batch size %d exceeds max %d", len(texts), maxBatchEmbedding)
	}

	instances := make([]map[string]string, len(texts))
	for i, t := range texts {
		instances[i] = map[string]string{
			"content":   t,
			"task_type": "RETRIEVAL_DOCUMENT",
		}
	}

	reqBody := map[string]interface{}{
		"instances": instances,
		"parameters": map[string]interface{}{
			"outputDimensionality": embeddingDim,
		},
	}

	url := fmt.Sprintf("%s/%s:predict",
		fmt.Sprintf(vertexBaseURL, a.config.Region, a.config.ProjectID, a.config.Region),
		embeddingModel,
	)

	respData, err := a.doRequest(ctx, url, reqBody)
	if err != nil {
		// Try fallback region
		if a.config.FallbackRegion != "" && a.config.FallbackRegion != a.config.Region {
			a.log.Warn().Err(err).Str("region", a.config.Region).Msg("primary region failed, trying fallback")
			url = fmt.Sprintf("%s/%s:predict",
				fmt.Sprintf(vertexBaseURL, a.config.FallbackRegion, a.config.ProjectID, a.config.FallbackRegion),
				embeddingModel,
			)
			respData, err = a.doRequest(ctx, url, reqBody)
			if err != nil {
				return nil, fmt.Errorf("VertexAI embedding failed in both regions: %w", err)
			}
		} else {
			return nil, err
		}
	}

	var result struct {
		Predictions []struct {
			Embeddings struct {
				Values []float32 `json:"values"`
			} `json:"embeddings"`
		} `json:"predictions"`
	}
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}

	embeddings := make([][]float32, len(result.Predictions))
	for i, p := range result.Predictions {
		embeddings[i] = p.Embeddings.Values
		if len(embeddings[i]) == 0 {
			return nil, fmt.Errorf("empty embedding for text %d", i)
		}
	}

	return embeddings, nil
}

// ── VertexGeminiAdapter ───────────────────────────────────────────────────────

// VertexGeminiAdapter implements LLMProvider backed by Gemini 1.5 Flash/Pro.
type VertexGeminiAdapter struct {
	config     Config
	model      string // geminiFlashModel | geminiProModel
	httpClient *http.Client
	log        zerolog.Logger
}

// NewVertexGeminiAdapter creates a Gemini adapter. Use flash for speed, pro for quality.
func NewVertexGeminiAdapter(config Config, model string, log zerolog.Logger) (*VertexGeminiAdapter, error) {
	if model == "" {
		model = geminiFlashModel
	}
	ts, err := google.DefaultTokenSource(context.Background(),
		"https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		return nil, fmt.Errorf("VertexAI credentials: %w", err)
	}

	return &VertexGeminiAdapter{
		config: config,
		model:  model,
		httpClient: &http.Client{
			Timeout:   60 * time.Second,
			Transport: &oauth2.Transport{Source: ts, Base: http.DefaultTransport},
		},
		log: log,
	}, nil
}

// Complete sends a prompt to Gemini and unmarshals the structured JSON response into result.
func (a *VertexGeminiAdapter) Complete(ctx context.Context, prompt string, result interface{}) error {
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.1,    // low temperature for structured output
			"maxOutputTokens": 1024,
			"responseMimeType": "application/json",
		},
		"safetySettings": []map[string]string{
			{"category": "HARM_CATEGORY_DANGEROUS_CONTENT", "threshold": "BLOCK_ONLY_HIGH"},
		},
	}

	url := fmt.Sprintf("%s/%s:generateContent",
		fmt.Sprintf(vertexBaseURL, a.config.Region, a.config.ProjectID, a.config.Region),
		a.model,
	)

	respData, err := a.doRequestWithRetry(ctx, url, reqBody)
	if err != nil {
		return err
	}

	// Parse Gemini response
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.Unmarshal(respData, &geminiResp); err != nil {
		return fmt.Errorf("parse Gemini response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return fmt.Errorf("no candidates in Gemini response")
	}

	candidate := geminiResp.Candidates[0]
	if candidate.FinishReason == "SAFETY" {
		return fmt.Errorf("Gemini refused due to safety: escalating to Pro model")
	}

	if len(candidate.Content.Parts) == 0 {
		return fmt.Errorf("empty Gemini response parts")
	}

	// Log token usage for cost tracking
	a.log.Debug().
		Int("prompt_tokens", geminiResp.UsageMetadata.PromptTokenCount).
		Int("output_tokens", geminiResp.UsageMetadata.CandidatesTokenCount).
		Str("model", a.model).
		Msg("Gemini token usage")

	// Parse the JSON text response
	text := candidate.Content.Parts[0].Text
	text = strings.TrimSpace(text)
	// Strip markdown code fences if present
	text = stripCodeFences(text)

	if err := json.Unmarshal([]byte(text), result); err != nil {
		return fmt.Errorf("parse structured response %q: %w", truncate(text, 200), err)
	}
	return nil
}

func (a *VertexGeminiAdapter) doRequestWithRetry(ctx context.Context, url string, body interface{}) ([]byte, error) {
	var lastErr error
	delays := []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second}

	for attempt, delay := range append([]time.Duration{0}, delays...) {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		data, err := (&VertexEmbeddingAdapter{httpClient: a.httpClient}).doRequest(ctx, url, body)
		if err == nil {
			return data, nil
		}
		lastErr = err

		// Don't retry on non-retriable errors
		if strings.Contains(err.Error(), "400") || strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
			return nil, err
		}
		a.log.Warn().Err(err).Int("attempt", attempt+1).Msg("Gemini request failed, retrying")
	}
	return nil, lastErr
}

// ── Shared HTTP helper ────────────────────────────────────────────────────────

func (a *VertexEmbeddingAdapter) doRequest(ctx context.Context, url string, body interface{}) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VertexAI request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("VertexAI rate limit (429): %s", truncate(string(respData), 100))
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("VertexAI error %d: %s", resp.StatusCode, truncate(string(respData), 200))
	}

	return respData, nil
}

// ── Note ──────────────────────────────────────────────────────────────────────
// OAuth2 token injection is handled by golang.org/x/oauth2.Transport.
// Both VertexEmbeddingAdapter and VertexGeminiAdapter use oauth2.Transport{Source: ts}.

// ── Utilities ─────────────────────────────────────────────────────────────────

func stripCodeFences(s string) string {
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
