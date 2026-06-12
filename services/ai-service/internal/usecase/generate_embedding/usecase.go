package generate_embedding

import "context"

// UseCase generates vector embeddings for CVE descriptions
type UseCase struct{}

// New creates a new embedding usecase
func New() *UseCase { return &UseCase{} }

// Execute generates an embedding for the given text
// Returns a float32 slice (e.g., 1536-dim for text-embedding-3-small)
func (uc *UseCase) Execute(ctx context.Context, cveID, text string) ([]float32, error) {
	// TODO: call embedding provider (OpenAI / Vertex AI)
	// TODO: cache result in Redis
	// TODO: store in Firestore
	return nil, nil
}
