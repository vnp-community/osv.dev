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
