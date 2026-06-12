package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/defectdojo/api-gateway/internal/infra/ratelimit"
)

func TestInMemoryLimiter(t *testing.T) {
	ctx := context.Background()

	t.Run("allows requests within quota", func(t *testing.T) {
		limiter := ratelimit.NewInMemoryLimiter(ratelimit.Quota{
			RequestsPerMinute: 60,
			BurstSize:         10,
		})
		result, err := limiter.Allow(ctx, "key-1")
		if err != nil {
			t.Fatalf("Allow: %v", err)
		}
		if !result.Allowed {
			t.Error("expected request to be allowed")
		}
	})

	t.Run("exhausts burst and rate limits", func(t *testing.T) {
		limiter := ratelimit.NewInMemoryLimiter(ratelimit.Quota{
			RequestsPerMinute: 60,
			BurstSize:         3,
		})
		// Exhaust the burst
		for i := 0; i < 3; i++ {
			r, _ := limiter.Allow(ctx, "key-burst")
			if !r.Allowed {
				t.Fatalf("request %d should be allowed", i+1)
			}
		}
		// Next should be rate limited
		r, _ := limiter.Allow(ctx, "key-burst")
		if r.Allowed {
			t.Error("expected 4th request to be rate limited")
		}
		if r.RetryAfter <= 0 {
			t.Error("RetryAfter should be > 0 when rate limited")
		}
	})

	t.Run("different keys are independent", func(t *testing.T) {
		limiter := ratelimit.NewInMemoryLimiter(ratelimit.Quota{
			RequestsPerMinute: 60,
			BurstSize:         2,
		})
		// Exhaust key-a
		limiter.Allow(ctx, "key-a")
		limiter.Allow(ctx, "key-a")
		r1, _ := limiter.Allow(ctx, "key-a")
		if r1.Allowed {
			t.Error("key-a should be rate limited")
		}
		// key-b should still work
		r2, _ := limiter.Allow(ctx, "key-b")
		if !r2.Allowed {
			t.Error("key-b should still be allowed")
		}
	})

	t.Run("SetQuota changes limits", func(t *testing.T) {
		limiter := ratelimit.NewInMemoryLimiter(ratelimit.Quota{
			RequestsPerMinute: 1,
			BurstSize:         1,
		})
		// Set a higher quota
		err := limiter.SetQuota(ctx, "key-q", ratelimit.Quota{
			RequestsPerMinute: 600,
			BurstSize:         100,
		})
		if err != nil {
			t.Fatalf("SetQuota: %v", err)
		}
		// Should now allow many requests
		for i := 0; i < 10; i++ {
			r, _ := limiter.Allow(ctx, "key-q")
			if !r.Allowed {
				t.Errorf("request %d should be allowed after SetQuota", i+1)
			}
		}
	})

	t.Run("GetQuota returns configured quota", func(t *testing.T) {
		limiter := ratelimit.NewInMemoryLimiter(ratelimit.Quota{
			RequestsPerMinute: 60,
			BurstSize:         120,
		})
		quota, err := limiter.GetQuota(ctx, "any-key")
		if err != nil {
			t.Fatalf("GetQuota: %v", err)
		}
		if quota.RequestsPerMinute != 60 {
			t.Errorf("RequestsPerMinute: got %d, want 60", quota.RequestsPerMinute)
		}
	})

	t.Run("daily limit enforced", func(t *testing.T) {
		limiter := ratelimit.NewInMemoryLimiter(ratelimit.Quota{
			RequestsPerMinute: 1000,
			BurstSize:         5,
			DailyLimit:        3,
		})
		// Use up the daily limit
		for i := 0; i < 3; i++ {
			r, _ := limiter.Allow(ctx, "key-daily")
			if !r.Allowed {
				t.Fatalf("request %d should be allowed within daily limit", i+1)
			}
		}
		// 4th request should be blocked by daily limit
		r, _ := limiter.Allow(ctx, "key-daily")
		if r.Allowed {
			t.Error("expected 4th request to be blocked by daily limit")
		}
	})

	t.Run("tier quotas defined", func(t *testing.T) {
		for tier, q := range ratelimit.TierQuotas {
			if q.RequestsPerMinute <= 0 {
				t.Errorf("tier %q: RequestsPerMinute must be > 0", tier)
			}
		}
	})
}

func TestResetAt(t *testing.T) {
	ctx := context.Background()
	limiter := ratelimit.NewInMemoryLimiter(ratelimit.Quota{
		RequestsPerMinute: 60,
		BurstSize:         5,
	})
	r, _ := limiter.Allow(ctx, "key-time")
	if r.ResetAt.IsZero() {
		t.Error("ResetAt should not be zero")
	}
	if r.ResetAt.Before(time.Now()) {
		t.Error("ResetAt should be in the future")
	}
}
