// Package repository defines the CVE repository interfaces (ports).
package repository

import (
	"context"

	"github.com/globalcve/mono/internal/cvesearch/domain/entity"
)

// CVERepository defines read operations for the CVE search service.
type CVERepository interface {
	// Search queries CVEs with filtering, sorting and pagination.
	Search(ctx context.Context, filter *entity.SearchFilter) ([]*entity.CVE, int64, error)

	// FindByID retrieves a single CVE by its ID.
	FindByID(ctx context.Context, id string) (*entity.CVE, error)

	// Count returns the total number of CVEs in the database.
	Count(ctx context.Context) (int64, error)

	// SearchSemantic performs pgvector cosine similarity search.
	SearchSemantic(ctx context.Context, embedding []float32, limit int, threshold float64) ([]*entity.CVE, error)
}

// CVESearchRepository defines full-text search operations (OpenSearch backend).
type CVESearchRepository interface {
	// Search performs full-text search against the OpenSearch index.
	Search(ctx context.Context, query string, filter *entity.SearchFilter) ([]*entity.CVE, int64, error)
}

// CVECacheRepository defines cache operations for CVE search results.
type CVECacheRepository interface {
	GetSearchResult(ctx context.Context, key string) ([]byte, bool, error)
	SetSearchResult(ctx context.Context, key string, data []byte, ttlSeconds int) error
	GetCVE(ctx context.Context, id string) ([]byte, bool, error)
	SetCVE(ctx context.Context, id string, data []byte, ttlSeconds int) error
}
