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

// Package ratelimit provides a Redis sliding-window rate limiter for the API Gateway.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Key identifies a unique rate limit bucket.
type Key struct {
	ClientID  string // API key ID or IP
	Endpoint  string
	Ecosystem string // optional
}

// Result is the outcome of a rate limit check.
type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int
	ResetAt    time.Time
	RetryAfter time.Duration
}

// RedisRateLimiter implements a sliding-window rate limiter using Redis Lua scripts.
type RedisRateLimiter struct {
	client *redis.Client
}

// NewRedisRateLimiter creates a new Redis-backed rate limiter.
func NewRedisRateLimiter(client *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{client: client}
}

// slidingWindowScript is an atomic Lua script implementing a sliding window counter.
// KEYS[1] = rate limit key
// ARGV[1] = window duration in seconds
// ARGV[2] = max requests in window
// ARGV[3] = current timestamp (Unix milliseconds)
var slidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local window = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local window_ms = window * 1000

-- Remove old entries outside the window
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window_ms)

-- Count current requests in window
local count = redis.call('ZCARD', key)

if count >= limit then
    -- Get oldest entry to calculate reset time
    local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local reset_at = 0
    if #oldest > 0 then
        reset_at = tonumber(oldest[2]) + window_ms
    end
    return {0, limit, 0, reset_at}
end

-- Add current request
redis.call('ZADD', key, now, now .. '-' .. math.random(1000000))
redis.call('EXPIRE', key, window)

return {1, limit, limit - count - 1, 0}
`)

// Check performs a sliding window rate limit check for the given key and tier.
// windowSeconds is the window duration; limit is max requests in that window.
func (r *RedisRateLimiter) Check(ctx context.Context, key Key, windowSeconds, limit int) (*Result, error) {
	if limit < 0 {
		// Unlimited tier
		return &Result{Allowed: true, Limit: -1, Remaining: -1}, nil
	}

	redisKey := fmt.Sprintf("ratelimit:%s:%s:%s", key.ClientID, key.Endpoint, key.Ecosystem)
	now := time.Now().UnixMilli()

	vals, err := slidingWindowScript.Run(ctx, r.client, []string{redisKey},
		windowSeconds, limit, now).Slice()
	if err != nil {
		return nil, fmt.Errorf("rate limiter: redis script: %w", err)
	}

	allowed := vals[0].(int64) == 1
	lim := int(vals[1].(int64))
	remaining := int(vals[2].(int64))
	resetAtMs := vals[3].(int64)

	result := &Result{
		Allowed:   allowed,
		Limit:     lim,
		Remaining: remaining,
	}

	if !allowed && resetAtMs > 0 {
		result.ResetAt = time.UnixMilli(resetAtMs)
		result.RetryAfter = time.Until(result.ResetAt)
		if result.RetryAfter < 0 {
			result.RetryAfter = 0
		}
	}

	return result, nil
}
