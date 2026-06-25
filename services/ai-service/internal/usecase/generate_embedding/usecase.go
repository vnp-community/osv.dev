package generate_embedding

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

// EmbeddingProvider generates vector embeddings for text input.
// Implemented by: provider.Chain (via adapter below).
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float32, string, error) // returns vector, model name, err
}

// VectorStore persists CVE embedding vectors in PostgreSQL (pgvector).
// Implemented by: internal/infra/vector.PgVectorStore (via adapter below).
type VectorStore interface {
	Save(ctx context.Context, cveID string, embedding []float32, model string) error
}

// UseCase generates vector embeddings for CVE descriptions and persists them.
// [FIX TASK-HC-006] Full implementation — replaces empty shell with real logic.
type UseCase struct {
	embedder    EmbeddingProvider
	vectorStore VectorStore
	log         zerolog.Logger
}

// New creates a UseCase. embedder must not be nil; vectorStore may be nil (skip persist).
func New(embedder EmbeddingProvider, vectorStore VectorStore, log zerolog.Logger) *UseCase {
	return &UseCase{
		embedder:    embedder,
		vectorStore: vectorStore,
		log:         log,
	}
}

// Execute generates an embedding for the given CVE text and persists it.
// Returns the float32 vector slice.
func (uc *UseCase) Execute(ctx context.Context, cveID, text string) ([]float32, error) {
	if cveID == "" || text == "" {
		return nil, fmt.Errorf("generate_embedding: cveID and text are required")
	}

	// 1. Generate embedding via provider chain
	vec, modelName, err := uc.embedder.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("generate_embedding.Execute embed %s: %w", cveID, err)
	}
	if len(vec) == 0 {
		return nil, fmt.Errorf("generate_embedding.Execute: empty vector returned for %s", cveID)
	}

	// 2. Persist to pgvector (best-effort — do not fail the request on storage error)
	if uc.vectorStore != nil {
		if err := uc.vectorStore.Save(ctx, cveID, vec, modelName); err != nil {
			uc.log.Warn().Err(err).Str("cve_id", cveID).Msg("generate_embedding: vector store persist failed")
		}
	}

	uc.log.Info().
		Str("cve_id", cveID).
		Int("dims", len(vec)).
		Str("model", modelName).
		Msg("embedding generated")

	return vec, nil
}
