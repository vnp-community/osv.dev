// Package redis — Redis implementation of BrowseRepository.
// Reads from CPE cache populated by data-service RedisCPECacheUpdater.
//
// Redis key schema (set by data-service internal/fetcher/redis_cpe_cache.go):
//
//	"a"          → SET of application vendor names
//	"o"          → SET of OS vendors
//	"h"          → SET of hardware vendors
//	"v:{vendor}" → SET of products for that vendor
package redis

import (
	"context"
	"fmt"
	"sort"
	"strings"

	goredis "github.com/redis/go-redis/v9"

	"github.com/osv/search-service/internal/domain/repository"
)

// RedisBrowseRepository implements BrowseRepository using Redis.
// Reads from the CPE cache keys populated by data-service.
type RedisBrowseRepository struct {
	client *goredis.Client
}

// NewRedisBrowseRepository creates a Redis-backed BrowseRepository.
func NewRedisBrowseRepository(client *goredis.Client) repository.BrowseRepository {
	return &RedisBrowseRepository{client: client}
}

// GetAllVendors returns all known vendors from Redis CPE cache.
// Uses SUNION of "a" (application), "o" (os), "h" (hardware) sets.
// Filters empty/wildcard entries ("*", "-") and returns sorted list.
// Mirrors Python: CPERedisBrowser.getBrowse() in cve-search.
func (r *RedisBrowseRepository) GetAllVendors(ctx context.Context) ([]string, error) {
	// SUNION combines all vendor type sets into one
	results, err := r.client.SUnion(ctx, "a", "o", "h").Result()
	if err != nil {
		return nil, fmt.Errorf("redis SUNION vendors: %w", err)
	}

	// Filter out empty/wildcard entries
	filtered := make([]string, 0, len(results))
	for _, v := range results {
		if v != "" && v != "*" && v != "-" {
			filtered = append(filtered, v)
		}
	}

	sort.Strings(filtered)
	return filtered, nil
}

// GetProductsByVendor returns the sorted list of products for a vendor.
// Uses Redis SMEMBERS "v:{vendor}" (vendor name is lowercased).
// Returns error if vendor not found (empty set = vendor doesn't exist).
// Mirrors Python: CPERedisBrowser.getVendorProducts() in cve-search.
func (r *RedisBrowseRepository) GetProductsByVendor(ctx context.Context, vendor string) ([]string, error) {
	// Normalize: lowercase, trim whitespace
	vendor = strings.ToLower(strings.TrimSpace(vendor))
	if vendor == "" {
		return nil, fmt.Errorf("vendor name is required")
	}

	key := "v:" + vendor
	results, err := r.client.SMembers(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis SMEMBERS %s: %w", key, err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("vendor '%s' not found in CPE database", vendor)
	}

	// Filter wildcard/empty entries and sort
	filtered := make([]string, 0, len(results))
	for _, p := range results {
		if p != "" && p != "*" && p != "-" {
			filtered = append(filtered, p)
		}
	}
	sort.Strings(filtered)
	return filtered, nil
}
