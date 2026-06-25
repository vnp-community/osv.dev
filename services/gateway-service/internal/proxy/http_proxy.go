// Package proxy provides the HTTP reverse proxy implementation for OpenVulnScan API Gateway.
// This file implements the HTTPProxy which forwards requests to upstream services
// using httputil.ReverseProxy wrapped with circuit breakers.
package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	
	"github.com/osv/gateway-service/internal/cache"
)

// UpstreamConfig holds address and timeout config for a single upstream service.
type UpstreamConfig struct {
	// Address is the full base URL of the upstream, e.g. "http://scan-service:9102"
	Address string
	// Timeout is the maximum time to wait for a response.
	Timeout time.Duration
}

// circuitState tracks basic circuit breaker state.
type circuitState struct {
	mu           sync.Mutex
	failures     int
	lastFailure  time.Time
	open         bool
	openUntil    time.Time
	threshold    int           // failures before opening
	resetTimeout time.Duration // how long to stay open
}

func newCircuitState() *circuitState {
	return &circuitState{
		threshold:    5,
		resetTimeout: 30 * time.Second,
	}
}

func (cs *circuitState) isOpen() bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.open && time.Now().After(cs.openUntil) {
		// Half-open: allow one request through
		cs.open = false
		cs.failures = 0
	}
	return cs.open
}

func (cs *circuitState) recordFailure() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.failures++
	cs.lastFailure = time.Now()
	if cs.failures >= cs.threshold {
		cs.open = true
		cs.openUntil = time.Now().Add(cs.resetTimeout)
	}
}

func (cs *circuitState) recordSuccess() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.failures = 0
}

// HTTPProxy routes incoming requests to upstream services using httputil.ReverseProxy.
// Each upstream has its own circuit breaker to prevent cascading failures.
type HTTPProxy struct {
	routes   []RouteConfig
	proxies  map[string]*httputil.ReverseProxy
	circuits map[string]*circuitState
	log      zerolog.Logger
	redis    *redis.Client
}

// NewHTTPProxy creates an HTTPProxy from a route list and upstream URL map.
// upstreamURLs maps upstream names to their base URLs.
func NewHTTPProxy(routes []RouteConfig, upstreamURLs map[string]string, redisClient *redis.Client, log zerolog.Logger) (*HTTPProxy, error) {
	proxies := make(map[string]*httputil.ReverseProxy, len(upstreamURLs))
	circuits := make(map[string]*circuitState, len(upstreamURLs))

	for name, rawURL := range upstreamURLs {
		target, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("parse upstream URL %q for %q: %w", rawURL, name, err)
		}

		rp := httputil.NewSingleHostReverseProxy(target)
		rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Error().Err(err).Str("upstream", name).Msg("proxy error")
			http.Error(w, `{"error":"upstream_error","message":"service unavailable"}`, http.StatusBadGateway)
		}

		proxies[name] = rp
		circuits[name] = newCircuitState()
	}

	return &HTTPProxy{
		routes:   routes,
		proxies:  proxies,
		circuits: circuits,
		log:      log,
		redis:    redisClient,
	}, nil
}

// ServeHTTP implements http.Handler. It matches the request path to a route,
// checks the circuit breaker, then forwards to the upstream.
func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Use RequestURI to get the full original path (chi.Mount strips URL.Path prefix).
	// Fall back to URL.Path if RequestURI is not available.
	routePath := r.RequestURI
	if idx := strings.Index(routePath, "?"); idx >= 0 {
		routePath = routePath[:idx]
	}
	if routePath == "" {
		routePath = r.URL.Path
	}
	route := FindRoute(routePath)
	if route == nil {
		http.Error(w, `{"error":"not_found","message":"no route matches this path"}`, http.StatusNotFound)
		return
	}

	// Restore r.URL.Path from the full RequestURI path.
	// chi.Mount() strips the mount prefix from r.URL.Path (e.g. /api/v2/cves/search → /cves/search).
	// httputil.ReverseProxy uses r.URL.Path to construct the upstream URL, so we must restore
	// the original full path to prevent the upstream from receiving a broken path.
	if routePath != r.URL.Path {
		r = r.Clone(r.Context())
		r.URL.Path = routePath
	}

	upstream := route.Upstream
	cb, ok := p.circuits[upstream]
	if !ok {
		http.Error(w, `{"error":"upstream_not_configured"}`, http.StatusBadGateway)
		return
	}

	if cb.isOpen() {
		p.log.Warn().Str("upstream", upstream).Msg("circuit breaker open")
		http.Error(w, `{"error":"service_unavailable","message":"upstream circuit breaker open"}`, http.StatusServiceUnavailable)
		return
	}

	proxy, ok := p.proxies[upstream]
	if !ok {
		http.Error(w, `{"error":"upstream_not_configured"}`, http.StatusBadGateway)
		return
	}

	// Wrap response writer to detect failures
	crw := &captureResponseWriter{ResponseWriter: w}
	
	var handler http.Handler = proxy
	if route.CacheTTL > 0 && p.redis != nil {
		handler = cache.Middleware(p.redis, route.CacheTTL)(handler)
	}

	handler.ServeHTTP(crw, r)

	if crw.statusCode >= 500 {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

// ForwardTo directly forwards a request to a named upstream, bypassing route matching.
// Used by the auth middleware to inject user context headers before forwarding.
func (p *HTTPProxy) ForwardTo(upstream string, w http.ResponseWriter, r *http.Request) {
	proxy, ok := p.proxies[upstream]
	if !ok {
		http.Error(w, `{"error":"upstream_not_configured"}`, http.StatusBadGateway)
		return
	}
	cb := p.circuits[upstream]
	if cb != nil && cb.isOpen() {
		http.Error(w, `{"error":"service_unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	proxy.ServeHTTP(w, r)
}

// InjectPrincipalHeaders adds authenticated user context as request headers.
// Downstream services use these headers to identify the caller without re-validating.
func InjectPrincipalHeaders(r *http.Request, userID, role string, permissions []string) *http.Request {
	r = r.Clone(r.Context())
	r.Header.Set("X-User-ID", userID)
	r.Header.Set("X-User-Role", role)
	r.Header.Set("X-User-Permissions", strings.Join(permissions, ","))
	// Remove Authorization to prevent forwarding JWT to internal services
	r.Header.Del("Authorization")
	return r
}

// captureResponseWriter wraps http.ResponseWriter to capture the status code.
type captureResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (crw *captureResponseWriter) WriteHeader(code int) {
	crw.statusCode = code
	crw.ResponseWriter.WriteHeader(code)
}

func (crw *captureResponseWriter) Write(b []byte) (int, error) {
	if crw.statusCode == 0 {
		crw.statusCode = http.StatusOK
	}
	return crw.ResponseWriter.Write(b)
}
