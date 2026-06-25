// Package ratelimit — Tiered rate limiter using Redis sliding window.
package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/osv/gateway-service/internal/auth"
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
	redisClient *redis.Client
	config      TierConfig
}

// NewTieredLimiter creates a new limiter.
func NewTieredLimiter(redisClient *redis.Client, config TierConfig) *TieredLimiter {
	return &TieredLimiter{redisClient: redisClient, config: config}
}

// Allow checks if the request is within rate limit.
// Returns (allowed bool, remaining int).
func (l *TieredLimiter) Allow(ctx context.Context, identifier, tier string) (bool, int) {
	limit := l.limitForTier(tier)
	window := time.Now().Unix() / 60 // 1-minute window
	key := fmt.Sprintf("rl:%s:%s:%d", tier, identifier, window)

	count, err := l.redisClient.Incr(ctx, key).Result()
	if err != nil {
		// Redis down → fail open (allow request)
		return true, limit
	}

	if count == 1 {
		l.redisClient.Expire(ctx, key, 70*time.Second) //nolint:errcheck
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
	if r.URL.Path == "/api/v2/cves/search/semantic" {
		return "semantic"
	}
	if strings.HasPrefix(r.URL.Path, "/api/v2/sync") {
		return "sync"
	}
	
	// Check auth context
	if claims, ok := r.Context().Value(auth.CtxKeyAPIKeyClaims).(*auth.APIKeyClaims); ok && claims != nil {
		return "api_key"
	}
	if r.Header.Get("X-User-ID") != "" {
		return "api_key" // Use API key tier for logged in JWT users too
	}

	return "public"
}

func detectIdentifier(r *http.Request) string {
	if claims, ok := r.Context().Value(auth.CtxKeyAPIKeyClaims).(*auth.APIKeyClaims); ok && claims != nil {
		return claims.UserID
	}
	if uid := r.Header.Get("X-User-ID"); uid != "" {
		return uid
	}
	return r.RemoteAddr
}
