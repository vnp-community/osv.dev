// Package cache provides Redis-backed caching for the auth service.
// Handles brute force protection, JWT blacklisting, and session caching.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Brute force: max 5 failed attempts in 15-minute window.
	maxLoginAttempts = 5
	loginWindowTTL   = 15 * time.Minute

	// JWT JTI blacklist: key TTL equals remaining token lifetime.
	jtiBlacklistPrefix = "auth:revoked:"

	// Login attempt tracking.
	bfPrefix = "auth:bf:"
)

// TokenCache wraps Redis for auth-specific caching operations.
type TokenCache struct {
	rdb *redis.Client
}

// NewTokenCache creates a new TokenCache backed by the given Redis client.
func NewTokenCache(rdb *redis.Client) *TokenCache {
	return &TokenCache{rdb: rdb}
}

// IncrLoginAttempt increments the failed login counter for email.
// Returns the new count. The key expires after loginWindowTTL from first failure.
func (c *TokenCache) IncrLoginAttempt(ctx context.Context, email string) (int64, error) {
	key := bfPrefix + email
	pipe := c.rdb.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, loginWindowTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("incr login attempt: %w", err)
	}
	return incrCmd.Val(), nil
}

// GetLoginAttempts returns the current failed login count for email.
// Returns 0 if no failures have been recorded.
func (c *TokenCache) GetLoginAttempts(ctx context.Context, email string) (int64, error) {
	key := bfPrefix + email
	val, err := c.rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// ResetLoginAttempts clears the failed login counter (on successful login).
func (c *TokenCache) ResetLoginAttempts(ctx context.Context, email string) error {
	return c.rdb.Del(ctx, bfPrefix+email).Err()
}

// IsLockedOut returns true if the email has exceeded the maximum failed attempts.
func (c *TokenCache) IsLockedOut(ctx context.Context, email string) (bool, error) {
	count, err := c.GetLoginAttempts(ctx, email)
	if err != nil {
		return false, err
	}
	return count >= maxLoginAttempts, nil
}

// RevokeJTI adds a JWT ID to the blacklist with the given TTL.
// TTL should be set to the remaining lifetime of the access token.
func (c *TokenCache) RevokeJTI(ctx context.Context, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil // already expired
	}
	return c.rdb.Set(ctx, jtiBlacklistPrefix+jti, "1", ttl).Err()
}

// IsJTIRevoked returns true if the given JTI is in the blacklist.
func (c *TokenCache) IsJTIRevoked(ctx context.Context, jti string) (bool, error) {
	val, err := c.rdb.Exists(ctx, jtiBlacklistPrefix+jti).Result()
	if err != nil {
		return false, fmt.Errorf("check JTI revoked: %w", err)
	}
	return val > 0, nil
}
