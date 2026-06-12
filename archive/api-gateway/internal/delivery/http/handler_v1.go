// Package http — v1 router registration for the API gateway.
// This file augments the existing handler.go with new v1 routes.
package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"github.com/osv/api-gateway/internal/adapter/upstream"
	"github.com/osv/api-gateway/internal/adapter/grpcclient"
	"github.com/osv/api-gateway/internal/domain/entity"
	gatemw "github.com/osv/api-gateway/internal/delivery/http/middleware"
	pkghealth "github.com/osv/pkg/health"
	"github.com/osv/api-gateway/internal/usecase/dbsync"
	"github.com/osv/api-gateway/internal/usecase/report"
	"github.com/osv/api-gateway/internal/usecase/scan"
)

// CVEHandler extends the base Handler with v1 use cases.
type CVEHandler struct {
	upstream       *upstream.HTTPUpstreamClient
	cfg            Config
	log            zerolog.Logger

	// v1 use cases (nil if not configured)
	scanUC        *scan.UseCase
	dbsyncUC      *dbsync.UseCase
	reportUC      *report.UseCase
	sbomvexClient *grpcclient.SBOMVEXClient
}

// CVEHandlerOptions configures v1 use cases.
type CVEHandlerOptions struct {
	ScanUC        *scan.UseCase
	DBSyncUC      *dbsync.UseCase
	ReportUC      *report.UseCase
	SBOMVEXClient *grpcclient.SBOMVEXClient
}

// NewCVEHandler creates a CVEHandler.
func NewCVEHandler(cfg Config, opts CVEHandlerOptions, log zerolog.Logger) *CVEHandler {
	services := make(map[string]upstream.UpstreamConfig, len(cfg.UpstreamURLs))
	for name, url := range cfg.UpstreamURLs {
		services[name] = upstream.UpstreamConfig{URL: url, Timeout: 15 * time.Second}
	}
	return &CVEHandler{
		upstream:       upstream.NewHTTPUpstreamClient(services),
		cfg:            cfg,
		log:            log,
		scanUC:         opts.ScanUC,
		dbsyncUC:       opts.DBSyncUC,
		reportUC:       opts.ReportUC,
		sbomvexClient:  opts.SBOMVEXClient,
	}
}

// Health handles GET /health
func (h *CVEHandler) Health(w http.ResponseWriter, r *http.Request) {
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

// handleReadiness handles GET /health/ready — checks all gRPC upstreams
func (h *CVEHandler) handleReadiness(w http.ResponseWriter, r *http.Request) {
	// TODO: ping each gRPC upstream
	respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// handleLiveness handles GET /health/live — always 200
func (h *CVEHandler) handleLiveness(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

// Proxy handles /api/v2/* proxying
func (h *CVEHandler) Proxy(w http.ResponseWriter, r *http.Request) {
	route, ok := entity.FindGlobalCVERoute(r.URL.Path)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "no route"})
		return
	}
	h.upstream.Forward(r.Context(), w, r, route.Upstream) //nolint:errcheck
}

// NewCVERouter builds the full v1+v2 router.
func NewCVERouter(h *CVEHandler, log zerolog.Logger, rateLimiter *gatemw.IPRateLimiter) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(corsMiddleware(h.cfg.CORSOrigins))
	r.Use(requestLogger(log))

	if rateLimiter != nil {
		r.Use(rateLimiter.Middleware)
	}

	// Health endpoints (no auth, no rate limit concern)
	r.Get("/health", h.Health)
	r.Get("/health/ready", h.handleReadiness)
	r.Get("/health/live", h.handleLiveness)

	// v1 API — CVE Binary Tool endpoints
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(simpleAuthMiddleware(h.cfg.JWTSecret))

		// Scan
		r.Post("/scan/binary", h.handleBinaryScan)
		r.Post("/scan/package-list", h.handlePackageListScan)
		r.Post("/scan/merge", h.handleMergeReports)

		// DB
		r.Post("/db/sync", h.handleSyncAll)
		r.Post("/db/sync/{source}", h.handleSyncSource)
		r.Get("/db/status", h.handleDBStatus)
		r.Post("/db/import", h.handleImportDB)
		r.Get("/db/export", h.handleExportDB)

		// Report
		r.Post("/report", h.handleGenerateReport)
		r.Get("/report/formats", h.handleListFormats)

		// SBOM
		r.Post("/sbom/parse", h.HandleSBOMParse)
		r.Post("/sbom/generate", h.HandleSBOMGenerate)
		r.Post("/sbom/detect", h.HandleSBOMDetect)

		// VEX
		r.Post("/vex/parse", h.HandleVEXParse)
		r.Post("/vex/generate", h.HandleVEXGenerate)
	})

	// v2 API — GlobalCVE proxy (unchanged)
	r.Group(func(r chi.Router) {
		r.Use(simpleAuthMiddleware(h.cfg.JWTSecret))

		r.Get("/api/v2/cves", h.Proxy)
		r.Get("/api/v2/cves/{id}", h.Proxy)
		r.Get("/api/v2/kev", h.Proxy)
		r.Get("/api/v2/kev/check", h.Proxy)
		r.Get("/api/v2/kev/stats", h.Proxy)
		r.Get("/api/v2/kev/sync/status", h.Proxy)
		r.Get("/api/v2/kev/{cveId}", h.Proxy)
		r.Get("/api/v2/sync/status", h.Proxy)
		r.Post("/api/v2/webhooks", h.Proxy)
		r.Get("/api/v2/webhooks", h.Proxy)
		r.Delete("/api/v2/webhooks/{id}", h.Proxy)
		r.Post("/api/v2/sync/trigger", h.Proxy)
	})

	return r
}
