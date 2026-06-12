// Package router — router.go
// Chi router chính của OpenVulnScan monolith.
// Mount tất cả HTTP handlers từ các service goroutines.
package router

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"

	"github.com/osv/apps/openvulnscan/internal/app"
	"github.com/osv/apps/openvulnscan/internal/middleware"
	"github.com/osv/apps/openvulnscan/internal/syslog"
)

//go:embed swagger-ui.html
var swaggerUIHTML []byte

// openAPIYAML is embedded from the local copy (synced from api/openapi.yaml).
//go:embed openapi.yaml
var openAPIYAML []byte

// New tạo chi.Router với tất cả routes đã mount.
// Gọi sau khi tất cả service runners đã Start().
func New(a *app.App) http.Handler {
	r := chi.NewRouter()

	// ── Global middleware ──────────────────────────────────────────────────
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(corsMiddleware)
	r.Use(requestLogger)

	// T17 Security hardening middleware
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.MaxBodySize(1 << 20)) // 1MB body limit

	// Rate limiter: 60 req/min per IP (1 req/s average)
	rl := middleware.NewRateLimiter(60, time.Minute)
	r.Use(middleware.RateLimit(rl))

	// Audit log for write operations
	r.Use(middleware.AuditLog(log.Logger))

	// Auth middleware dùng gRPC client đến auth runner
	authMW := middleware.NewAuthMiddleware(a.AuthClient)

	// ── Public routes (không cần auth) ────────────────────────────────────

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "openvulnscan",
		})
	})

	// Readiness — kiểm tra registry
	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		statuses := a.Registry.Status()
		all := true
		for _, s := range statuses {
			if s != "running" && s != "idle" {
				all = false
				break
			}
		}
		if all {
			writeJSON(w, http.StatusOK, statuses)
		} else {
			writeJSON(w, http.StatusServiceUnavailable, statuses)
		}
	})

	// Auth routes (login, register, OAuth) — public
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/login", a.HandleLogin)
		r.Post("/register", a.HandleRegister)
		r.Post("/logout", authMW.RequireAuth(http.HandlerFunc(a.HandleLogout)).ServeHTTP)
		r.Post("/refresh", a.HandleRefresh)
		r.Get("/google", a.HandleGoogleLogin)
		r.Get("/google/callback", a.HandleGoogleCallback)
	})

	// Agent download — public (agent tự download script)
	r.Get("/agent/download", a.HandleAgentDownload)
	r.Post("/agent/report", a.HandleAgentReport)

	// T19 — API Documentation (Swagger UI + OpenAPI YAML)
	r.Get("/api/v1/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(swaggerUIHTML) //nolint:errcheck
	})
	r.Get("/api/v1/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.WriteHeader(http.StatusOK)
		w.Write(openAPIYAML) //nolint:errcheck
	})

	// ── Protected routes (cần JWT) ─────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(authMW.RequireAuth)

		// Mount scan-service HTTP handler nguyên si (/api/v1/scans/*)
		if a.ScanRunner != nil && a.ScanRunner.HTTPHandler != nil {
			r.Mount("/", a.ScanRunner.HTTPHandler)
		}

		// Mount vulnerability-service HTTP handler (/cve/*)
		if a.VulnRunner != nil && a.VulnRunner.HTTPHandler != nil {
			r.Mount("/", a.VulnRunner.HTTPHandler)
		}

		// T07 — Mount query-service browse handler (/browse/, /vendors/search, /search/*)
		if a.QueryRunner != nil && a.QueryRunner.HTTPHandler != nil {
			r.Mount("/", a.QueryRunner.HTTPHandler)
		}

		// T10 — Mount product-service HTTP handler (/api/v1/products, /api/v1/engagements)
		if a.ProductRunner != nil && a.ProductRunner.HTTPHandler != nil {
			r.Mount("/", a.ProductRunner.HTTPHandler)
		}

		// T11 (notification) — Mount notification webhooks handler
		if a.NotificationRunner != nil && a.NotificationRunner.HTTPHandler != nil {
			r.Mount("/", a.NotificationRunner.HTTPHandler)
		}

		// T12 — Mount report-service HTTP handler (/api/v1/reports, /api/v1/scans/{id}/report)
		if a.ReportRunner != nil && a.ReportRunner.HTTPHandler != nil {
			r.Mount("/", a.ReportRunner.HTTPHandler)
		}

		// T11 (ingestion) — Mount ingestion status handler
		if a.IngestionRunner != nil && a.IngestionRunner.HTTPHandler != nil {
			r.Mount("/", a.IngestionRunner.HTTPHandler)
		}

		// Finding routes (thin HTTP wrapper over gRPC)
		r.Route("/api/v1/findings", func(r chi.Router) {
			r.Get("/", a.HandleListFindings)
			r.Get("/{id}", a.HandleGetFinding)
			r.Patch("/{id}/status", a.HandleUpdateFindingStatus)
			r.Post("/{id}/accept-risk", a.HandleAcceptRisk)
			r.Get("/summary", a.HandleFindingsSummary)
		})

		// Agent report management
		r.Route("/api/v1/agent", func(r chi.Router) {
			r.Get("/reports", a.HandleListAgentReports)
			r.Get("/reports/{id}", a.HandleGetAgentReport)
		})

		// T15 — Dashboard routes (aggregated stats from Postgres)
		mountDashboardRoutes(r, a)

		// T14 — SIEM config routes
		r.Route("/api/v1/siem", func(r chi.Router) {
			r.Get("/config", func(w http.ResponseWriter, r *http.Request) {
				if a.SyslogChannel == nil {
					writeJSON(w, http.StatusOK, map[string]interface{}{"enabled": false})
					return
				}
				cfg := a.SyslogChannel.GetConfig()
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"host": cfg.Host, "port": cfg.Port,
					"protocol": cfg.Protocol, "enabled": cfg.Enabled,
				})
			})
			r.Post("/config", func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Host     string `json:"host"`
					Port     int    `json:"port"`
					Protocol string `json:"protocol"`
					Enabled  bool   `json:"enabled"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
					return
				}
				if a.SyslogChannel != nil {
					a.SyslogChannel.UpdateConfig(syslog.Config{
						Host: req.Host, Port: req.Port,
						Protocol: req.Protocol, Enabled: req.Enabled, Facility: 16,
					})
				}
				writeJSON(w, http.StatusOK, map[string]string{"message": "siem config updated"})
			})
			r.Post("/test", func(w http.ResponseWriter, r *http.Request) {
				if a.SyslogChannel == nil {
					writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "syslog channel not configured"})
					return
				}
				if err := a.SyslogChannel.TestConnectivity(r.Context()); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]string{
						"error": err.Error(), "message": "connectivity test failed",
					})
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{"message": "connectivity test passed"})
			})
		})

		// Admin routes
		r.Group(func(r chi.Router) {
			r.Use(authMW.RequireAdmin)
			r.Get("/api/v1/admin/status", func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, http.StatusOK, a.Registry.Status())
			})
			r.Get("/api/v1/admin/services", func(w http.ResponseWriter, r *http.Request) {
				results := a.Registry.HealthAll(r.Context())
				health := make(map[string]string)
				for svc, err := range results {
					if err != nil {
						health[svc] = "unhealthy: " + err.Error()
					} else {
						health[svc] = "healthy"
					}
				}
				writeJSON(w, http.StatusOK, health)
			})
		})
	})

	return r
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote", r.RemoteAddr).
			Msg("request")
		next.ServeHTTP(w, r)
	})
}
