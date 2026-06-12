// application/port/embedding_provider.go
package port

import "context"

// EmbeddingProvider is the interface for generating text embeddings.
// All concrete adapters (Vertex, OpenAI, Ollama) implement this interface.
type EmbeddingProvider interface {
	// GenerateEmbedding generates an embedding vector for a single text.
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

// BatchEmbeddingProvider extends EmbeddingProvider with batch support.
type BatchEmbeddingProvider interface {
	EmbeddingProvider
	// GenerateEmbeddingBatch generates embeddings for multiple texts.
	GenerateEmbeddingBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// LLMProvider is the interface for LLM text generation.
// All concrete adapters (Vertex, OpenAI, Ollama) implement this interface.
type LLMProvider interface {
	// GenerateStructured generates structured output from a prompt, unmarshaling into out.
	GenerateStructured(ctx context.Context, prompt string, out interface{}) error
}

// EventPublisher publishes domain events after enrichment completes.
type EventPublisher interface {
	Publish(ctx context.Context, payload interface{}) error
}
