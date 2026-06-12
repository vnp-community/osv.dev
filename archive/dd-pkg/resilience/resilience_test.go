// Package resilience_test provides unit tests for circuit breaker and retry.
package resilience_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/defectdojo/pkg/resilience"
)

// ── Circuit Breaker Tests ─────────────────────────────────────────────────────

func TestCircuitBreaker_ClosedByDefault(t *testing.T) {
	cb := resilience.NewCircuitBreaker("test", resilience.DefaultCircuitBreakerConfig())
	if cb.State() != resilience.StateClosed {
		t.Fatalf("expected CLOSED, got %s", cb.State())
	}
}

func TestCircuitBreaker_OpensAfterMaxFailures(t *testing.T) {
	cfg := resilience.CircuitBreakerConfig{MaxFailures: 3, Timeout: 10 * time.Second}
	cb := resilience.NewCircuitBreaker("test", cfg)
	boom := errors.New("boom")

	for i := 0; i < 3; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error { //nolint:errcheck
			return boom
		})
	}

	if cb.State() != resilience.StateOpen {
		t.Fatalf("expected OPEN after 3 failures, got %s", cb.State())
	}
}

func TestCircuitBreaker_RejectsWhenOpen(t *testing.T) {
	cfg := resilience.CircuitBreakerConfig{MaxFailures: 1, Timeout: 10 * time.Second}
	cb := resilience.NewCircuitBreaker("test", cfg)
	boom := errors.New("boom")

	cb.Execute(context.Background(), func(ctx context.Context) error { return boom }) //nolint:errcheck

	err := cb.Execute(context.Background(), func(ctx context.Context) error { return nil })
	if !errors.Is(err, resilience.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got: %v", err)
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cfg := resilience.CircuitBreakerConfig{
		MaxFailures:         1,
		Timeout:             50 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	}
	cb := resilience.NewCircuitBreaker("test", cfg)

	cb.Execute(context.Background(), func(ctx context.Context) error { //nolint:errcheck
		return errors.New("boom")
	})
	if cb.State() != resilience.StateOpen {
		t.Fatal("expected OPEN")
	}

	time.Sleep(60 * time.Millisecond)

	// Should allow one probe request
	err := cb.Execute(context.Background(), func(ctx context.Context) error { return nil })
	if err != nil {
		t.Fatalf("expected probe allowed, got: %v", err)
	}
	if cb.State() != resilience.StateClosed {
		t.Fatalf("expected CLOSED after successful probe, got %s", cb.State())
	}
}

func TestCircuitBreaker_ResetsOnSuccess(t *testing.T) {
	cfg := resilience.CircuitBreakerConfig{MaxFailures: 5, Timeout: 10 * time.Second}
	cb := resilience.NewCircuitBreaker("test", cfg)
	boom := errors.New("boom")

	// Cause some failures but not enough to open
	for i := 0; i < 3; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error { return boom }) //nolint:errcheck
	}

	// Success resets counter
	cb.Execute(context.Background(), func(ctx context.Context) error { return nil }) //nolint:errcheck

	if cb.State() != resilience.StateClosed {
		t.Fatal("expected CLOSED after success reset")
	}
}

// ── Retry Tests ───────────────────────────────────────────────────────────────

func TestRetry_SucceedsOnFirstAttempt(t *testing.T) {
	cfg := resilience.DefaultRetryConfig()
	calls := int32(0)

	err := resilience.Do(context.Background(), cfg, func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetry_RetriesOnError(t *testing.T) {
	cfg := resilience.RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 1 * time.Millisecond,
		MaxInterval:     5 * time.Millisecond,
		Multiplier:      2.0,
	}
	calls := int32(0)
	boom := errors.New("boom")

	err := resilience.Do(context.Background(), cfg, func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return boom
	})

	if err == nil {
		t.Fatal("expected error after max retries")
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetry_SucceedsOnSecondAttempt(t *testing.T) {
	cfg := resilience.RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 1 * time.Millisecond,
		MaxInterval:     5 * time.Millisecond,
		Multiplier:      2.0,
	}
	calls := int32(0)
	boom := errors.New("boom")

	err := resilience.Do(context.Background(), cfg, func(ctx context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 2 {
			return boom
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil on 2nd attempt, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestRetry_RespectsCtxCancellation(t *testing.T) {
	cfg := resilience.RetryConfig{
		MaxAttempts:     100,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     500 * time.Millisecond,
		Multiplier:      2.0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := resilience.Do(ctx, cfg, func(ctx context.Context) error {
		return errors.New("boom")
	})

	if err == nil {
		t.Fatal("expected error (ctx cancelled)")
	}
}

func TestRetry_SkipsNonRetryableErrors(t *testing.T) {
	boom := errors.New("400 bad request")
	cfg := resilience.RetryConfig{
		MaxAttempts:     5,
		InitialInterval: 1 * time.Millisecond,
		IsRetryable: func(err error) bool {
			return !errors.Is(err, boom)
		},
	}

	calls := int32(0)
	err := resilience.Do(context.Background(), cfg, func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return boom
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (non-retryable), got %d", calls)
	}
}
