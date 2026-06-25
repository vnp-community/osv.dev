package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

// RegisterKevRoutes registers all KEV routes onto the provided chi router.
func RegisterKevRoutes(r chi.Router, h *KevHandler, epssH *EPSSHandler, cweH *CWEHandler, vendorH *VendorHandler, log zerolog.Logger) {
	// Middleware (can be mounted globally instead, but we apply here for now if not already applied)

	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(requestLogger(log))

	// Public endpoints — /api/v2/kev/* (data-service internal path)
	r.Get("/health", h.Health)
	r.Get("/api/v2/kev", h.ListKEV)
	r.Get("/api/v2/kev/check", h.BulkCheck)
	r.Get("/api/v2/kev/stats", h.GetStats)
	r.Get("/api/v2/kev/sync/status", h.GetSyncStatus)
	r.Get("/api/v2/kev/ransomware", h.GetRansomware)
	r.Get("/api/v2/kev/{cveId}", h.GetKEVEntry)

	// Public endpoints — /api/v1/kev/* (alias for gateway proxy — same path forwarded from gateway)
	r.Get("/api/v1/kev", h.ListKEV)
	r.Get("/api/v1/kev/check", h.BulkCheck)
	r.Get("/api/v1/kev/stats", h.GetStats)
	r.Get("/api/v1/kev/sync/status", h.GetSyncStatus)
	r.Get("/api/v1/kev/ransomware", h.GetRansomware)
	r.Get("/api/v1/kev/{cveId}", h.GetKEVEntry)

	// EPSS endpoints — P1-07 (literal routes before wildcard!)
	if epssH != nil {
		r.Get("/api/v2/epss/top", epssH.GetEPSSTop)
		r.Get("/api/v2/epss/distribution", epssH.GetEPSSDistribution)
		r.Get("/api/v2/epss/{cveId}", epssH.GetEPSSByCVE) // wildcard MUST come last
		// /api/v1 aliases
		r.Get("/api/v1/epss/top", epssH.GetEPSSTop)
		r.Get("/api/v1/epss/distribution", epssH.GetEPSSDistribution)
		r.Get("/api/v1/epss/{cveId}", epssH.GetEPSSByCVE)
	}

	// /api/v1/cve/* alias — CVE handler also registers these but we add them here too
	// (they'll be handled via the CVE router mounted in main.go)

	// Internal endpoints (not exposed via gateway)
	r.Post("/internal/kev/sync", h.TriggerSync)
	r.Get("/internal/kev/ids", h.GetAllIDs)

	// CWE searchable list endpoint — P1-06
	// NOTE: /api/v2/cwe/{id} (detail) served by taxonomy handler (MongoDB)
	if cweH != nil {
		r.Get("/api/v2/cwe", cweH.ListCWE)
		r.Get("/api/v2/cwe/{id}", cweH.GetCWEByID) // detail by CWE-ID — P1-06
	}

	// Vendor endpoints — P1-08
	// Important: literal /browse must come BEFORE wildcard /{vendor}
	if vendorH != nil {
		r.Get("/api/v2/vendors", vendorH.ListVendors)               // list with CVE counts
		r.Get("/api/v2/browse", vendorH.Browse)                     // vendor summary (no path param)
		r.Get("/api/v2/browse/{vendor}", vendorH.BrowseVendor)       // products for vendor
		r.Get("/api/v2/browse/{vendor}/{product}", vendorH.BrowseProduct) // CVEs for vendor+product
	}

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
