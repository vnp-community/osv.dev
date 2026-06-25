// Package entity — Browse domain entities for vendor/product catalog.
// Populated from Redis CPE cache keys set by data-service fetcher.
//
// Redis key schema (populated by data-service fetcher/redis_cpe_cache.go):
//   "a"          → SET of application vendor names
//   "o"          → SET of OS vendors
//   "h"          → SET of hardware vendors
//   "v:{vendor}" → SET of products for that vendor
package entity

import "time"

// VendorCatalog is the response type for listing all vendors.
// Data is sourced from Redis CPE cache (SUNION of "a", "o", "h" keys).
type VendorCatalog struct {
	Vendors  []string  `json:"vendors"`
	Total    int       `json:"total"`
	CachedAt time.Time `json:"cached_at"`
}

// ProductCatalog is the response type for listing products of a vendor.
// Data is sourced from Redis CPE cache (SMEMBERS "v:{vendor}").
type ProductCatalog struct {
	Vendor   string    `json:"vendor"`
	Products []string  `json:"products"`
	Total    int       `json:"total"`
	CachedAt time.Time `json:"cached_at"`
}
