// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package redis provides Redis-backed idempotency store.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultTTL = 24 * time.Hour
const keyPrefix = "osv:idempotency:"

// IdempotencyStore uses Redis SETNX for content-hash-based deduplication.
type IdempotencyStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewIdempotencyStore creates a new Redis-backed idempotency store.
func NewIdempotencyStore(client *redis.Client, ttl time.Duration) *IdempotencyStore {
	if ttl == 0 {
		ttl = defaultTTL
	}
	return &IdempotencyStore{client: client, ttl: ttl}
}

// IsProcessed returns true if the content hash is already marked as processed.
func (s *IdempotencyStore) IsProcessed(ctx context.Context, contentHash string) (bool, error) {
	key := keyPrefix + contentHash
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis idempotency check: %w", err)
	}
	return exists > 0, nil
}

// MarkProcessed marks the content hash as processed with the configured TTL.
func (s *IdempotencyStore) MarkProcessed(ctx context.Context, contentHash string) error {
	key := keyPrefix + contentHash
	// SETNX with TTL
	set, err := s.client.SetNX(ctx, key, "1", s.ttl).Result()
	if err != nil {
		return fmt.Errorf("redis mark processed: %w", err)
	}
	if !set {
		// Key already exists — already marked, that's fine
		return nil
	}
	return nil
}
