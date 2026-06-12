// Package resilience provides retry utilities with exponential backoff and jitter.
package resilience

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// RetryConfig holds retry configuration.
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts (including the first).
	MaxAttempts int
	// InitialInterval is the delay before the first retry.
	InitialInterval time.Duration
	// MaxInterval is the maximum delay between retries.
	MaxInterval time.Duration
	// Multiplier is the backoff multiplier (typically 2.0).
	Multiplier float64
	// JitterFraction adds randomness to prevent thundering herd (0.0–1.0).
	JitterFraction float64
	// IsRetryable determines whether an error should trigger a retry.
	// If nil, all errors are retried.
	IsRetryable func(error) bool
}

// DefaultRetryConfig returns sensible defaults for microservice calls.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		Multiplier:      2.0,
		JitterFraction:  0.3,
	}
}

// NetworkRetryConfig returns retry config for network/remote calls.
func NetworkRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:     5,
		InitialInterval: 200 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		Multiplier:      2.0,
		JitterFraction:  0.25,
		IsRetryable:     IsRetryableNetworkError,
	}
}

// ErrMaxRetriesExceeded is wrapped around the last error when retries are exhausted.
var ErrMaxRetriesExceeded = errors.New("max retries exceeded")

// Do executes fn with retries according to config.
// The provided context deadline is respected — if ctx expires during a wait, the function returns immediately.
func Do(ctx context.Context, config RetryConfig, fn func(ctx context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		// Check if retryable
		if config.IsRetryable != nil && !config.IsRetryable(lastErr) {
			return lastErr
		}

		// Last attempt — don't sleep
		if attempt == config.MaxAttempts-1 {
			break
		}

		// Compute backoff with jitter
		sleep := computeBackoff(config, attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleep):
		}
	}

	return &RetryError{Attempts: config.MaxAttempts, Err: lastErr}
}

// DoWithResult executes fn returning a result with retries.
func DoWithResult[T any](ctx context.Context, config RetryConfig, fn func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	var lastErr error
	var result T

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		result, lastErr = fn(ctx)
		if lastErr == nil {
			return result, nil
		}

		if config.IsRetryable != nil && !config.IsRetryable(lastErr) {
			return zero, lastErr
		}

		if attempt == config.MaxAttempts-1 {
			break
		}

		sleep := computeBackoff(config, attempt)
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(sleep):
		}
	}

	return zero, &RetryError{Attempts: config.MaxAttempts, Err: lastErr}
}

// RetryError wraps the last error from a retry loop.
type RetryError struct {
	Attempts int
	Err      error
}

func (e *RetryError) Error() string {
	return "after " + itoa(e.Attempts) + " attempts: " + e.Err.Error()
}

func (e *RetryError) Unwrap() error { return e.Err }

// IsRetryableNetworkError returns true for errors that indicate transient network failures.
func IsRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// Retry on context deadline exceeded (but not context cancelled — that's intentional)
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// Don't retry on circuit open errors
	if errors.Is(err, ErrCircuitOpen) {
		return false
	}
	// Retry on non-permanent errors (heuristic: if error message contains these keywords)
	msg := err.Error()
	for _, keyword := range []string{"connection refused", "timeout", "EOF", "reset", "unavailable", "429", "503"} {
		for i := 0; i < len(msg)-len(keyword)+1; i++ {
			if msg[i:i+len(keyword)] == keyword {
				return true
			}
		}
	}
	return false
}

func computeBackoff(config RetryConfig, attempt int) time.Duration {
	base := float64(config.InitialInterval) * math.Pow(config.Multiplier, float64(attempt))
	if base > float64(config.MaxInterval) {
		base = float64(config.MaxInterval)
	}

	// Add jitter: ±JitterFraction of base
	if config.JitterFraction > 0 {
		jitter := base * config.JitterFraction * (rand.Float64()*2 - 1)
		base += jitter
		if base < float64(config.InitialInterval) {
			base = float64(config.InitialInterval)
		}
	}

	return time.Duration(base)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte(n%10) + '0'
		n /= 10
	}
	return string(buf[pos:])
}
