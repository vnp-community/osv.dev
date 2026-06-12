// Package http — cve-search v1 routes (search/browse/query/taxonomy/ranking/ingest/info).
// This file registers all /api/v1/cve-search/* routes on top of the existing CVEHandler.
package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	gatemw "github.com/osv/gateway-service/internal/delivery/http/middleware"
)

// NewCVESearchRouter creates the chi sub-router for all cve-search v1 routes.
// Routes are split into public (no JWT required) and protected (JWT + optional role).
//
// Mount under "/" in NewCVERouter to register alongside existing v1 endpoints:
//
//	r.Mount("/", NewCVESearchRouter(h, jwtSecret))
func NewCVESearchRouter(h *CVEHandler, jwtSecret string) http.Handler {
	r := chi.NewRouter()

	// ── Public routes — no auth required ─────────────────────────────────────
	r.Post("/api/v1/auth/login", h.Proxy)
	r.Post("/api/v1/auth/register", h.Proxy)
	r.Post("/api/v1/auth/refresh", h.Proxy)
	r.Get("/api/v1/auth/jwks.json", h.Proxy)
	r.Get("/.well-known/jwks.json", h.Proxy)

	// ── Protected routes — JWT required ──────────────────────────────────────
	publicPaths := []string{
		"/health", "/ready",
		"/api/v1/auth",
		"/.well-known/",
		"/api/v2/cves", "/api/v2/kev",
	}

	r.Group(func(r chi.Router) {
		r.Use(gatemw.JWTVerify(jwtSecret, publicPaths))

		// Auth extras (protected)
		r.Post("/api/v1/auth/logout", h.Proxy)
		r.Get("/api/v1/auth/me", h.Proxy)

		// CVE service — lookup, recent, search
		r.Get("/api/v1/cve/{id}", h.Proxy)
		r.Get("/api/v1/cve/last/{n}", h.Proxy)
		r.Get("/api/v1/cve/recent/{timeframe}", h.Proxy)
		r.Get("/api/v1/cve/search", h.Proxy)

		// Browse service
		r.Get("/api/v1/browse/", h.Proxy)
		r.Get("/api/v1/browse/{vendor}", h.Proxy)
		r.Get("/api/v1/browse/{vendor}/{product}", h.Proxy)
		r.Get("/api/v1/search/{vendor}/{product}", h.Proxy)
		r.Get("/api/v1/search/{vendor}/{product}/{version}", h.Proxy)
		r.Get("/api/v1/vendors/search", h.Proxy)
		r.Get("/api/v1/products/search", h.Proxy)
		r.Get("/api/v1/versions/{vendor}/{product}", h.Proxy)

		// Query service — advanced CVE queries
		r.Get("/api/v1/query", h.Proxy)
		r.Post("/api/v1/query", h.Proxy)

		// Full-text search
		r.Get("/api/v1/fulltext", h.Proxy)

		// Taxonomy service — CWE + CAPEC
		r.Get("/api/v1/cwe/{id}", h.Proxy)
		r.Get("/api/v1/cwe/{id}/capec", h.Proxy)
		r.Get("/api/v1/capec/{id}", h.Proxy)
		r.Get("/api/v1/capec/{id}/cwe", h.Proxy)

		// Ranking service — EPSS / CVSS rankings
		r.Get("/api/v1/ranking", h.Proxy)
		r.Post("/api/v1/ranking", h.Proxy)
		r.Delete("/api/v1/ranking/{id}", h.Proxy)

		// Info service — database statistics
		r.Get("/api/v1/dbinfo", h.Proxy)
		r.Get("/api/v1/dbinfo/collections", h.Proxy)

		// Ingest service — admin-only operations
		r.Group(func(r chi.Router) {
			r.Use(gatemw.RequireRole("admin"))
			r.Get("/api/v1/ingest/status", h.Proxy)
			r.Post("/api/v1/ingest/trigger", h.Proxy)
			r.Post("/api/v1/ingest/import", h.Proxy)
		})
	})

	return r
}
