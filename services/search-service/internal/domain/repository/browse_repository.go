// Package repository — Browse repository interface for vendor/product data.
// Implementation reads from Redis CPE cache populated by data-service fetcher.
package repository

import "context"

// BrowseRepository defines data access for vendor/product browsing.
// The Redis CPE cache is populated by data-service's RedisCPECacheUpdater.
//
// Key schema:
//
//	"a"          → SET of application vendor names
//	"o"          → SET of OS vendors
//	"h"          → SET of hardware vendors
//	"v:{vendor}" → SET of products for that vendor
type BrowseRepository interface {
	// GetAllVendors returns the sorted list of all known vendors.
	// Reads Redis SUNION of keys "a", "o", "h" (application, os, hardware vendors).
	// Filters empty/wildcard entries ("*", "-").
	GetAllVendors(ctx context.Context) ([]string, error)

	// GetProductsByVendor returns the sorted list of products for a given vendor.
	// Reads Redis SMEMBERS "v:{vendor}" (vendor name is lowercased).
	// Returns error if vendor not found (empty set = vendor doesn't exist).
	GetProductsByVendor(ctx context.Context, vendor string) ([]string, error)
}
