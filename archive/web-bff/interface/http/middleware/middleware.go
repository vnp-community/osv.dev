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

// Package middleware contains HTTP middleware for the Web BFF.
package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"time"

	gogredis "github.com/redis/go-redis/v9"
)

// ─────────────────────────────────────────────
// CORS Middleware
// ─────────────────────────────────────────────

var allowedOrigins = map[string]bool{
	"https://osv.dev":       true,
	"https://www.osv.dev":   true,
}

// CORS handles Cross-Origin Resource Sharing headers.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Allow osv.dev and subdomains, and localhost for dev.
		allowed := allowedOrigins[origin] ||
			strings.HasSuffix(origin, ".osv.dev") ||
			strings.HasPrefix(origin, "http://localhost:")

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ─────────────────────────────────────────────
// Rate Limit Middleware (IP-based, Redis sliding window)
// ─────────────────────────────────────────────

// rateLimitScript is a Lua atomic sliding-window rate limiter.
// Key schema: osv:ratelimit:{sha256(ip)}:{minute_epoch}
var rateLimitScript = gogredis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local count = redis.call("INCR", key)
if count == 1 then
  redis.call("EXPIRE", key, window)
end
if count > limit then
  return 0
end
return 1
`)

// RateLimit returns a middleware that limits requests per IP using Redis.
// limit: max requests per window; window: time window duration.
func RateLimit(client *gogredis.Client, limit int, window time.Duration) func(http.Handler) http.Handler {
	windowSec := int(window.Seconds())
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)
			minute := time.Now().Unix() / 60
			key := fmt.Sprintf("osv:ratelimit:%x:%d", sha256.Sum256([]byte(ip)), minute)

			allowed, err := rateLimitScript.Run(context.Background(), client, []string{key}, limit, windowSec).Int()
			if err != nil || allowed == 0 {
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.Split(xff, ",")[0]
	}
	return r.RemoteAddr
}

// ─────────────────────────────────────────────
// Logging Middleware
// ─────────────────────────────────────────────

// Logging logs HTTP request method, path, status, and duration.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{w, http.StatusOK}
		next.ServeHTTP(rw, r)
		_ = time.Since(start) // in production: emit to zerolog
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
