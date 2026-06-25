// Package pgvector — pgvector_store.go
// PgVectorStore provides CVE embedding storage and cosine similarity search
// using the pgvector PostgreSQL extension.
//
// S3-AI-02: Vector Storage for ai-service
// ADDITIVE: new infra package, no existing code modified.
package pgvector

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PgVectorStore persists and searches CVE embeddings in PostgreSQL (pgvector).
type PgVectorStore struct {
	db *pgxpool.Pool
}

// New creates a new PgVectorStore.
func New(db *pgxpool.Pool) *PgVectorStore {
	return &PgVectorStore{db: db}
}

// Save upserts a CVE embedding (1536-dim float32 vector).
func (s *PgVectorStore) Save(ctx context.Context, cveID string, embedding []float32, model string) error {
	if len(embedding) == 0 {
		return fmt.Errorf("pgvector.Save: empty embedding for %s", cveID)
	}

	// pgvector expects a Go []float32 represented as vector literal '[0.1,0.2,...]'
	vecLiteral := float32SliceToLiteral(embedding)

	_, err := s.db.Exec(ctx, `
		INSERT INTO cve_embeddings (cve_id, embedding, model, created_at)
		VALUES ($1, $2::vector, $3, NOW())
		ON CONFLICT (cve_id)
		DO UPDATE SET embedding = EXCLUDED.embedding,
		              model = EXCLUDED.model,
		              created_at = NOW()
	`, cveID, vecLiteral, model)
	return err
}

// SimilaritySearch returns the top-k CVE IDs most similar to the query embedding.
// Uses IVFFlat cosine similarity index (defined in migration 002_vector_store.sql).
func (s *PgVectorStore) SimilaritySearch(ctx context.Context, queryEmbedding []float32, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}

	vecLiteral := float32SliceToLiteral(queryEmbedding)

	rows, err := s.db.Query(ctx, `
		SELECT cve_id,
		       1 - (embedding <=> $1::vector) AS similarity
		FROM cve_embeddings
		ORDER BY embedding <=> $1::vector
		LIMIT $2
	`, vecLiteral, limit)
	if err != nil {
		return nil, fmt.Errorf("pgvector.SimilaritySearch: %w", err)
	}
	defer rows.Close()

	var cveIDs []string
	for rows.Next() {
		var id string
		var similarity float64
		if err := rows.Scan(&id, &similarity); err != nil {
			continue
		}
		cveIDs = append(cveIDs, id)
	}
	return cveIDs, rows.Err()
}

// Get retrieves the embedding for a single CVE.
// Returns nil if the CVE has no embedding stored.
func (s *PgVectorStore) Get(ctx context.Context, cveID string) ([]float32, error) {
	var vecLiteral string
	err := s.db.QueryRow(ctx,
		`SELECT embedding::text FROM cve_embeddings WHERE cve_id = $1`,
		cveID,
	).Scan(&vecLiteral)
	if err != nil {
		return nil, nil // not found = nil
	}
	return parsePgVectorLiteral(vecLiteral), nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// float32SliceToLiteral converts []float32 to pgvector literal "[0.1,0.2,...]".
func float32SliceToLiteral(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	out := make([]byte, 0, len(v)*8)
	out = append(out, '[')
	for i, f := range v {
		if i > 0 {
			out = append(out, ',')
		}
		out = fmt.Appendf(out, "%g", f)
	}
	out = append(out, ']')
	return string(out)
}

// parsePgVectorLiteral converts pgvector text "[0.1,0.2,...]" back to []float32.
func parsePgVectorLiteral(s string) []float32 {
	if len(s) < 2 {
		return nil
	}
	// Strip brackets
	s = s[1 : len(s)-1]
	if s == "" {
		return nil
	}
	// Parse comma-separated floats
	var result []float32
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			var f float32
			fmt.Sscanf(s[start:i], "%g", &f)
			result = append(result, f)
			start = i + 1
		}
	}
	return result
}
