// Package redis implements SearchCache backed by Redis.
package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/osv/search/internal/domain/entity"
	"github.com/osv/search/internal/domain/repository"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const defaultSearchTTL = 30 * time.Second
const cacheKeyPrefix = "search:"

// SearchCache implements repository.SearchCache using Redis.
type SearchCache struct {
	rc  *redis.Client
	log zerolog.Logger
}

// NewSearchCache creates a Redis-backed search cache.
func NewSearchCache(rc *redis.Client, log zerolog.Logger) *SearchCache {
	return &SearchCache{rc: rc, log: log}
}

// Get retrieves a cached search response.
func (c *SearchCache) Get(ctx context.Context, key string) (*repository.SearchResponse, bool) {
	data, err := c.rc.Get(ctx, cacheKeyPrefix+key).Bytes()
	if err != nil {
		return nil, false
	}

	var resp repository.SearchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		c.log.Warn().Err(err).Msg("cache unmarshal failed")
		return nil, false
	}
	return &resp, true
}

// Set stores a search response in Redis.
func (c *SearchCache) Set(ctx context.Context, key string, resp *repository.SearchResponse, ttl time.Duration) {
	if ttl <= 0 {
		ttl = defaultSearchTTL
	}

	data, err := json.Marshal(resp)
	if err != nil {
		c.log.Warn().Err(err).Msg("cache marshal failed")
		return
	}
	c.rc.Set(ctx, cacheKeyPrefix+key, data, ttl) //nolint:errcheck
}

// Ensure SearchCache satisfies repository interface at compile time.
var _ repository.SearchCache = (*SearchCache)(nil)

// VulnerabilityCache interface compatibility (keep both caches in same package).
type vulnResult struct {
	Data entity.SearchDocument `json:"data"`
}
