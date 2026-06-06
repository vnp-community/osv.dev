// Package middleware provides HTTP middleware for the API gateway.
package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

// IPRateLimiter implements per-IP token bucket rate limiting.
type IPRateLimiter struct {
	limiters sync.Map     // key: IP string → *rate.Limiter
	r        rate.Limit   // requests per second
	b        int          // burst size
}

// NewIPRateLimiter creates a rate limiter with rps requests/second and burst size.
func NewIPRateLimiter(rps float64, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		r: rate.Limit(rps),
		b: burst,
	}
}

// getLimiter returns (or creates) the limiter for a given IP.
func (rl *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	if v, ok := rl.limiters.Load(ip); ok {
		return v.(*rate.Limiter)
	}
	lim := rate.NewLimiter(rl.r, rl.b)
	rl.limiters.Store(ip, lim)
	return lim
}

// Middleware returns an HTTP middleware that enforces rate limiting per IP.
func (rl *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := realIP(r)
		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"rate limit exceeded","status":429}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// realIP extracts the real client IP from request headers.
func realIP(r *http.Request) string {
	// Check X-Real-IP first (set by nginx/proxy)
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	// Check X-Forwarded-For (may contain multiple IPs)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Take the first (leftmost) IP
		parts := strings.SplitN(forwarded, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
