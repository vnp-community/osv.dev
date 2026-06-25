// Package vector — postgres_vector_store.go
// PostgresVectorStore provides CVE embedding storage and semantic similarity search
// for search-service using pgvector.
//
// S3-SEARCH-01: Semantic Search Enhancement
// ADDITIVE: no existing repository interfaces modified.
package vector

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresVectorStore stores and searches CVE embeddings via pgvector.
type PostgresVectorStore struct {
	db *pgxpool.Pool
}

// New creates a new PostgresVectorStore.
func New(db *pgxpool.Pool) *PostgresVectorStore {
	return &PostgresVectorStore{db: db}
}

// Save upserts a CVE embedding vector.
func (s *PostgresVectorStore) Save(ctx context.Context, cveID string, embedding []float32) error {
	vecLiteral := toVectorLiteral(embedding)
	_, err := s.db.Exec(ctx, `
		INSERT INTO cve_embeddings (cve_id, embedding, indexed_at)
		VALUES ($1, $2::vector, NOW())
		ON CONFLICT (cve_id)
		DO UPDATE SET embedding = EXCLUDED.embedding, indexed_at = NOW()
	`, cveID, vecLiteral)
	return err
}

// SimilarCVEs finds similar CVEs using cosine similarity (ORDER BY <=> operator).
// Returns CVE IDs ordered by highest similarity to queryEmbedding.
func (s *PostgresVectorStore) SimilarCVEs(ctx context.Context, queryEmbedding []float32, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}
	vecLiteral := toVectorLiteral(queryEmbedding)

	rows, err := s.db.Query(ctx, `
		SELECT cve_id,
		       1 - (embedding <=> $1::vector) AS similarity
		FROM cve_embeddings
		ORDER BY embedding <=> $1::vector
		LIMIT $2
	`, vecLiteral, limit)
	if err != nil {
		return nil, fmt.Errorf("vector SimilarCVEs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		var sim float64
		if err := rows.Scan(&id, &sim); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ── helpers ──────────────────────────────────────────────────────────────────

func toVectorLiteral(v []float32) string {
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
