// Package integration provides testcontainers-go based integration tests for pkg/resilience.
//
// These tests spin up real Redis and NATS containers to verify circuit breaker
// and health prober behavior against actual infrastructure.
//
// Run with: go test -tags=integration -timeout=120s ./test/integration/...
//
//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/nats"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/rs/zerolog"

	"github.com/osv/shared/pkg/health"
	"github.com/osv/shared/pkg/resilience"
)

// TestRedisProber_Integration verifies Redis health prober against a real Redis container.
func TestRedisProber_Integration(t *testing.T) {
	ctx := context.Background()

	// Start Redis container
	redisC, err := testredis.RunContainer(ctx,
		testcontainers.WithImage("redis:7.2-alpine"),
	)
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	t.Cleanup(func() { redisC.Terminate(ctx) }) //nolint:errcheck

	endpoint, err := redisC.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("get redis endpoint: %v", err)
	}

	rc := redis.NewClient(&redis.Options{Addr: endpoint})
	defer rc.Close()

	// Probe should be healthy
	prober := health.RedisProber(rc)
	result := prober.Check(ctx)
	if result.Status != health.StatusOK {
		t.Errorf("expected OK, got %s: %s", result.Status, result.Message)
	}

	// Stop redis, probe should fail
	redisC.Stop(ctx, nil) //nolint:errcheck
	time.Sleep(200 * time.Millisecond)

	result = prober.Check(ctx)
	if result.Status != health.StatusDown {
		t.Errorf("expected DOWN after stop, got %s", result.Status)
	}
}

// TestNATSProber_Integration verifies NATS health prober against a real NATS container.
func TestNATSProber_Integration(t *testing.T) {
	ctx := context.Background()

	natsC, err := nats.RunContainer(ctx,
		testcontainers.WithImage("nats:2.10-alpine"),
	)
	if err != nil {
		t.Fatalf("start nats container: %v", err)
	}
	t.Cleanup(func() { natsC.Terminate(ctx) }) //nolint:errcheck

	endpoint, err := natsC.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("get nats endpoint: %v", err)
	}

	nc, err := nats.Connect(endpoint)
	if err != nil {
		t.Fatalf("connect to nats: %v", err)
	}
	defer nc.Close()

	prober := health.NATSProber(nc)
	result := prober.Check(ctx)
	if result.Status != health.StatusOK {
		t.Errorf("expected OK, got %s: %s", result.Status, result.Message)
	}
}

// TestCircuitBreaker_WithRealRedis verifies circuit breaker behavior with real Redis.
func TestCircuitBreaker_WithRealRedis(t *testing.T) {
	ctx := context.Background()

	redisC, err := testredis.RunContainer(ctx,
		testcontainers.WithImage("redis:7.2-alpine"),
	)
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	t.Cleanup(func() { redisC.Terminate(ctx) }) //nolint:errcheck

	endpoint, err := redisC.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("get redis endpoint: %v", err)
	}

	rc := redis.NewClient(&redis.Options{Addr: endpoint})
	defer rc.Close()

	// Create circuit breaker with tight config for testing
	cb := resilience.NewCircuitBreaker("redis-test", resilience.CircuitBreakerConfig{
		MaxFailures:         2,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 1,
	})

	// Successful operation via circuit breaker
	err = cb.Execute(ctx, func(ctx context.Context) error {
		return rc.Ping(ctx).Err()
	})
	if err != nil {
		t.Fatalf("unexpected error on healthy redis: %v", err)
	}
	if cb.State() != resilience.StateClosed {
		t.Errorf("expected CLOSED, got %s", cb.State())
	}

	// Stop redis → simulate failures
	redisC.Stop(ctx, nil) //nolint:errcheck
	time.Sleep(100 * time.Millisecond)

	boom := errors.New("connection refused")
	for i := 0; i < 3; i++ {
		cb.Execute(ctx, func(ctx context.Context) error { //nolint:errcheck
			if err := rc.Ping(ctx).Err(); err != nil {
				return err
			}
			return boom
		})
	}

	if cb.State() != resilience.StateOpen {
		t.Errorf("expected OPEN after failures, got %s", cb.State())
	}
	t.Logf("circuit breaker correctly OPENED after redis failure")
}

// TestMultiChecker_Integration verifies MultiChecker with real Redis and NATS.
func TestMultiChecker_Integration(t *testing.T) {
	ctx := context.Background()
	log := zerolog.Nop()
	_ = log

	// Start Redis
	redisC, err := testredis.RunContainer(ctx,
		testcontainers.WithImage("redis:7.2-alpine"),
	)
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() { redisC.Terminate(ctx) }) //nolint:errcheck

	redisEndpoint, _ := redisC.ConnectionString(ctx)
	rc := redis.NewClient(&redis.Options{Addr: redisEndpoint})
	defer rc.Close()

	// Start NATS
	natsC, err := nats.RunContainer(ctx,
		testcontainers.WithImage("nats:2.10-alpine"),
	)
	if err != nil {
		t.Fatalf("start nats: %v", err)
	}
	t.Cleanup(func() { natsC.Terminate(ctx) }) //nolint:errcheck

	natsEndpoint, _ := natsC.ConnectionString(ctx)
	nc, _ := nats.Connect(natsEndpoint)
	defer nc.Close()

	// Both healthy
	checker := health.NewMultiChecker(5*time.Second,
		health.RedisProber(rc),
		health.NATSProber(nc),
	)
	report := checker.Check(ctx)
	if report.Status != health.StatusOK {
		t.Errorf("expected OK, got %s: %+v", report.Status, report.Checks)
	}
	t.Logf("all probers healthy: %s", report.Status)
}
