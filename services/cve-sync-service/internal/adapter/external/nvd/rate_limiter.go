package nvd

import (
	"context"
	"sync"
	"time"
)

const windowDuration = 30 * time.Second

// RateLimiter implements a sliding-window rate limiter for NVD API calls.
// Without API key: 5 requests per 30 seconds.
// With API key: 50 requests per 30 seconds.
type RateLimiter struct {
	hasAPIKey bool
	limit     int
	mu        sync.Mutex
	requests  []time.Time
}

// NewRateLimiter creates a RateLimiter.
// Pass hasAPIKey=true to use the 50 req/30s limit instead of 5 req/30s.
func NewRateLimiter(hasAPIKey bool) *RateLimiter {
	limit := 5
	if hasAPIKey {
		limit = 50
	}
	return &RateLimiter{
		hasAPIKey: hasAPIKey,
		limit:     limit,
		requests:  make([]time.Time, 0, limit),
	}
}

// Wait blocks until it is safe to make the next request.
// Returns context.Canceled or context.DeadlineExceeded if ctx is done.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		waitDur := rl.check()
		if waitDur == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDur):
			// retry
		}
	}
}

// check evicts expired timestamps and returns how long to wait (0 = proceed now).
func (rl *RateLimiter) check() time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-windowDuration)

	// Evict timestamps older than window
	var fresh []time.Time
	for _, t := range rl.requests {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	rl.requests = fresh

	if len(rl.requests) < rl.limit {
		// Under limit: record request and proceed
		rl.requests = append(rl.requests, now)
		return 0
	}

	// Over limit: wait until oldest + window expires
	oldest := rl.requests[0]
	return oldest.Add(windowDuration).Sub(now) + time.Millisecond
}

// Limit returns the current request limit per 30 seconds.
func (rl *RateLimiter) Limit() int {
	return rl.limit
}
