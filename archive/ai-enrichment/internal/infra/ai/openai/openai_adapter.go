// Package openai implements LLMProvider and EmbeddingProvider backed by OpenAI API.
// Supports GPT-4o-mini (default) and text-embedding-3-small.
package openai

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
)

const (
	openAIBaseURL          = "https://api.openai.com/v1"
	defaultChatModel       = "gpt-4o-mini"
	defaultEmbeddingModel  = "text-embedding-3-small"
	embeddingDimension     = 768 // text-embedding-3-small supports 768 via dimensions param
)

// Client wraps the OpenAI API.
type Client struct {
	apiKey     string
	chatModel  string
	embedModel string
	httpClient *http.Client
	log        zerolog.Logger
}

// NewClient creates an OpenAI client.
func NewClient(apiKey, chatModel, embedModel string, log zerolog.Logger) *Client {
	if chatModel == "" {
		chatModel = defaultChatModel
	}
	if embedModel == "" {
		embedModel = defaultEmbeddingModel
	}
	return &Client{
		apiKey:     apiKey,
		chatModel:  chatModel,
		embedModel: embedModel,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		log:        log,
	}
}

// ── LLMProvider ───────────────────────────────────────────────────────────────

// Complete sends a prompt and unmarshals the structured JSON response.
func (c *Client) Complete(ctx context.Context, prompt string, result interface{}) error {
	reqBody := map[string]interface{}{
		"model": c.chatModel,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a security expert. Respond ONLY with valid JSON."},
			{"role": "user", "content": prompt},
		},
		"temperature":     0.1,
		"max_tokens":      1024,
		"response_format": map[string]string{"type": "json_object"},
	}

	respData, err := c.doRequestWithRetry(ctx, "/chat/completions", reqBody)
	if err != nil {
		return err
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respData, &chatResp); err != nil {
		return fmt.Errorf("parse OpenAI response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return fmt.Errorf("no choices in OpenAI response")
	}

	c.log.Debug().
		Int("prompt_tokens", chatResp.Usage.PromptTokens).
		Int("completion_tokens", chatResp.Usage.CompletionTokens).
		Str("model", c.chatModel).
		Msg("OpenAI token usage")

	text := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	if err := json.Unmarshal([]byte(text), result); err != nil {
		return fmt.Errorf("parse structured response: %w", err)
	}
	return nil
}

// ── EmbeddingProvider ─────────────────────────────────────────────────────────

// GenerateEmbedding generates a single text embedding.
func (c *Client) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model":      c.embedModel,
		"input":      text,
		"dimensions": embeddingDimension,
	}

	respData, err := c.doRequestWithRetry(ctx, "/embeddings", reqBody)
	if err != nil {
		return nil, err
	}

	var embedResp struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respData, &embedResp); err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}

	if len(embedResp.Data) == 0 || len(embedResp.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding in OpenAI response")
	}
	return embedResp.Data[0].Embedding, nil
}

// ── HTTP helper ───────────────────────────────────────────────────────────────

func (c *Client) doRequestWithRetry(ctx context.Context, path string, body interface{}) ([]byte, error) {
	delays := []time.Duration{0, 1 * time.Second, 3 * time.Second}

	var lastErr error
	for attempt, delay := range delays {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
		data, err := c.doRequest(ctx, path, body)
		if err == nil {
			return data, nil
		}
		lastErr = err

		if strings.Contains(err.Error(), "400") || strings.Contains(err.Error(), "401") {
			return nil, err // non-retryable
		}
		c.log.Warn().Err(err).Int("attempt", attempt+1).Str("path", path).Msg("OpenAI retry")
	}
	return nil, lastErr
}

func (c *Client) doRequest(ctx context.Context, path string, body interface{}) ([]byte, error) {
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIBaseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request: %w", err)
	}
	defer resp.Body.Close()

	respData, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("OpenAI rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("OpenAI %s error %d: %s", path, resp.StatusCode, string(respData)[:min(200, len(respData))])
	}
	return respData, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
