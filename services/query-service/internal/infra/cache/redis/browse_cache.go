// Package redis provides Redis L1 cache for browse-service vendor/product data.
// Redis key schema (mirrors Python CPERedisBrowser):
//   "a"          → SMEMBERS: application vendors
//   "o"          → SMEMBERS: OS vendors
//   "h"          → SMEMBERS: hardware vendors
//   "v:{vendor}" → SMEMBERS: products for vendor
package redis

import (
	"context"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
)

const cacheTTL = 24 * time.Hour

// BrowseCache implements repository.CacheRepository using Redis Sets.
type BrowseCache struct {
	client *redis.Client
}

// NewBrowseCache creates a Redis-backed browse cache.
func NewBrowseCache(client *redis.Client) *BrowseCache {
	return &BrowseCache{client: client}
}

// GetVendors returns cached vendors for a CPE type ("a", "o", "h").
// Returns nil if the key does not exist (cache miss).
func (c *BrowseCache) GetVendors(ctx context.Context, cpeType string) ([]string, error) {
	members, err := c.client.SMembers(ctx, cpeType).Result()
	if err == redis.Nil || len(members) == 0 {
		return nil, nil // cache miss
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(members)
	return members, nil
}

// GetProducts returns cached products for a vendor.
// Returns nil if the key does not exist (cache miss).
func (c *BrowseCache) GetProducts(ctx context.Context, vendor string) ([]string, error) {
	members, err := c.client.SMembers(ctx, "v:"+vendor).Result()
	if err == redis.Nil || len(members) == 0 {
		return nil, nil // cache miss
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(members)
	return members, nil
}

// SetVendors populates the vendor cache for a CPE type.
func (c *BrowseCache) SetVendors(ctx context.Context, cpeType string, vendors []string) error {
	if len(vendors) == 0 {
		return nil
	}
	members := make([]interface{}, len(vendors))
	for i, v := range vendors {
		members[i] = v
	}
	pipe := c.client.Pipeline()
	pipe.SAdd(ctx, cpeType, members...)
	pipe.Expire(ctx, cpeType, cacheTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// SetProducts populates the product cache for a vendor.
func (c *BrowseCache) SetProducts(ctx context.Context, vendor string, products []string) error {
	if len(products) == 0 {
		return nil
	}
	members := make([]interface{}, len(products))
	for i, p := range products {
		members[i] = p
	}
	key := "v:" + vendor
	pipe := c.client.Pipeline()
	pipe.SAdd(ctx, key, members...)
	pipe.Expire(ctx, key, cacheTTL)
	_, err := pipe.Exec(ctx)
	return err
}
