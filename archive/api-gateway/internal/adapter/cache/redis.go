// Package cache provides a Redis cache adapter for the API gateway.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenClaims holds cached JWT validation results.
type TokenClaims struct {
	UserID      string   `json:"user_id"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

// RedisCache provides token validation caching for the gateway.
type RedisCache struct {
	client *redis.Client
}

// New creates a RedisCache.
func New(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

// GetTokenValidation retrieves cached token claims.
func (c *RedisCache) GetTokenValidation(ctx context.Context, tokenHash string) (*TokenClaims, bool) {
	key := "gw:token:" + tokenHash[:min(8, len(tokenHash))]
	data, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) || err != nil {
		return nil, false
	}
	var claims TokenClaims
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil, false
	}
	return &claims, true
}

// SetTokenValidation caches token claims.
func (c *RedisCache) SetTokenValidation(ctx context.Context, tokenHash string, claims *TokenClaims, ttl time.Duration) error {
	data, err := json.Marshal(claims)
	if err != nil {
		return err
	}
	key := "gw:token:" + tokenHash[:min(8, len(tokenHash))]
	return c.client.Set(ctx, key, data, ttl).Err()
}

// IncrRateLimit increments the rate limit counter for a key.
// Returns the new count and any error.
func (c *RedisCache) IncrRateLimit(ctx context.Context, key string, window time.Duration) (int64, error) {
	pipe := c.client.TxPipeline()
	incr := pipe.Incr(ctx, "gw:rl:"+key)
	pipe.Expire(ctx, "gw:rl:"+key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("rate limit incr: %w", err)
	}
	return incr.Val(), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
