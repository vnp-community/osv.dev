// Package redis — CVE search cache repository.
package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// CVECacheRepo implements CVECacheRepository using Redis.
type CVECacheRepo struct {
	client *goredis.Client
}

// NewCVECacheRepo creates a new Redis cache repository.
func NewCVECacheRepo(client *goredis.Client) *CVECacheRepo {
	return &CVECacheRepo{client: client}
}

// GetSearchResult retrieves a cached search result.
func (r *CVECacheRepo) GetSearchResult(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := r.client.Get(ctx, "search:"+key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis get search: %w", err)
	}
	return val, true, nil
}

// SetSearchResult stores a search result in Redis with a TTL.
func (r *CVECacheRepo) SetSearchResult(ctx context.Context, key string, data []byte, ttlSeconds int) error {
	return r.client.Set(ctx, "search:"+key, data, time.Duration(ttlSeconds)*time.Second).Err()
}

// GetCVE retrieves a cached CVE by ID.
func (r *CVECacheRepo) GetCVE(ctx context.Context, id string) ([]byte, bool, error) {
	val, err := r.client.Get(ctx, "cve:"+id).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis get cve: %w", err)
	}
	return val, true, nil
}

// SetCVE stores a CVE in Redis with a TTL.
func (r *CVECacheRepo) SetCVE(ctx context.Context, id string, data []byte, ttlSeconds int) error {
	return r.client.Set(ctx, "cve:"+id, data, time.Duration(ttlSeconds)*time.Second).Err()
}

// InvalidateSearch clears all search cache entries matching a pattern.
func (r *CVECacheRepo) InvalidateSearch(ctx context.Context) error {
	var cursor uint64
	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, "search:*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			r.client.Del(ctx, keys...)
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}
