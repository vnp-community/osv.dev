// Package repository defines search-side repository interfaces and supporting types.
package repository

import (
	"context"
	"time"

	"github.com/osv/search-service/internal/domain/entity"
	"github.com/osv/search-service/internal/domain/valueobject"
)

// SearchFilters holds optional filter criteria for search queries.
type SearchFilters struct {
	Ecosystems       []string
	Severities       []string
	DateFrom         time.Time
	DateTo           time.Time
	IncludeWithdrawn bool
	Sources          []string
}

// SearchParams is the canonical search request passed to the adapter.
type SearchParams struct {
	Query     string
	Filters   SearchFilters
	SortOrder valueobject.SortOrder
	PageSize  int
	Cursor    string
}

// SearchResponse is the raw result from the search backend.
type SearchResponse struct {
	Results    []entity.SearchResult
	TotalHits  int64
	NextCursor string
	Facets     map[string][]entity.Facet
}

// SearchIndexRepository defines the port for full-text and vector search.
type SearchIndexRepository interface {
	// Search performs a full-text search.
	Search(ctx context.Context, params *SearchParams) (*SearchResponse, error)

	// VectorSearch performs a kNN similarity search using a query embedding.
	// Returns the top-k most semantically similar vulnerabilities.
	VectorSearch(ctx context.Context, queryEmbedding []float32, topK int, filters SearchFilters) (*SearchResponse, error)

	// Upsert indexes or updates a vulnerability document.
	Upsert(ctx context.Context, doc *entity.SearchDocument) error

	// MarkWithdrawn sets the is_withdrawn flag on a document.
	MarkWithdrawn(ctx context.Context, vulnID string) error

	// GetByIDs returns multiple documents by ID (for semantic search hydration).
	GetByIDs(ctx context.Context, ids []string) ([]*entity.SearchDocument, error)
}

// SearchCache is the cache interface for hot search results.
type SearchCache interface {
	Get(ctx context.Context, key string) (*SearchResponse, bool)
	Set(ctx context.Context, key string, resp *SearchResponse, ttl time.Duration)
}

// SearchIndexRepo is an alias for SearchIndexRepository for backward compatibility.
type SearchIndexRepo = SearchIndexRepository

// VulnerabilityReader defines read operations for vulnerability data from Firestore.
// Must match the implementation in infra/persistence/firestore/vulnerability_reader.go
type VulnerabilityReader interface {
	GetByID(ctx context.Context, id string) (*entity.VulnerabilityReadModel, error)
	QueryBySemver(ctx context.Context, ecosystem, project, normalizedVersion, cursor string, limit int) ([]*entity.VulnerabilityReadModel, string, error)
	QueryByCommit(ctx context.Context, commitHash, cursor string, limit int) ([]*entity.VulnerabilityReadModel, string, error)
}

// VulnerabilityCache provides caching for vulnerability full records.
type VulnerabilityCache interface {
	Get(ctx context.Context, key string) (*entity.VulnerabilityReadModel, error)
	Set(ctx context.Context, key string, vuln *entity.VulnerabilityReadModel, ttl interface{}) error
}
