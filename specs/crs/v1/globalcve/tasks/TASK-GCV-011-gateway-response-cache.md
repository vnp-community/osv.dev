# TASK-GCV-011 — Response Cache + Tiered Rate Limiter (gateway-service)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-011 |
| **Service** | `gateway-service` |
| **CR** | CR-GCV-008 |
| **Phase** | 1 — Core Pipeline |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-GCV-009 |

## Context

Thêm **Redis-backed response cache** cho GET requests và **tiered rate limiter** (public: 60/min, API key: 300/min, semantic: 10/min). Cả hai được implement như HTTP middleware wrapping proxy calls.

## Reference

- Solution: [SOL-GCV-008](../solutions/SOL-GCV-008-api-gateway-enhancement.md) §2.4, §2.5
- CR: [CR-GCV-008](../CR-GCV-008-api-gateway-enhancement.md) §5, §6

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/cache/response_cache.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/ratelimit/tiered_limiter.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/proxy/http_proxy.go
        (thêm ProxyWithCache wrapper)
```

**Đọc trước**: `gateway-service/internal/ratelimit/` (nếu đã tồn tại) để hiểu cấu trúc, chỉ extend thay vì replace.

## Implementation Spec

### cache/response_cache.go

```go
// Package cache — Redis-backed HTTP response cache.
package cache

import (
    "bytes"
    "context"
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/redis/go-redis/v9"
)

// ResponseRecorder captures response body + status for caching.
type ResponseRecorder struct {
    http.ResponseWriter
    statusCode int
    body       bytes.Buffer
}

func (r *ResponseRecorder) WriteHeader(code int) {
    r.statusCode = code
    r.ResponseWriter.WriteHeader(code)
}

func (r *ResponseRecorder) Write(b []byte) (int, error) {
    r.body.Write(b)
    return r.ResponseWriter.Write(b)
}

func (r *ResponseRecorder) StatusCode() int {
    if r.statusCode == 0 {
        return http.StatusOK
    }
    return r.statusCode
}

// Middleware returns a caching HTTP middleware.
// Only caches GET requests with 2xx responses.
// Cache key: "gw:cache:" + request URI
func Middleware(redis *redis.Client, ttl time.Duration) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Only cache GET requests
            if r.Method != http.MethodGet {
                next.ServeHTTP(w, r)
                return
            }

            cacheKey := cacheKeyFor(r)

            // Cache HIT
            if cached, err := redis.Get(r.Context(), cacheKey).Bytes(); err == nil {
                w.Header().Set("Content-Type", "application/json")
                w.Header().Set("X-Cache", "HIT")
                w.WriteHeader(http.StatusOK)
                w.Write(cached) //nolint:errcheck
                return
            }

            // Cache MISS — record response
            rec := &ResponseRecorder{ResponseWriter: w}
            w.Header().Set("X-Cache", "MISS")
            next.ServeHTTP(rec, r)

            // Cache 2xx responses
            if rec.StatusCode() >= 200 && rec.StatusCode() < 300 && rec.body.Len() > 0 {
                redis.Set(r.Context(), cacheKey, rec.body.Bytes(), ttl) //nolint:errcheck
            }
        })
    }
}

func cacheKeyFor(r *http.Request) string {
    return "gw:cache:" + r.URL.RequestURI()
}
```

### ratelimit/tiered_limiter.go

```go
// Package ratelimit — Tiered rate limiter using Redis sliding window.
package ratelimit

import (
    "context"
    "fmt"
    "net/http"
    "strconv"
    "time"

    "github.com/redis/go-redis/v9"
)

// TierConfig defines rate limits per tier.
type TierConfig struct {
    PublicPerMin   int // default: 60 req/min
    APIKeyPerMin   int // default: 300 req/min
    SemanticPerMin int // default: 10 req/min (expensive AI call)
    SyncPerMin     int // default: 5 req/min (admin sync endpoints)
}

// DefaultTierConfig returns recommended tier defaults.
func DefaultTierConfig() TierConfig {
    return TierConfig{
        PublicPerMin:   60,
        APIKeyPerMin:   300,
        SemanticPerMin: 10,
        SyncPerMin:     5,
    }
}

// TieredLimiter applies per-tier rate limiting using Redis 1-minute sliding window.
type TieredLimiter struct {
    redis  *redis.Client
    config TierConfig
}

func NewTieredLimiter(redis *redis.Client, config TierConfig) *TieredLimiter {
    return &TieredLimiter{redis: redis, config: config}
}

// Allow checks if the request is within rate limit.
// Returns (allowed bool, remaining int).
// identifier: IP address or owner_id.
// tier: "public" | "api_key" | "semantic" | "sync"
func (l *TieredLimiter) Allow(ctx context.Context, identifier, tier string) (bool, int) {
    limit := l.limitForTier(tier)
    window := time.Now().Unix() / 60  // 1-minute window
    key := fmt.Sprintf("rl:%s:%s:%d", tier, identifier, window)

    count, err := l.redis.Incr(ctx, key).Result()
    if err != nil {
        // Redis down → fail open (allow request)
        return true, limit
    }

    if count == 1 {
        l.redis.Expire(ctx, key, 70*time.Second) //nolint:errcheck
    }

    remaining := limit - int(count)
    if remaining < 0 {
        remaining = 0
    }
    return count <= int64(limit), remaining
}

func (l *TieredLimiter) limitForTier(tier string) int {
    switch tier {
    case "api_key":
        return l.config.APIKeyPerMin
    case "semantic":
        return l.config.SemanticPerMin
    case "sync":
        return l.config.SyncPerMin
    default: // "public"
        return l.config.PublicPerMin
    }
}

// Middleware returns HTTP middleware applying tiered rate limiting.
func Middleware(limiter *TieredLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            tier := detectTier(r)
            identifier := detectIdentifier(r)

            allowed, remaining := limiter.Allow(r.Context(), identifier, tier)

            // Set standard rate limit headers
            limit := limiter.limitForTier(tier)
            w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
            w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

            if !allowed {
                w.Header().Set("Content-Type", "application/json")
                w.Header().Set("Retry-After", "60")
                w.WriteHeader(http.StatusTooManyRequests)
                w.Write([]byte(`{"error":"Rate limit exceeded. Please retry after 60 seconds."}`))
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

func detectTier(r *http.Request) string {
    // Check if it's a semantic search request
    if r.URL.Path == "/api/v2/cves/search/semantic" {
        return "semantic"
    }
    // Check if it's an admin sync request
    if strings.HasPrefix(r.URL.Path, "/api/v2/sync") {
        return "sync"
    }
    // Check if API key auth is present (set by auth middleware)
    if getAuthType(r.Context()) == "api_key" {
        return "api_key"
    }
    return "public"
}

func detectIdentifier(r *http.Request) string {
    // If authenticated, use owner_id
    if ownerID := getOwnerID(r.Context()); ownerID != "" {
        return ownerID
    }
    // Otherwise use IP
    return r.RemoteAddr
}
```

## Acceptance Criteria

- [x] `GET /api/v2/cves/aggregations` → second request hits `X-Cache: HIT`
- [x] `POST /api/v2/cves/search` (không phải GET) → không cache
- [x] Cache MISS → `X-Cache: MISS` header
- [x] Rate limit exceeded (> 60 req/min, public) → 429 với `{"error":"Rate limit exceeded..."}`
- [x] 429 response có `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `Retry-After` headers
- [x] API key user: 300 req/min limit (không bị limit sớm như public)
- [x] `/api/v2/cves/search/semantic` → tier `semantic` (10/min)
- [x] Redis down → rate limit fail open (request allowed), không panic
- [x] `go build ./...` pass không lỗi
