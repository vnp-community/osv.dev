// Package cache — Redis-backed HTTP response cache.
package cache

import (
	"bytes"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// ResponseRecorder captures response body + status for caching.
type ResponseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (r *ResponseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *ResponseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *ResponseRecorder) StatusCode() int {
	if r.statusCode == 0 {
		return http.StatusOK
	}
	return r.statusCode
}

// Middleware returns a caching HTTP middleware.
// Only caches GET requests with 2xx responses.
// Cache key: "gw:cache:" + request URI
func Middleware(redisClient *redis.Client, ttl time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only cache GET requests
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}

			cacheKey := cacheKeyFor(r)

			// Cache HIT
			if cached, err := redisClient.Get(r.Context(), cacheKey).Bytes(); err == nil {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "HIT")
				w.WriteHeader(http.StatusOK)
				w.Write(cached) //nolint:errcheck
				return
			}

			// Cache MISS — record response
			rec := &ResponseRecorder{ResponseWriter: w}
			w.Header().Set("X-Cache", "MISS")
			next.ServeHTTP(rec, r)

			// Cache 2xx responses
			if rec.StatusCode() >= 200 && rec.StatusCode() < 300 && rec.body.Len() > 0 {
				redisClient.Set(r.Context(), cacheKey, rec.body.Bytes(), ttl) //nolint:errcheck
			}
		})
	}
}

func cacheKeyFor(r *http.Request) string {
	return "gw:cache:" + r.URL.RequestURI()
}
