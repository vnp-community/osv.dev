// adapter_shim.go — Method shims to satisfy port interfaces.
// Vertex and OpenAI use Complete() internally; this file adds GenerateStructured() wrappers.
// Ollama uses Embed(); this adds GenerateEmbedding() wrapper.
package ollama

import "context"

// GenerateEmbedding satisfies port.EmbeddingProvider by delegating to Embed.
func (a *OllamaAdapter) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return a.Embed(ctx, text)
}
