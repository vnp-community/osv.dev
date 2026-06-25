# S1-GW-03 — Thêm Redis Token Cache (gateway-service)


## ✅ Execution Status: COMPLETED
- **Executed**: 2026-06-13
- **Result**: `go build` + `go vet` PASSED
- **Files Created**: `internal/infra/redis/token_cache.go`
- **Files Modified**: `internal/auth/osv_middleware.go`
- **Changes**:
  - `TokenCache`: SHA-256 hashed keys, configurable TTL, nil-safe
  - `CachePrincipal`: public type to avoid circular import auth↔redis
  - `JWTVerify()`: variadic cache param (backward-compatible), cache check before crypto
- **Security**: Raw JWT never stored in Redis (only first 16 bytes of SHA-256 hash)

## Metadata
- **Task ID**: S1-GW-03
- **Service**: gateway-service
- **Sprint**: 1 (P0)
- **Ước tính**: 2 giờ
- **Dependencies**: S1-GW-01 (identity_client)
- **Spec nguồn**: `specs/develop/08_gateway-service-upgrade.md` § "Thêm: Redis Token Cache"

## Context

```bash
cat services/gateway-service/internal/auth/osv_middleware.go
cat services/gateway-service/go.mod | grep redis
# Nếu redis chưa có: go get github.com/redis/go-redis/v9
```

## Goal

Tạo `infra/redis/token_cache.go` và inject vào `auth/osv_middleware.go` như một layer optional (nếu Redis không available, middleware hoạt động bình thường qua gRPC).

## Files to Create

### File: `services/gateway-service/internal/infra/redis/token_cache.go`

```go
package redis

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/osv/gateway-service/internal/domain/auth"
)

const (
	defaultTTL    = 60 * time.Second
	keyPrefix     = "gw:token:"
	hashBytesLen  = 32 // sha256
)

// CachedPrincipal is the Redis-serializable form of auth.Principal.
type CachedPrincipal struct {
	UserID      string   `json:"user_id"`
	Role        string   `json:"role"`
	Permissions []string `json:"perms"`
	ExpiresAt   int64    `json:"exp"`
}

// TokenCache caches validated JWT results in Redis to reduce identity-service gRPC calls.
// If Redis is unavailable, all operations are no-ops (graceful degradation).
type TokenCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewTokenCache creates a new TokenCache.
// ttl: how long to cache a validated token (default 60s if zero).
func NewTokenCache(client *redis.Client, ttl time.Duration) *TokenCache {
	if ttl == 0 {
		ttl = defaultTTL
	}
	return &TokenCache{client: client, ttl: ttl}
}

// hashToken creates a short key from the first 32 bytes of a JWT.
// We never store the full token to avoid security exposure.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%s%x", keyPrefix, h[:16])
}

// Get retrieves a cached principal. Returns (nil, false) on miss or Redis error.
func (c *TokenCache) Get(ctx context.Context, token string) (*auth.Principal, bool) {
	if c == nil || c.client == nil {
		return nil, false
	}

	key := hashToken(token)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		// Cache miss or Redis error — both are non-fatal
		return nil, false
	}

	var cached CachedPrincipal
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, false
	}

	// Check token expiry
	if cached.ExpiresAt > 0 && time.Now().Unix() > cached.ExpiresAt {
		_ = c.client.Del(ctx, key)
		return nil, false
	}

	return &auth.Principal{
		UserID:      cached.UserID,
		Role:        cached.Role,
		Permissions: cached.Permissions,
	}, true
}

// Set stores a validated principal in Redis.
// Silently ignores errors (Redis is best-effort).
func (c *TokenCache) Set(ctx context.Context, token string, principal *auth.Principal, tokenExpiry time.Time) {
	if c == nil || c.client == nil {
		return
	}

	cached := CachedPrincipal{
		UserID:      principal.UserID,
		Role:        principal.Role,
		Permissions: principal.Permissions,
		ExpiresAt:   tokenExpiry.Unix(),
	}

	data, err := json.Marshal(cached)
	if err != nil {
		return
	}

	// Use min(c.ttl, time until token expiry) as Redis TTL
	ttl := c.ttl
	if !tokenExpiry.IsZero() {
		remaining := time.Until(tokenExpiry)
		if remaining > 0 && remaining < ttl {
			ttl = remaining
		}
	}

	key := hashToken(token)
	_ = c.client.Set(ctx, key, data, ttl).Err()
}

// Invalidate removes a cached token (call on logout/token revocation).
func (c *TokenCache) Invalidate(ctx context.Context, token string) {
	if c == nil || c.client == nil {
		return
	}
	_ = c.client.Del(ctx, hashToken(token)).Err()
}

// InvalidateUser removes all cached tokens for a user.
// Note: This requires user_id indexed keys — use pattern scan (expensive).
// Alternative: use short TTL and let tokens expire naturally.
func (c *TokenCache) InvalidateUser(ctx context.Context, userID string) {
	// Pattern-based delete is expensive; use TTL expiry instead for now.
	// Implementation left as future optimization.
}
```

## Files to Extend

### Extend: `services/gateway-service/internal/auth/osv_middleware.go`

Đọc file trước, sau đó:

1. **Thêm import** (nếu chưa có):
```go
redis_infra "github.com/osv/gateway-service/internal/infra/redis"
```

2. **Thêm field vào middleware struct** (giữ existing fields):
```go
tokenCache *redis_infra.TokenCache  // optional, can be nil
```

3. **Thêm constructor parameter** (hoặc setter):
```go
// Nếu có constructor function, thêm optional param:
func (m *OSVMiddleware) WithTokenCache(cache *redis_infra.TokenCache) *OSVMiddleware {
    m.tokenCache = cache
    return m
}
```

4. **Thêm cache check vào middleware handler** (thêm 10 dòng trước phần gRPC call):
```go
// Trong http.HandlerFunc body, SAU khi extract token, TRƯỚC khi gọi gRPC:

// Check cache first (best-effort)
if m.tokenCache != nil {
    if principal, ok := m.tokenCache.Get(r.Context(), token); ok {
        ctx := context.WithValue(r.Context(), contextKeyPrincipal, principal)
        next.ServeHTTP(w, r.WithContext(ctx))
        return
    }
}

// [existing gRPC validation code unchanged]
// ...
// After successful validation, cache the result:
if m.tokenCache != nil && err == nil {
    m.tokenCache.Set(r.Context(), token, principal, tokenExpiry)
}
```

### Extend: `services/gateway-service/cmd/server/main.go`

```go
// Thêm Redis client init (sau các existing inits):
redisClient := redis.NewClient(&redis.Options{
    Addr:     cfg.Redis.Addr,         // e.g., "localhost:6379"
    Password: cfg.Redis.Password,
    DB:       cfg.Redis.DB,           // default 0
})

tokenCache := redis_infra.NewTokenCache(redisClient, 60*time.Second)

// Wire vào OSVMiddleware:
osvMiddleware := auth.NewOSVMiddleware(...).WithTokenCache(tokenCache)
```

## Verification

```bash
cd services/gateway-service && go build ./...

# Test cache hit/miss:
# 1. Send request with valid token → gRPC call to identity-service
# 2. Send same request again within 60s → should NOT hit identity-service (cache hit)
# Check logs for "cache hit" vs "gRPC validate" messages
```

## Notes

- `TokenCache` phải hoàn toàn optional — nếu `nil` thì middleware hoạt động như cũ
- Không dùng SCAN trên Redis production (quá chậm) — dùng TTL expiry thay thế
- Redis key format: `gw:token:<sha256-prefix>` (không bao giờ lưu full JWT)
