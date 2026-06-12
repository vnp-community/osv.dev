// Package redis implements the CVE search cache using go-redis/v9.
package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	domainerrors "github.com/osv/search-service/internal/domain/errors"
	"github.com/osv/search-service/internal/domain/repository"
)

type cveCache struct {
	client *redis.Client
}

// NewCVECache creates a Redis-backed CVECacheRepository.
func NewCVECache(client *redis.Client) repository.CVECacheRepository {
	return &cveCache{client: client}
}

func (c *cveCache) GetSearchResult(ctx context.Context, key string) ([]byte, error) {
	data, err := c.client.Get(ctx, "cve:search:"+key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, domainerrors.ErrCacheMiss
	}
	return data, err
}

func (c *cveCache) SetSearchResult(ctx context.Context, key string, data []byte, ttlSec int) error {
	return c.client.Set(ctx, "cve:search:"+key, data, time.Duration(ttlSec)*time.Second).Err()
}

func (c *cveCache) InvalidatePattern(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, next, err := c.client.Scan(ctx, cursor, "cve:search:"+pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("cache scan: %w", err)
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("cache del: %w", err)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}
