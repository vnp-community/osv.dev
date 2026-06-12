// Package ratelimit implements per-API-key rate limiting for the API gateway.
// TASK-08-03: Token bucket rate limiting with configurable quotas per API key.
package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Quota defines rate limiting parameters for an API key or tier.
type Quota struct {
	// RequestsPerMinute is the steady-state rate limit.
	RequestsPerMinute int

	// BurstSize is the maximum burst above the steady rate.
	// Defaults to RequestsPerMinute if zero.
	BurstSize int

	// DailyLimit is the maximum requests per 24-hour window (0 = unlimited).
	DailyLimit int
}

// TierQuotas maps tier names to quota configurations.
var TierQuotas = map[string]Quota{
	"free":       {RequestsPerMinute: 10, BurstSize: 20, DailyLimit: 1000},
	"developer":  {RequestsPerMinute: 60, BurstSize: 120, DailyLimit: 10000},
	"enterprise": {RequestsPerMinute: 600, BurstSize: 1200, DailyLimit: 0},
	"internal":   {RequestsPerMinute: 6000, BurstSize: 6000, DailyLimit: 0},
}

// RateLimitResult is the outcome of a rate limit check.
type RateLimitResult struct {
	Allowed        bool
	Remaining      int
	ResetAt        time.Time
	RetryAfter     time.Duration
	DailyRemaining int
}

// Limiter checks whether requests are within quota.
type Limiter interface {
	// Allow checks and consumes one request unit for the given key.
	Allow(ctx context.Context, apiKey string) (RateLimitResult, error)

	// SetQuota configures the quota for an API key.
	SetQuota(ctx context.Context, apiKey string, quota Quota) error

	// GetQuota returns the current quota configuration for an API key.
	GetQuota(ctx context.Context, apiKey string) (Quota, error)
}

// ---- Token Bucket in-memory implementation ----

type tokenBucket struct {
	quota     Quota
	tokens    float64
	lastRefil time.Time
	mu        sync.Mutex

	// Daily counter
	dailyCount int
	dayStart   time.Time
}

func newBucket(q Quota) *tokenBucket {
	burst := q.BurstSize
	if burst <= 0 {
		burst = q.RequestsPerMinute
	}
	return &tokenBucket{
		quota:     q,
		tokens:    float64(burst),
		lastRefil: time.Now(),
		dayStart:  startOfDay(time.Now()),
	}
}

func (b *tokenBucket) allow() RateLimitResult {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()

	// Reset daily counter if new day
	if now.After(b.dayStart.Add(24 * time.Hour)) {
		b.dailyCount = 0
		b.dayStart = startOfDay(now)
	}

	// Check daily limit
	if b.quota.DailyLimit > 0 && b.dailyCount >= b.quota.DailyLimit {
		resetAt := b.dayStart.Add(24 * time.Hour)
		return RateLimitResult{
			Allowed:        false,
			RetryAfter:     time.Until(resetAt),
			ResetAt:        resetAt,
			DailyRemaining: 0,
		}
	}

	// Refill tokens based on elapsed time
	burst := float64(b.quota.BurstSize)
	if burst <= 0 {
		burst = float64(b.quota.RequestsPerMinute)
	}
	elapsed := now.Sub(b.lastRefil).Seconds()
	refillRate := float64(b.quota.RequestsPerMinute) / 60.0
	b.tokens = min(burst, b.tokens+elapsed*refillRate)
	b.lastRefil = now

	if b.tokens < 1 {
		// Rate limited: calculate wait time
		waitSeconds := (1 - b.tokens) / refillRate
		return RateLimitResult{
			Allowed:        false,
			Remaining:      0,
			RetryAfter:     time.Duration(waitSeconds*float64(time.Second)) + time.Millisecond,
			ResetAt:        now.Add(time.Duration(waitSeconds * float64(time.Second))),
			DailyRemaining: b.quota.DailyLimit - b.dailyCount,
		}
	}

	b.tokens--
	b.dailyCount++

	return RateLimitResult{
		Allowed:        true,
		Remaining:      int(b.tokens),
		ResetAt:        now.Add(time.Minute),
		DailyRemaining: max(0, b.quota.DailyLimit-b.dailyCount),
	}
}

// InMemoryLimiter is a thread-safe in-memory token bucket rate limiter.
type InMemoryLimiter struct {
	mu           sync.RWMutex
	buckets      map[string]*tokenBucket
	defaultQuota Quota
}

// NewInMemoryLimiter creates an in-memory rate limiter with the given default quota.
func NewInMemoryLimiter(defaultQuota Quota) *InMemoryLimiter {
	return &InMemoryLimiter{
		buckets:      make(map[string]*tokenBucket),
		defaultQuota: defaultQuota,
	}
}

// Allow checks the rate limit for the given API key.
func (l *InMemoryLimiter) Allow(_ context.Context, apiKey string) (RateLimitResult, error) {
	l.mu.Lock()
	bucket, ok := l.buckets[apiKey]
	if !ok {
		bucket = newBucket(l.defaultQuota)
		l.buckets[apiKey] = bucket
	}
	l.mu.Unlock()

	return bucket.allow(), nil
}

// SetQuota configures the quota for a specific API key.
func (l *InMemoryLimiter) SetQuota(_ context.Context, apiKey string, quota Quota) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buckets[apiKey] = newBucket(quota)
	return nil
}

// GetQuota returns the quota for a specific API key.
func (l *InMemoryLimiter) GetQuota(_ context.Context, apiKey string) (Quota, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	bucket, ok := l.buckets[apiKey]
	if !ok {
		return l.defaultQuota, nil
	}
	return bucket.quota, nil
}

// ---- HTTP middleware ----

// Middleware returns an HTTP middleware that enforces rate limits.
// The apiKeyFn extracts the API key from the request (e.g. from Authorization header).
func Middleware(limiter Limiter, apiKeyFn func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := apiKeyFn(r)
			if key == "" {
				http.Error(w, `{"error":"missing API key"}`, http.StatusUnauthorized)
				return
			}

			result, err := limiter.Allow(r.Context(), key)
			if err != nil {
				http.Error(w, `{"error":"rate limit check failed"}`, http.StatusInternalServerError)
				return
			}

			// Always set rate limit headers
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", result.ResetAt.Unix()))
			if result.DailyRemaining > 0 {
				w.Header().Set("X-RateLimit-Daily-Remaining", fmt.Sprintf("%d", result.DailyRemaining))
			}

			if !result.Allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", result.RetryAfter.Seconds()))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprintf(w, `{"error":"rate limit exceeded","retry_after_seconds":%.0f}`, result.RetryAfter.Seconds())
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// BearerKeyExtractor extracts the API key from the "Authorization: Bearer <key>" header.
func BearerKeyExtractor(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !hasPrefix(auth, "Bearer ") {
		// Also check X-API-Key header as fallback
		return r.Header.Get("X-API-Key")
	}
	return auth[7:]
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
