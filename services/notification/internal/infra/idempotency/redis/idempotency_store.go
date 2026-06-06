// infra/idempotency/redis/idempotency_store.go
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultTTL = 24 * time.Hour

// IdempotencyStore prevents duplicate notification delivery using Redis.
type IdempotencyStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewIdempotencyStore creates a Redis-backed idempotency store.
func NewIdempotencyStore(client *redis.Client) *IdempotencyStore {
	return &IdempotencyStore{client: client, ttl: defaultTTL}
}

// key formats the Redis key for a given event ID.
func (s *IdempotencyStore) key(eventID string) string {
	return fmt.Sprintf("osv:idem:notif:%s", eventID)
}

// IsProcessed returns true if the event has already been processed.
func (s *IdempotencyStore) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	result, err := s.client.Exists(ctx, s.key(eventID)).Result()
	if err != nil {
		return false, fmt.Errorf("check idempotency for %s: %w", eventID, err)
	}
	return result > 0, nil
}

// MarkProcessed marks an event as processed (SETNX with TTL).
// Returns true if this was the first time (successfully marked).
func (s *IdempotencyStore) MarkProcessed(ctx context.Context, eventID string) (bool, error) {
	ok, err := s.client.SetNX(ctx, s.key(eventID), "1", s.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("mark idempotency for %s: %w", eventID, err)
	}
	return ok, nil
}
