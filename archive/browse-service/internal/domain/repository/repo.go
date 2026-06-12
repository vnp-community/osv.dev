// Package repository defines browse-service repository interfaces.
package repository

import "context"

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
