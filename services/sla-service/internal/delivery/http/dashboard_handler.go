// Package http provides dashboard and violations HTTP handlers for sla-service.
package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/osv/sla-service/internal/usecase/dashboard"
)

// DashboardHandler handles GET /api/v2/sla-dashboard.
type DashboardHandler struct {
	uc *dashboard.SLADashboardUseCase
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(uc *dashboard.SLADashboardUseCase) *DashboardHandler {
	return &DashboardHandler{uc: uc}
}

// Handle handles GET /api/v2/sla-dashboard?product_id=<uuid>
func (h *DashboardHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var productID *uuid.UUID
	if pidStr := r.URL.Query().Get("product_id"); pidStr != "" {
		pid, err := uuid.Parse(pidStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiErr("invalid product_id"))
			return
		}
		productID = &pid
	}

	out, err := h.uc.Execute(r.Context(), productID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiErr("failed to compute dashboard"))
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ─── ViolationsHandler ────────────────────────────────────────────────────────

// ViolationsHandler handles GET /api/v2/sla-violations.
type ViolationsHandler struct {
	repo dashboard.ViolationRepository
}

// NewViolationsHandler creates a new ViolationsHandler.
func NewViolationsHandler(r dashboard.ViolationRepository) *ViolationsHandler {
	return &ViolationsHandler{repo: r}
}

// List handles GET /api/v2/sla-violations?severity=High&days_overdue_min=1
func (h *ViolationsHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := dashboard.ViolationFilter{
		Limit:  intOr(r.URL.Query().Get("limit"), 100),
		Offset: intOr(r.URL.Query().Get("offset"), 0),
	}
	if sev := r.URL.Query().Get("severity"); sev != "" {
		filter.Severity = &sev
	}
	if min := r.URL.Query().Get("days_overdue_min"); min != "" {
		n := intOr(min, 0)
		filter.MinDaysOverdue = n
	}

	violations, total, err := h.repo.ListViolations(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiErr("failed to list violations"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":   total,
		"results": violations,
	})
}

// ListByProduct handles GET /api/v2/sla-violations/{product_id}
func (h *ViolationsHandler) ListByProduct(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(r.PathValue("product_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid product_id"))
		return
	}

	filter := dashboard.ViolationFilter{
		ProductID: &productID,
		Limit:     intOr(r.URL.Query().Get("limit"), 100),
		Offset:    intOr(r.URL.Query().Get("offset"), 0),
	}
	if sev := r.URL.Query().Get("severity"); sev != "" {
		filter.Severity = &sev
	}

	violations, total, err := h.repo.ListViolations(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiErr("failed to list violations"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":   total,
		"results": violations,
	})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body) //nolint:errcheck
}

func apiErr(msg string) map[string]string {
	return map[string]string{"detail": msg}
}

func intOr(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}
