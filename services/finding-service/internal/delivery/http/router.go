// Package http — router.go
// NewRouter builds the chi router for the finding-service HTTP server.
// Runs on port 8085 alongside the existing gRPC server.
//
// Routes:
//   GET  /health                                      → Health check
//   GET  /api/v1/findings                             → FindingHandler.List
//   GET  /api/v1/findings/{id}                        → FindingHandler.Get
//   PATCH /api/v1/findings/{id}                       → FindingHandler.PatchFinding
//   PUT  /api/v1/findings/{id}/close                  → FindingHandler.Close
//   PUT  /api/v1/findings/{id}/reopen                 → FindingHandler.Reopen
//   PUT  /api/v1/findings/{id}/false-positive         → FindingHandler.MarkFalsePositive
//   PUT  /api/v1/findings/{id}/risk-accepted          → FindingHandler.AcceptRisk
//   POST /api/v1/findings/bulk/close                  → BulkHandler.BulkClose
//   POST /api/v1/findings/bulk/reopen                 → BulkHandler.BulkReopen
//   POST /api/v1/findings/bulk/assign                 → BulkHandler.BulkAssign
//   POST /api/v2/findings/bulk-create                 → FindingSeedHandler.BulkCreateFindings (SEED-003)
//   POST /api/v2/findings/import                      → FindingSeedHandler.ImportFindings (SEED-003)
//   POST /api/v2/findings/bulk                        → BulkHandler.BulkUpdate
//   GET  /api/v2/findings/{id}/notes                  → NoteHandler.ListNotes
//   POST /api/v2/findings/{id}/notes                  → NoteHandler.AddNote
//   POST /api/v2/product-types/bulk                   → ProductSeedHandler.BulkCreateProductTypes (SEED-002)
//   POST /api/v2/products/bulk                        → ProductSeedHandler.BulkCreateProducts (SEED-002)
//   POST /api/v2/products/import                      → ProductSeedHandler.ImportProducts (SEED-002)
//   POST /api/v2/products/{id}/seed                   → ProductSeedHandler.SeedProduct (SEED-002)
//   GET  /api/v2/engagements                          → EngagementHandler.List
//   POST /api/v2/engagements                          → EngagementHandler.Create
//   GET  /api/v2/engagements/{id}                     → EngagementHandler.Get
//   POST /api/v2/engagements/{id}/close               → EngagementHandler.Close
//   POST /api/v2/engagements/{id}/reopen              → EngagementHandler.Reopen
//   GET  /api/v2/tests                                → TestHandler.List
//   POST /api/v2/tests                                → TestHandler.Create
//   GET  /api/v2/tests/{id}                           → TestHandler.Get
//   DELETE /api/v2/tests/{id}                         → TestHandler.Delete
package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

// NewRouter builds and returns the chi router for the finding-service HTTP server.
func NewRouter(
	handler *FindingHandler,
	bulk *BulkHandler,
	note *NoteHandler,
	engagement *EngagementHandler,
	test *TestHandler,
	member *MemberHandler,
	tool *ToolHandler,
	report *ReportHandler,
	riskAcceptance *RiskAcceptanceHandler,
	internal *InternalHandler,
	sla *SLAHandler,
	product *ProductHandler,
	productSeed *ProductSeedHandler, // SEED-002
	findingSeed *FindingSeedHandler, // SEED-003
	findingGroup *FindingGroupHandler,
	log zerolog.Logger,
) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "finding-service",
		})
	})

	// ── Finding endpoints (v1 — state transitions + list/get) ──
	r.Route("/api/v1/findings", func(r chi.Router) {
		r.Get("/", handler.List)
		// TASK-004 FIX: bulk literal paths MUST be registered BEFORE /{id} wildcard
		if bulk != nil {
			r.Post("/bulk/close",  bulk.BulkClose)   // gateway path: /bulk/close  (slash)
			r.Post("/bulk/reopen", bulk.BulkReopen)  // gateway path: /bulk/reopen (slash)
			r.Post("/bulk/assign", bulk.BulkAssign)  // gateway path: /bulk/assign (slash)
		}
		// /stats MUST be before /{id} to avoid "stats" being matched as an ID
		if internal != nil {
			r.Get("/stats", internal.GetFindingStats) // severity counts for dashboard BFF
		}
		r.Get("/{id}", handler.Get)
		r.Patch("/{id}", handler.PatchFinding)   // TASK-004 FIX: PATCH /api/v1/findings/{id}
		if note != nil {
			r.Get("/{id}/notes", note.ListNotes)
			r.Post("/{id}/notes", note.AddNote)
		}
		r.Put("/{id}/close", handler.Close)
		r.Put("/{id}/reopen", handler.Reopen)
		r.Put("/{id}/false-positive", handler.MarkFalsePositive)
		r.Put("/{id}/risk-accepted", handler.AcceptRisk)
		// Notes are accessible from both v1 and v2 paths — handled above (93-95)
	})

	// ── Product v1 compatibility endpoints ──
	if product != nil {
		r.Route("/api/v1/products", func(r chi.Router) {
			r.Get("/", product.List)
			r.Post("/", product.Create)
			r.Get("/grades", product.GetProductGrades)
			r.Get("/{id}", product.Get)
			r.Put("/{id}", product.Update)
			r.Patch("/{id}", product.Update)    // CR-003: PATCH alias (same logic as PUT)
			r.Delete("/{id}", product.Delete)
			// v1 sub-resources: engagements by product
			if engagement != nil {
				r.Get("/{id}/engagements", engagement.List)
				r.Post("/{id}/engagements", engagement.CreateForProduct) // CR-003
			}
		})
	} else if internal != nil {
		// When ProductHandler is nil, register /grades via InternalHandler
		// to feed the gateway-service DashboardHandler.
		r.Get("/api/v1/products/grades", internal.GetProductGradesPublic)
	}

	// ── Engagement v1 compatibility endpoints ──
	if engagement != nil {
		r.Route("/api/v1/engagements", func(r chi.Router) {
			r.Get("/", engagement.List)
			r.Post("/", engagement.Create)
			r.Get("/{id}", engagement.Get)
			r.Post("/{id}/close", engagement.Close)
			r.Post("/{id}/reopen", engagement.Reopen)
			// v1 sub-resource: tests by engagement
			if test != nil {
				r.Get("/{id}/tests", test.List)
			}
		})
	}

	// ── ProductType v2 ──
	// SEED-002: literal /product-types/bulk MUST be registered before any /{id} patterns
	if productSeed != nil {
		r.Post("/api/v2/product-types/bulk", productSeed.BulkCreateProductTypes)
	}

	// ── Finding v2 — bulk ops + notes ──
	r.Route("/api/v2/findings", func(r chi.Router) {
		// SEED-003: literal paths BEFORE /{id} wildcard
		if findingSeed != nil {
			r.Post("/bulk-create", findingSeed.BulkCreateFindings)
			r.Post("/import", findingSeed.ImportFindings)
		}
		if bulk != nil {
			r.Post("/bulk", bulk.BulkUpdate)
			r.Post("/bulk_reopen", bulk.BulkReopen)
			r.Post("/bulk_assign", bulk.BulkAssign)
			r.Get("/stats", bulk.GetStats)
			r.Get("/severity_count", bulk.GetStats) // alias
		}
		r.Route("/{id}", func(r chi.Router) {
			if note != nil {
				r.Get("/notes", note.ListNotes)
				r.Post("/notes", note.AddNote)
			}
		})
	})

	// ── Product v2 ──
	if product != nil {
		r.Get("/api/v2/products", product.List)
		r.Post("/api/v2/products", product.Create)
		r.Get("/api/v2/products/grades", product.GetProductGrades)
	}
	r.Route("/api/v2/products", func(r chi.Router) {
		// SEED-002: literal paths BEFORE /{id}
		if productSeed != nil {
			r.Post("/bulk",   productSeed.BulkCreateProducts) // BEFORE /{id}
			r.Post("/import", productSeed.ImportProducts)      // BEFORE /{id}
		}
		if product != nil {
			r.Get("/{id}", product.Get)
			r.Put("/{id}", product.Update)
			r.Delete("/{id}", product.Delete)
		}
		// SEED-002: seed endpoint (after literal paths)
		if productSeed != nil {
			r.Post("/{id}/seed", productSeed.SeedProduct)
		}
	})

	// ── Engagement endpoints ──
	if engagement != nil {
		r.Get("/api/v2/engagements", engagement.List)
		r.Post("/api/v2/engagements", engagement.Create)
		r.Route("/api/v2/engagements", func(r chi.Router) {
			r.Get("/{id}", engagement.Get)
			r.Post("/{id}/close", engagement.Close)
			r.Post("/{id}/reopen", engagement.Reopen)
		})
	}

	// ── Test endpoints ──
	if test != nil {
		r.Get("/api/v2/tests", test.List)
		r.Post("/api/v2/tests", test.Create)
		r.Route("/api/v2/tests", func(r chi.Router) {
			r.Get("/{id}", test.Get)
			r.Delete("/{id}", test.Delete)
		})
	}

	// ── Member endpoints ──
	if member != nil {
		r.Route("/api/v2/members", func(r chi.Router) {
			// r.Get("/", member.ListByProduct)
			r.Post("/", member.Add)
			r.Delete("/{id}", member.Remove)
			// r.Put("/{id}", member.UpdateRole)
		})
	}

	// ── Tool configuration endpoints ──
	if tool != nil {
		r.Route("/api/v2/tool-configurations", func(r chi.Router) {
			r.Get("/", tool.List)
			r.Post("/", tool.Create)
			r.Get("/{id}", tool.Get)
			r.Put("/{id}", tool.Update)
			r.Delete("/{id}", tool.Delete)
		})
	}

	// ── Report endpoints ──
	if report != nil {
		// TASK-010: v1 compatibility routes (gateway forwards /api/v1/reports/* → finding-service)
		// IMPORTANT: literal paths /templates and /{id}/download MUST be before /{id}
		r.Get("/api/v1/reports/templates", report.GetTemplates)       // literal BEFORE /{id}
		r.Get("/api/v1/reports/{id}/download", report.Download)       // literal BEFORE /{id}
		r.Get("/api/v1/reports", report.List)
		r.Post("/api/v1/reports", report.Create)
		r.Get("/api/v1/reports/{id}", report.Get)
		r.Delete("/api/v1/reports/{id}", report.Delete)

		// v2 routes (unchanged)
		r.Route("/api/v2/reports", func(r chi.Router) {
			// CR-010: /templates MUST come before /{id} to avoid routing conflict
			r.Get("/templates", report.GetTemplates) // ← CR-010: literal path first
			r.Post("/generate", report.Create)
			r.Get("/", report.List)
			r.Get("/{id}", report.Get)
			r.Get("/{id}/download", report.Download)
			r.Delete("/{id}", report.Delete)
		})
	}

	// ── Risk acceptance endpoints ──
	// TASK-009: Add v1 compat routes + List/Get/Delete handlers
	if riskAcceptance != nil {
		// v1 compatibility routes (gateway forwards /api/v1/risk-acceptances → finding-service)
		r.Get("/api/v1/risk-acceptances", riskAcceptance.List)
		r.Post("/api/v1/risk-acceptances", riskAcceptance.Create)
		r.Delete("/api/v1/risk-acceptances/{id}", riskAcceptance.Delete)

		r.Route("/api/v2/risk-acceptances", func(r chi.Router) {
			r.Get("/", riskAcceptance.List)
			r.Post("/", riskAcceptance.Create)
			r.Get("/{id}", riskAcceptance.Get)
			r.Delete("/{id}", riskAcceptance.Delete)
		})
	}

	// ── Finding group endpoints ──
	if findingGroup != nil {
		r.Post("/api/v2/finding-groups", findingGroup.Create)
	}

	// ── Internal endpoints (for BFF consumption) ──
	// Register directly on chi router with full absolute paths to avoid
	// chi-Mount + http.ServeMux RoutePath-vs-URL.Path incompatibility.
	if internal != nil {
		r.Get("/internal/stats", internal.GetStats)
		r.Get("/internal/risk-trend", internal.GetRiskTrend)
		r.Get("/internal/product-grades", internal.GetProductGrades)
		r.Get("/internal/sla-breaches", internal.GetSLABreaches)
		r.Post("/internal/findings/count-by-cve-ids", internal.CountByCVEIds)
		if sla != nil {
			r.Get("/internal/sla-dashboard", sla.GetSLADashboard)
		}
	}

	return r
}

// NewMinimalRouter creates a router with only finding endpoints (for testing/minimal deploys).
func NewMinimalRouter(handler *FindingHandler, log zerolog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "finding-service",
		})
	})

	r.Route("/api/v1/findings", func(r chi.Router) {
		r.Get("/", handler.List)
		r.Get("/{id}", handler.Get)
		r.Put("/{id}/close", handler.Close)
		r.Put("/{id}/reopen", handler.Reopen)
		r.Put("/{id}/false-positive", handler.MarkFalsePositive)
		r.Put("/{id}/risk-accepted", handler.AcceptRisk)
	})

	return r
}
