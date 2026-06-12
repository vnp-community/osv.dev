// Package health provides standard health probers for common infrastructure dependencies.
package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisProber checks Redis connectivity.
func RedisProber(rc redis.Cmdable) Checker {
	return NewCheckerFunc("redis", func(ctx context.Context) error {
		return rc.Ping(ctx).Err()
	})
}

// NATSProber checks NATS connectivity using a simple ping pattern.
type natsConn interface {
	IsConnected() bool
	RTT() (time.Duration, error)
}

// NATSProber checks NATS connectivity.
func NATSProber(nc natsConn) Checker {
	return NewCheckerFunc("nats", func(ctx context.Context) error {
		if !nc.IsConnected() {
			return fmt.Errorf("NATS not connected")
		}
		return nil
	})
}

// OpenSearchProber checks OpenSearch connectivity via /_cluster/health endpoint.
func OpenSearchProber(baseURL string) Checker {
	return NewCheckerFunc("opensearch", func(ctx context.Context) error {
		url := baseURL + "/_cluster/health?timeout=5s"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		client := &http.Client{Timeout: 6 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("opensearch unreachable: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 500 {
			return fmt.Errorf("opensearch cluster unhealthy: status %d", resp.StatusCode)
		}
		return nil
	})
}

// FirestoreProber checks Firestore connectivity by attempting a document read.
// It reads from a known-to-exist (or known-to-not-exist) path — either way a response means connectivity is OK.
type firestoreProber struct {
	checkFn func(ctx context.Context) error
}

// NewFirestoreProber creates a Firestore probe with a custom check function.
// Example: func(ctx) error { _, err := client.Collection("health").Doc("probe").Get(ctx); return ignoreNotFound(err) }
func NewFirestoreProber(checkFn func(ctx context.Context) error) Checker {
	return NewCheckerFunc("firestore", checkFn)
}
