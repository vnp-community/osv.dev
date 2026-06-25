package redis

import (
    "context"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

const (
    jtiPrefix    = "jti:"
    defaultTTL   = 15 * time.Minute // Access token lifetime
    revokedValue = "revoked"
)

// JTICache manages JWT Token IDs in Redis for fast invalidation checks.
// Hot path: Get() must be sub-millisecond.
type JTICache struct {
    client *redis.Client
}

// NewJTICache creates a JTICache backed by the provided redis client.
func NewJTICache(client *redis.Client) *JTICache {
    return &JTICache{client: client}
}

// key returns the Redis key for a JTI.
func (c *JTICache) key(jti string) string {
    return jtiPrefix + jti
}

// Store marks a JTI as active until ttl. Called on login / token issue.
func (c *JTICache) Store(ctx context.Context, jti string, ttl time.Duration) error {
    if ttl <= 0 {
        ttl = defaultTTL
    }
    return c.client.Set(ctx, c.key(jti), "active", ttl).Err()
}

// Revoke blacklists a JTI immediately (logout / rotation).
// Keeps the key alive until original TTL to prevent re-use after expiry.
func (c *JTICache) Revoke(ctx context.Context, jti string, remainingTTL time.Duration) error {
    if remainingTTL <= 0 {
        remainingTTL = defaultTTL
    }
    return c.client.Set(ctx, c.key(jti), revokedValue, remainingTTL).Err()
}

// IsRevoked returns true if the JTI is revoked or not present.
// Hot path: single Redis GET.
func (c *JTICache) IsRevoked(ctx context.Context, jti string) (bool, error) {
    val, err := c.client.Get(ctx, c.key(jti)).Result()
    if err == redis.Nil {
        // Key not found: treat as revoked (token unknown to cache = invalid)
        return true, nil
    }
    if err != nil {
        return false, fmt.Errorf("jti cache get: %w", err)
    }
    return val == revokedValue, nil
}

// RevokeAll revokes all JTIs for a user (used on password change / account lock).
// NOTE: Requires JTI keys to be stored in a user-indexed set.
func (c *JTICache) RevokeAllForUser(ctx context.Context, userID string, jtis []string) error {
    pipe := c.client.Pipeline()
    for _, jti := range jtis {
        pipe.Set(ctx, c.key(jti), revokedValue, defaultTTL)
    }
    _, err := pipe.Exec(ctx)
    return err
}

// Ping checks Redis connectivity.
func (c *JTICache) Ping(ctx context.Context) error {
    return c.client.Ping(ctx).Err()
}

// TTL returns remaining TTL for a JTI key.
func (c *JTICache) TTL(ctx context.Context, jti string) (time.Duration, error) {
    d, err := c.client.TTL(ctx, c.key(jti)).Result()
    if err != nil {
        return 0, fmt.Errorf("jti cache ttl: %w", err)
    }
    return d, nil
}
