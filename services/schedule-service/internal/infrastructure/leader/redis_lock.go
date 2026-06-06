// Package leader provides distributed leader election via Redis SET NX.
package leader

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLock implements distributed lock using Redis SET NX EX.
type RedisLock struct{ rdb *redis.Client }

// NewRedisLock creates a RedisLock backed by the given Redis client.
func NewRedisLock(rdb *redis.Client) *RedisLock { return &RedisLock{rdb: rdb} }

// TryAcquire attempts to acquire the lock. Returns true if acquired.
// Uses SET key "1" NX EX ttl — atomic, safe for distributed leader election.
func (l *RedisLock) TryAcquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	result, err := l.rdb.SetNX(ctx, key, "1", ttl).Result()
	return result, err
}

// Release releases the lock. Best-effort, ignores errors.
func (l *RedisLock) Release(ctx context.Context, key string) {
	l.rdb.Del(ctx, key) //nolint:errcheck
}
