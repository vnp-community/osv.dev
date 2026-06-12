// Package ai provides the LLM backend factory for ai-service.
// Select backend via AI_BACKEND env var: "vertex" | "ollama" | "openai"
package ai

import (
	"fmt"
	"os"
)

// Backend identifies which LLM provider to use.
type Backend string

const (
	BackendVertex Backend = "vertex"
	BackendOllama Backend = "ollama"
	BackendOpenAI Backend = "openai"
)

// Config holds common LLM provider configuration.
type Config struct {
	Backend    Backend
	ModelName  string
	ProjectID  string // Vertex AI
	Location   string // Vertex AI
	BaseURL    string // Ollama / OpenAI
	APIKey     string // OpenAI
}

// FromEnv reads Config from environment variables.
func FromEnv() Config {
	return Config{
		Backend:   Backend(envOrDefault("AI_BACKEND", "ollama")),
		ModelName: envOrDefault("AI_MODEL", "llama3"),
		ProjectID: os.Getenv("VERTEX_PROJECT_ID"),
		Location:  envOrDefault("VERTEX_LOCATION", "us-central1"),
		BaseURL:   envOrDefault("AI_BASE_URL", "http://localhost:11434"),
		APIKey:    os.Getenv("OPENAI_API_KEY"),
	}
}

// Validate returns an error if the config is invalid for the chosen backend.
func (c Config) Validate() error {
	switch c.Backend {
	case BackendVertex:
		if c.ProjectID == "" {
			return fmt.Errorf("VERTEX_PROJECT_ID required for vertex backend")
		}
	case BackendOpenAI:
		if c.APIKey == "" {
			return fmt.Errorf("OPENAI_API_KEY required for openai backend")
		}
	case BackendOllama:
		// no mandatory fields
	default:
		return fmt.Errorf("unknown AI_BACKEND: %q (valid: vertex, ollama, openai)", c.Backend)
	}
	return nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
