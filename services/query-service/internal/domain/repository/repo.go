// Package repository defines browse-service repository interfaces.
package repository

import (
	"context"
	"github.com/osv/query-service/internal/domain/entity"
	"time"
)

// CacheRepository provides Redis-backed vendor/product caching (L1).
type CacheRepository interface {
	// GetVendors returns the set of vendors for a CPE type ("a", "o", "h").
	// Returns nil slice if cache miss.
	GetVendors(ctx context.Context, cpeType string) ([]string, error)

	// GetProducts returns the set of products for a vendor.
	// Returns nil slice if cache miss.
	GetProducts(ctx context.Context, vendor string) ([]string, error)

	// SetVendors caches the vendors for a CPE type.
	SetVendors(ctx context.Context, cpeType string, vendors []string) error

	// SetProducts caches the products for a vendor.
	SetProducts(ctx context.Context, vendor string, products []string) error
}

// CPERepository provides MongoDB-backed vendor/product data (L2).
type CPERepository interface {
	// ListVendors returns all distinct vendors for a CPE type.
	ListVendors(ctx context.Context, cpeType string) ([]string, error)

	// ListProducts returns all distinct products for a vendor.
	ListProducts(ctx context.Context, vendor string) ([]string, error)

	// SearchVendors returns vendors whose name matches the query pattern.
	SearchVendors(ctx context.Context, query string) ([]string, error)
}

// VulnerabilityReader provides read-only access to vulnerability data.
type VulnerabilityReader interface {
	// GetByID retrieves a vulnerability by its ID.
	// Returns pkg/errors.ErrNotFound if not found.
	GetByID(ctx context.Context, id string) (*entity.VulnerabilityReadModel, error)

	// QueryBySemver finds vulnerabilities affecting the given package version.
	// Returns the matching vulnerabilities and a Firestore cursor for pagination.
	QueryBySemver(ctx context.Context, ecosystem, project, normalizedVersion, cursor string, limit int) ([]*entity.VulnerabilityReadModel, string, error)

	// QueryByCommit finds vulnerabilities affecting the given git commit hash.
	QueryByCommit(ctx context.Context, commitHash, cursor string, limit int) ([]*entity.VulnerabilityReadModel, string, error)
}

// VulnerabilityCache provides a cache layer for vulnerability data.
type VulnerabilityCache interface {
	// Get retrieves a cached result. Returns (nil, nil) on cache miss.
	Get(ctx context.Context, key string) (*entity.VulnerabilityReadModel, error)

	// GetMany retrieves multiple cached results.
	GetMany(ctx context.Context, keys []string) (map[string]*entity.VulnerabilityReadModel, error)

	// Set stores a result in the cache with the given TTL.
	Set(ctx context.Context, key string, v *entity.VulnerabilityReadModel, ttl time.Duration) error

	// Delete removes a cached result.
	Delete(ctx context.Context, key string) error
}
