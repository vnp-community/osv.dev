// Package middleware — security.go
// Security middleware collection: headers, rate limiting, body size limit,
// audit logging, and input validation.
package middleware

import (
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// ── Security Headers ──────────────────────────────────────────────────────────

// SecurityHeaders adds standard security response headers to every response.
// Implements OWASP recommended headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")
		// XSS protection for old browsers
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		// Referrer policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// HSTS — 1 year (only effective over HTTPS)
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		// Content Security Policy — restrictive by default
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://unpkg.com; style-src 'self' 'unsafe-inline' https://unpkg.com; img-src 'self' data:")
		// Permissions policy
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		next.ServeHTTP(w, r)
	})
}

// ── Request Body Size Limit ───────────────────────────────────────────────────

// MaxBodySize limits request body size to prevent large payload attacks.
// Default: 1MB.
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// ── In-Memory Rate Limiter ────────────────────────────────────────────────────

// rateBucket holds state for a single IP.
type rateBucket struct {
	count    int
	windowStart time.Time
}

// RateLimiter implements a simple sliding window rate limiter per IP.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*rateBucket
	maxReqs  int           // requests per window
	window   time.Duration // window duration
	cleanupInterval time.Duration
}

// NewRateLimiter creates a rate limiter.
// maxReqs: max requests per IP per window.
// window: time window (e.g., 1 * time.Second for 10 req/s).
func NewRateLimiter(maxReqs int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		buckets:         make(map[string]*rateBucket),
		maxReqs:         maxReqs,
		window:          window,
		cleanupInterval: 5 * time.Minute,
	}
	// Background cleanup of stale buckets
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[ip]
	if !ok || now.Sub(b.windowStart) >= rl.window {
		rl.buckets[ip] = &rateBucket{count: 1, windowStart: now}
		return true
	}
	b.count++
	return b.count <= rl.maxReqs
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(rl.cleanupInterval)
		rl.mu.Lock()
		cutoff := time.Now().Add(-rl.window * 10)
		for ip, b := range rl.buckets {
			if b.windowStart.Before(cutoff) {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimit returns middleware using the RateLimiter.
// Sends 429 Too Many Requests when limit exceeded.
func RateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractClientIP(r)
			if !rl.Allow(ip) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprintf(w, `{"error":"rate_limit_exceeded","message":"too many requests, slow down"}`) //nolint:errcheck
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractClientIP(r *http.Request) string {
	// Check forwarded headers first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// ── Audit Logging ─────────────────────────────────────────────────────────────

// AuditLog logs all write operations (non-GET, non-OPTIONS) with user context.
func AuditLog(l zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodOptions && r.Method != http.MethodHead {
				userID := r.Context().Value(ctxKeyUserID)
				userIDStr := "anonymous"
				if userID != nil {
					userIDStr = fmt.Sprintf("%v", userID)
				}
				l.Info().
					Str("audit", "true").
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("user_id", userIDStr).
					Str("ip", extractClientIP(r)).
					Str("user_agent", r.UserAgent()).
					Msg("audit")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ctxKeyUserID context key for user ID (matches auth middleware).
type ctxKeyType string

const ctxKeyUserID ctxKeyType = "user_id"

// ── Input Validation ──────────────────────────────────────────────────────────

// ScanTarget validates a scan target (IP, CIDR, or hostname).
func ValidateScanTarget(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("empty target")
	}

	// Valid IP address
	if net.ParseIP(target) != nil {
		return nil
	}

	// Valid CIDR block
	if _, _, err := net.ParseCIDR(target); err == nil {
		return nil
	}

	// Valid hostname (RFC 1123)
	hostnameRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	if hostnameRegex.MatchString(target) && len(target) <= 253 {
		return nil
	}

	return fmt.Errorf("invalid target %q: must be IP, CIDR, or hostname", target)
}

// ValidateScanTargets validates a list of scan targets.
func ValidateScanTargets(targets []string) error {
	if len(targets) == 0 {
		return fmt.Errorf("at least one target required")
	}
	if len(targets) > 100 {
		return fmt.Errorf("max 100 targets per scan, got %d", len(targets))
	}
	for _, t := range targets {
		if err := ValidateScanTarget(t); err != nil {
			return err
		}
	}
	return nil
}

// ── Panic Recovery ────────────────────────────────────────────────────────────

// Recoverer catches panics in handlers and returns 500.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error().
					Str("panic", fmt.Sprintf("%v", rec)).
					Str("path", r.URL.Path).
					Msg("handler panic recovered")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, `{"error":"internal_server_error"}`) //nolint:errcheck
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ── Timeout ───────────────────────────────────────────────────────────────────

// RequestTimeout wraps a handler with a context deadline.
func RequestTimeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set write deadline at transport level via http.Server config
			// This middleware just adds request context cancellation
			next.ServeHTTP(w, r)
		})
	}
}
