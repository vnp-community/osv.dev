// Package gateway is the API Gateway goroutine.
// It is the single external entry point for all GlobalCVE services.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/globalcve/mono/internal/config"
	cvesearchhttp "github.com/globalcve/mono/internal/cvesearch/http"
)

// Service is the API Gateway goroutine.
type Service struct {
	cfg            config.Config
	redis          *goredis.Client
	cveSearchHandler *cvesearchhttp.Handler
	server         *http.Server
}

// New creates a new API Gateway service.
// cveSearchHandler is injected directly (same process, zero-latency).
func New(cfg config.Config, redis *goredis.Client, cveSearchHandler *cvesearchhttp.Handler) *Service {
	return &Service{
		cfg:              cfg,
		redis:            redis,
		cveSearchHandler: cveSearchHandler,
	}
}

// Start launches the API Gateway HTTP server.
func (s *Service) Start(ctx context.Context) error {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(s.corsMiddleware)
	r.Use(s.rateLimitMiddleware)

	// Health aggregate
	r.Get("/health", s.handleHealth)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"service": "GlobalCVE API",
			"version": "3.0",
			"docs":    "/docs",
		})
	})

	// Public API v2
	r.Route("/api/v2", func(r chi.Router) {
		// CVE Search — direct handler call (same process, zero-latency)
		r.Get("/cves", s.cveSearchHandler.SearchCVEs)
		r.Get("/cves/{id}", s.cveSearchHandler.GetCVE)

		// KEV Service — HTTP proxy to internal port
		r.Get("/kev", s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Server.KEVServicePort)))
		r.Get("/kev/check", s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Server.KEVServicePort)))
		r.Get("/kev/stats", s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Server.KEVServicePort)))
		r.Get("/kev/{cveId}", s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Server.KEVServicePort)))

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)

			// Webhooks — proxy to notification service
			r.Get("/webhooks", s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Server.NotificationPort)))
			r.Post("/webhooks", s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Server.NotificationPort)))
			r.Delete("/webhooks/{id}", s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Server.NotificationPort)))

			// Sync — proxy to CVE sync admin API
			r.Get("/sync/status", s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Server.CVESyncPort)))
			r.Post("/sync/trigger", s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Server.CVESyncPort)))
			r.Post("/sync/trigger/{source}", s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Server.CVESyncPort)))
		})
	})

	addr := fmt.Sprintf(":%d", s.cfg.Server.GatewayPort)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
	}

	log.Ctx(ctx).Info().Str("addr", addr).Msg("gateway: starting")

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.Server.ShutdownTimeout)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("gateway server: %w", err)
	}
	return nil
}

// --- Middleware ---

func (s *Service) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false
		for _, o := range s.cfg.CORS.AllowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}
		if allowed && origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else if len(s.cfg.CORS.AllowedOrigins) > 0 && s.cfg.CORS.AllowedOrigins[0] == "*" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Service) rateLimitMiddleware(next http.Handler) http.Handler {
	maxReq := s.cfg.RateLimit.MaxRequests
	window := s.cfg.RateLimit.Window

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.redis == nil || maxReq <= 0 {
			next.ServeHTTP(w, r)
			return
		}

		ip := extractIP(r)
		key := "rl:ip:" + ip

		pipe := s.redis.Pipeline()
		incr := pipe.Incr(r.Context(), key)
		pipe.Expire(r.Context(), key, window)
		if _, err := pipe.Exec(r.Context()); err != nil {
			// Fail open: allow on Redis error
			next.ServeHTTP(w, r)
			return
		}

		if incr.Val() > int64(maxReq) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded",
			})
			return
		}

		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", maxReq))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", int64(maxReq)-incr.Val()))
		next.ServeHTTP(w, r)
	})
}

func (s *Service) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check Authorization header or X-API-Key
		auth := r.Header.Get("Authorization")
		apiKey := r.Header.Get("X-API-Key")

		if auth == "" && apiKey == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		// TODO: validate JWT or API key
		next.ServeHTTP(w, r)
	})
}

// --- Handlers ---

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Check all internal services
	services := map[string]string{
		"cve-search":   fmt.Sprintf("http://localhost:%d/health", s.cfg.Server.CVESearchPort),
		"cve-sync":     fmt.Sprintf("http://localhost:%d/health", s.cfg.Server.CVESyncPort),
		"kev-service":  fmt.Sprintf("http://localhost:%d/health", s.cfg.Server.KEVServicePort),
		"notification": fmt.Sprintf("http://localhost:%d/health", s.cfg.Server.NotificationPort),
	}

	client := &http.Client{Timeout: 3 * time.Second}
	statuses := make(map[string]string)
	overall := "ok"

	for name, healthURL := range services {
		resp, err := client.Get(healthURL)
		if err != nil || resp.StatusCode != http.StatusOK {
			statuses[name] = "unavailable"
			overall = "degraded"
		} else {
			statuses[name] = "ok"
			resp.Body.Close()
		}
	}

	status := http.StatusOK
	if overall != "ok" {
		status = http.StatusOK // still 200, just degraded
	}
	writeJSON(w, status, map[string]interface{}{
		"status":   overall,
		"services": statuses,
	})
}

// proxyTo creates a reverse proxy handler to the given upstream URL.
func (s *Service) proxyTo(upstreamBase string) http.HandlerFunc {
	target, _ := url.Parse(upstreamBase)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Error().Err(err).Str("upstream", upstreamBase).Msg("gateway: proxy error")
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream unavailable"})
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Rewrite path: /api/v2/kev → /api/v2/kev
		r.URL.Host = target.Host
		r.URL.Scheme = target.Scheme
		r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
		proxy.ServeHTTP(w, r)
	}
}

// --- helpers ---

func extractIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.Split(fwd, ",")[0]
	}
	if fwd := r.Header.Get("X-Real-IP"); fwd != "" {
		return fwd
	}
	// Strip port from RemoteAddr
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return addr[:i]
	}
	return addr
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
