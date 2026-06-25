// Package redis provides Redis-backed caching infrastructure for gateway-service.
// token_cache.go: Caches validated JWT results in Redis to reduce identity-service gRPC RTT.
//
// Design:
//   - All operations are best-effort: Redis errors are silently swallowed.
//   - TokenCache may be nil — the middleware treats nil as "no cache".
//   - Token hashed via SHA-256 — never stores the raw JWT in Redis.
//   - TTL = min(configured TTL, time until token expiry).
//
// Key format: gw:token:<first-16-bytes-of-sha256-hex>
package redis

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// defaultTTL is the default cache TTL for validated tokens.
	defaultTTL = 60 * time.Second
	// keyPrefix namespaces gateway token cache keys.
	keyPrefix = "gw:token:"
)

// CachedPrincipal is the Redis-serializable form of auth.Principal.
// Stores only the fields needed for subsequent request authorization.
type CachedPrincipal struct {
	ID          string   `json:"id"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"perms"`
	ExpiresAt   int64    `json:"exp"` // Unix timestamp; 0 = no expiry tracked
}

// CachePrincipal is the public interface for middleware to construct a principal
// for caching, without importing the auth domain package (avoids circular imports).
// Fields map 1:1 to auth.Principal fields used in JWTVerify.
type CachePrincipal struct {
	ID    string
	Roles []string
}


// TokenCache caches validated JWT principal results in Redis.
// If Redis is unavailable or TokenCache is nil, all operations are no-ops.
type TokenCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewTokenCache creates a new TokenCache.
//   - client: Redis client (required).
//   - ttl: how long to cache a validated principal (0 → defaultTTL of 60s).
func NewTokenCache(client *redis.Client, ttl time.Duration) *TokenCache {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &TokenCache{client: client, ttl: ttl}
}

// hashToken derives a short, opaque cache key from the token.
// We use only the first 16 bytes of the SHA-256 hash to keep keys short.
// The full token is never written to Redis.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%s%x", keyPrefix, h[:16])
}

// Get retrieves a cached CachePrincipal for the given token.
// Returns (CachePrincipal{}, false) on cache miss, expired entry, or any Redis error.
func (c *TokenCache) Get(ctx context.Context, token string) (CachePrincipal, bool) {
	if c == nil || c.client == nil {
		return CachePrincipal{}, false
	}

	key := hashToken(token)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		// MISS or Redis error — both are acceptable, caller will do gRPC
		return CachePrincipal{}, false
	}

	var cached CachedPrincipal
	if err := json.Unmarshal(data, &cached); err != nil {
		return CachePrincipal{}, false
	}

	// Check application-level expiry (belt + suspenders on top of Redis TTL)
	if cached.ExpiresAt > 0 && time.Now().Unix() > cached.ExpiresAt {
		_ = c.client.Del(ctx, key) // best-effort cleanup
		return CachePrincipal{}, false
	}

	return CachePrincipal{
		ID:    cached.ID,
		Roles: cached.Roles,
	}, true
}

// Set stores a validated CachePrincipal in Redis with appropriate TTL.
// tokenExpiry: the JWT expiry time. If zero, only the configured TTL is used.
// All errors are silently ignored — cache is best-effort.
func (c *TokenCache) Set(ctx context.Context, token string, principal *CachePrincipal, tokenExpiry time.Time) {
	if c == nil || c.client == nil || principal == nil {
		return
	}

	cached := CachedPrincipal{
		ID:    principal.ID,
		Roles: principal.Roles,
	}
	if !tokenExpiry.IsZero() {
		cached.ExpiresAt = tokenExpiry.Unix()
	}

	data, err := json.Marshal(cached)
	if err != nil {
		return
	}

	// Effective TTL = min(configured TTL, remaining token lifetime)
	effectiveTTL := c.ttl
	if !tokenExpiry.IsZero() {
		remaining := time.Until(tokenExpiry)
		if remaining > 0 && remaining < effectiveTTL {
			effectiveTTL = remaining
		}
	}

	key := hashToken(token)
	_ = c.client.Set(ctx, key, data, effectiveTTL).Err()
}

// Invalidate removes the cached entry for a given token (e.g. on logout).
func (c *TokenCache) Invalidate(ctx context.Context, token string) {
	if c == nil || c.client == nil {
		return
	}
	_ = c.client.Del(ctx, hashToken(token)).Err()
}
