// Package http provides the HTTP router for audit-service.
package http

import (
	"net/http"
)

// NewAuditRouter creates the HTTP mux for audit-service.
func NewAuditRouter(h *AuditHandler) http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"audit-service"}`)) //nolint:errcheck
	})

	// TASK-013: Audit log endpoints
	// v1 route — gateway forwards GET /api/v1/audit-log → audit-service
	mux.HandleFunc("GET /api/v1/audit-log", h.ListAuditLog)

	// v2 routes — extended paths with pagination + filtering
	mux.HandleFunc("GET /api/v2/audit-log", h.ListAuditLog)

	// Legacy path: /audit-log (for backward compat with internal tools)
	mux.HandleFunc("GET /audit-log", h.ListAuditLog)

	return mux
}
