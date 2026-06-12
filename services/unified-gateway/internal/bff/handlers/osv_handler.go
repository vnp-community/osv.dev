// Package http provides the GlobalCVE v2 HTTP delivery layer for the API gateway.
package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"github.com/osv/unified-gateway/internal/adapter/upstream"
	"github.com/osv/unified-gateway/internal/domain/entity"
	pkghealth "github.com/osv/shared/pkg/health"
)

// Config holds the gateway delivery configuration.
type Config struct {
	UpstreamURLs map[string]string // service name → base URL
	CORSOrigins  []string
	MaxRPM       int
	JWTSecret    string
}

// Handler holds all HTTP handlers for the GlobalCVE API gateway.
type Handler struct {
	upstream *upstream.HTTPUpstreamClient
	cfg      Config
	log      zerolog.Logger
}

// NewHandler creates an HTTP Handler.
func NewHandler(cfg Config, log zerolog.Logger) *Handler {
	services := make(map[string]upstream.UpstreamConfig, len(cfg.UpstreamURLs))
	for name, url := range cfg.UpstreamURLs {
		services[name] = upstream.UpstreamConfig{URL: url, Timeout: 15 * time.Second}
	}
	return &Handler{
		upstream: upstream.NewHTTPUpstreamClient(services),
		cfg:      cfg,
		log:      log,
	}
}

// Health handles GET /health — aggregate health of all upstream services.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	healthURLs := make(map[string]string, len(h.cfg.UpstreamURLs))
	for name, url := range h.cfg.UpstreamURLs {
		healthURLs[name] = url + "/health"
	}
	results := pkghealth.AggregateUpstreams(r.Context(), healthURLs)
	status := http.StatusOK
	if !pkghealth.IsAllHealthy(results) {
		status = http.StatusMultiStatus
	}
	respondJSON(w, status, map[string]interface{}{
		"status":    "ok",
		"service":   "api-gateway",
		"upstreams": results,
		"time":      time.Now().UTC().Format(time.RFC3339),
	})
}

// Proxy handles all /api/v2/* requests by routing to the appropriate upstream.
func (h *Handler) Proxy(w http.ResponseWriter, r *http.Request) {
	route, ok := entity.FindGlobalCVERoute(r.URL.Path)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "no route"})
		return
	}
	h.upstream.Forward(r.Context(), w, r, route.Upstream) //nolint:errcheck
}

// NewGlobalCVERouter creates the chi router for GlobalCVE v2 API.
func NewGlobalCVERouter(h *Handler, log zerolog.Logger) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(corsMiddleware(h.cfg.CORSOrigins))
	r.Use(requestLogger(log))

	// Health (public)
	r.Get("/health", h.Health)

	// Public CVE routes
	r.Get("/api/v2/cves", h.Proxy)
	r.Get("/api/v2/cves/{id}", h.Proxy)

	// Public KEV routes
	r.Get("/api/v2/kev", h.Proxy)
	r.Get("/api/v2/kev/check", h.Proxy)
	r.Get("/api/v2/kev/stats", h.Proxy)
	r.Get("/api/v2/kev/sync/status", h.Proxy)
	r.Get("/api/v2/kev/{cveId}", h.Proxy)

	// Sync status (public read)
	r.Get("/api/v2/sync/status", h.Proxy)

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(simpleAuthMiddleware(h.cfg.JWTSecret))
		r.Post("/api/v2/webhooks", h.Proxy)
		r.Get("/api/v2/webhooks", h.Proxy)
		r.Delete("/api/v2/webhooks/{id}", h.Proxy)
		r.Post("/api/v2/sync/trigger", h.Proxy)
	})

	return r
}

// --- middleware helpers ---

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-API-Key")
				w.Header().Set("Access-Control-Max-Age", "300")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// simpleAuthMiddleware performs minimal Bearer token presence check.
// Full JWT validation is done by the existing OVS auth validator.
func simpleAuthMiddleware(_ string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" || !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requestLogger(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.Status()).
				Dur("latency", time.Since(start)).
				Msg("v2 request")
		})
	}
}

func respondJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, "%s\n", mustJSON(v))
}

func respondError(w http.ResponseWriter, code int, message string) {
	respondJSON(w, code, map[string]string{"error": message})
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
