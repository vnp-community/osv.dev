// Package generate_embedding — adapters.go
// Adapters connecting infrastructure implementations to the UseCase port interfaces.
// This keeps the domain UseCase free of infra imports.
package generate_embedding

import (
	"context"
	"fmt"

	pgvectorinfra "github.com/osv/ai-service/internal/infra/vector"
	"github.com/osv/ai-service/internal/provider"
)

// ── Chain Adapter ────────────────────────────────────────────────────────────

// ChainAdapter adapts provider.Chain to the EmbeddingProvider interface.
// provider.Chain.GenerateEmbedding returns ([]float32, string, error);
// we forward that directly.
type ChainAdapter struct {
	chain *provider.Chain
}

// NewChainAdapter creates an EmbeddingProvider backed by a provider.Chain.
func NewChainAdapter(chain *provider.Chain) *ChainAdapter {
	return &ChainAdapter{chain: chain}
}

// Embed generates an embedding via the provider chain.
func (a *ChainAdapter) Embed(ctx context.Context, text string) ([]float32, string, error) {
	if a.chain == nil {
		return nil, "", fmt.Errorf("chain adapter: no provider chain configured")
	}
	vec, model, err := a.chain.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, "", fmt.Errorf("chain embed: %w", err)
	}
	return vec, model, nil
}

// ── PgVector Store Adapter ───────────────────────────────────────────────────

// PgVectorAdapter adapts infra/vector.PgVectorStore to the VectorStore interface.
type PgVectorAdapter struct {
	store *pgvectorinfra.PgVectorStore
}

// NewPgVectorAdapter wraps a PgVectorStore as a VectorStore.
func NewPgVectorAdapter(store *pgvectorinfra.PgVectorStore) *PgVectorAdapter {
	return &PgVectorAdapter{store: store}
}

// Save persists an embedding via PgVectorStore.
func (a *PgVectorAdapter) Save(ctx context.Context, cveID string, embedding []float32, model string) error {
	if a.store == nil {
		return nil // no-op when not configured
	}
	return a.store.Save(ctx, cveID, embedding, model)
}
