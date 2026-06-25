package pgvector

import (
    "context"
    "fmt"

    "github.com/jmoiron/sqlx"
    "github.com/pgvector/pgvector-go"

    "github.com/osv/search-service/internal/domain/entity"
)

type SemanticSearcher struct {
    db *sqlx.DB
}

func New(db *sqlx.DB) *SemanticSearcher {
    return &SemanticSearcher{db: db}
}

type SemanticSearchResult struct {
    entity.CVE
    SimilarityScore float64 `db:"similarity_score"`
}

// Search finds CVEs similar to query embedding using cosine distance.
func (s *SemanticSearcher) Search(ctx context.Context, embedding []float32, limit int) ([]*SemanticSearchResult, error) {
    if limit <= 0 || limit > 100 { limit = 20 }

    rows, err := s.db.QueryxContext(ctx, `
        SELECT id, description, severity, COALESCE(epss, 0) as epss,
               published, source, 1 - (embedding <=> $1) as similarity_score
        FROM cves
        WHERE embedding IS NOT NULL
        ORDER BY embedding <=> $1
        LIMIT $2
    `, pgvector.NewVector(embedding), limit)
    if err != nil {
        return nil, fmt.Errorf("pgvector: query: %w", err)
    }
    defer rows.Close()

    var cves []*SemanticSearchResult
    for rows.Next() {
        var cve SemanticSearchResult
        if err := rows.StructScan(&cve); err != nil { continue }
        cves = append(cves, &cve)
    }
    return cves, rows.Err()
}

// AIEmbedder generates text embeddings.
type AIEmbedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
}

// NOTE: MockEmbedder has been removed from production code (TASK-HC-005).
// Use aigrpc.NewEmbedder(addr) in embedded.go to wire a real AI provider.
// For tests, define a local mock implementing AIEmbedder in your test package.

// SemanticSearchRequest is the input for semantic search.
type SemanticSearchRequest struct {
    Query string // natural language query
    Limit int    // max results (default 20, max 100)
}

// UseCase orchestrates embedding + vector search.
type UseCase struct {
    searcher *SemanticSearcher
    embedder AIEmbedder
}

func NewUseCase(searcher *SemanticSearcher, embedder AIEmbedder) *UseCase {
    return &UseCase{searcher: searcher, embedder: embedder}
}

func (uc *UseCase) Execute(ctx context.Context, req *SemanticSearchRequest) ([]*SemanticSearchResult, error) {
    // 1. Get embedding from ai-service
    embedding, err := uc.embedder.Embed(ctx, req.Query)
    if err != nil {
        return nil, fmt.Errorf("semantic: embed failed: %w", err)
    }

    // 2. Vector similarity search
    limit := req.Limit
    if limit == 0 { limit = 20 }
    return uc.searcher.Search(ctx, embedding, limit)
}
