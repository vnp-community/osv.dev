// Package resilience provides enterprise-grade circuit breaker and retry utilities.
package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// State is the circuit breaker state.
type State int

const (
	StateClosed   State = iota // Normal operation
	StateOpen                  // Failing fast
	StateHalfOpen              // Testing recovery
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreakerConfig holds circuit breaker configuration.
type CircuitBreakerConfig struct {
	// MaxFailures is the number of consecutive failures before opening.
	MaxFailures int
	// Timeout is how long to wait before transitioning to HALF_OPEN.
	Timeout time.Duration
	// HalfOpenMaxRequests is max concurrent requests allowed in HALF_OPEN state.
	HalfOpenMaxRequests int
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		MaxFailures:         5,
		Timeout:             30 * time.Second,
		HalfOpenMaxRequests: 2,
	}
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	name            string
	config          CircuitBreakerConfig
	mu              sync.Mutex
	state           State
	failures        int
	lastFailure     time.Time
	halfOpenAllowed int
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		name:   name,
		config: config,
		state:  StateClosed,
	}
}

// Execute runs fn through the circuit breaker.
// Returns ErrCircuitOpen if the circuit is open.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	if err := cb.allow(); err != nil {
		return err
	}

	err := fn(ctx)
	cb.record(err)
	return err
}

// ExecuteWithResult runs fn returning a result through the circuit breaker.
func ExecuteWithResult[T any](cb *CircuitBreaker, ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if err := cb.allow(); err != nil {
		return zero, err
	}
	result, err := fn(ctx)
	cb.record(err)
	return result, err
}

func (cb *CircuitBreaker) allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateOpen:
		if time.Since(cb.lastFailure) > cb.config.Timeout {
			cb.state = StateHalfOpen
			cb.halfOpenAllowed = cb.config.HalfOpenMaxRequests
			return nil // allow probe request
		}
		return fmt.Errorf("%w: circuit %q open until %v",
			ErrCircuitOpen, cb.name, cb.lastFailure.Add(cb.config.Timeout))

	case StateHalfOpen:
		if cb.halfOpenAllowed <= 0 {
			return fmt.Errorf("%w: circuit %q half-open limit reached", ErrCircuitOpen, cb.name)
		}
		cb.halfOpenAllowed--
		return nil

	default: // StateClosed
		return nil
	}
}

func (cb *CircuitBreaker) record(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()

		if cb.state == StateHalfOpen {
			cb.state = StateOpen // Back to open on failure in half-open
		} else if cb.failures >= cb.config.MaxFailures {
			cb.state = StateOpen
		}
		return
	}

	// Success
	if cb.state == StateHalfOpen {
		cb.state = StateClosed
	}
	cb.failures = 0
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Name returns the circuit breaker name.
func (cb *CircuitBreaker) Name() string { return cb.name }
