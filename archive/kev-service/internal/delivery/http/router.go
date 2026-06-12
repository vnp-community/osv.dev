package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

// NewRouter creates the chi router with all KEV routes registered.
func NewRouter(h *Handler, log zerolog.Logger) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(requestLogger(log))

	// Public endpoints
	r.Get("/health", h.Health)
	r.Get("/api/v2/kev", h.ListKEV)
	r.Get("/api/v2/kev/check", h.BulkCheck)
	r.Get("/api/v2/kev/stats", h.GetStats)
	r.Get("/api/v2/kev/sync/status", h.GetSyncStatus)
	r.Get("/api/v2/kev/{cveId}", h.GetKEVEntry)

	// Internal endpoints (not exposed via gateway)
	r.Post("/internal/kev/sync", h.TriggerSync)
	r.Get("/internal/kev/ids", h.GetAllIDs)

	return r
}

// requestLogger is a simple zerolog-based request logging middleware.
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
				Msg("request")
		})
	}
}
