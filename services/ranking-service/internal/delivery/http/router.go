// Package http — Chi router for ranking-service.
package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter builds the ranking-service chi router.
// Note: Auth enforcement is done by the gateway (apps/osv).
// This service does NOT enforce auth — it trusts the gateway.
func NewRouter(h *Handler) *chi.Mux {
	r := chi.NewMux()

	// Standard middleware
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(30 * time.Second))

	// Health check (public)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "ranking-service",
		})
	})

	// Ranking lookup (public read — auth enforced by gateway)
	// NOTE: /ranking/lookup MUST come before /ranking (longer prefix)
	r.Get("/ranking/lookup", h.LookupRanking)

	// SEED-004: /ranking/bulk MUST come before /ranking/{id} (literal before wildcard)
	r.Post("/ranking/bulk", h.BulkCreateRankings)

	// Ranking CRUD (admin — auth enforced by gateway)
	r.Get("/ranking", h.ListRankings)
	r.Post("/ranking", h.CreateRanking)
	r.Delete("/ranking/{id}", h.DeleteRanking)

	return r
}
