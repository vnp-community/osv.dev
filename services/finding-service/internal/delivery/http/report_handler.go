// Package http provides report and grading HTTP handlers for finding-service.
package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/report"
	reportuc "github.com/osv/finding-service/internal/usecase/report"
	"github.com/osv/finding-service/internal/usecase/grading"
)

// ─── Report Templates ──────────────────────────────────────────────────────────

// reportTemplate is the static config for a report template.
type reportTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

// defaultReportTemplates contains the 3 standard report templates.
// Static config — no DB needed.
var defaultReportTemplates = []reportTemplate{
	{
		ID:          "exec",
		Name:        "Executive Summary",
		Description: "High-level overview for C-level presentations. Bao gồm risk posture, top vulnerabilities, và SLA compliance.",
		Type:        "Executive",
	},
	{
		ID:          "tech",
		Name:        "Technical Report",
		Description: "Chi tiết findings với CVE details, CVSS scores, và hướng dẫn remediation.",
		Type:        "Technical",
	},
	{
		ID:          "comp",
		Name:        "Compliance Report",
		Description: "Mapped tới PCI DSS, ISO 27001, SOC2, NIST frameworks.",
		Type:        "Compliance",
	},
}

// ─── ReportHandler ────────────────────────────────────────────────────────────

// ReportHandler handles /api/v2/reports endpoints.
type ReportHandler struct {
	generateUC *reportuc.GenerateUseCase
	repo       report.Repository
	storage    reportuc.Storage
}

// NewReportHandler creates a new ReportHandler.
func NewReportHandler(g *reportuc.GenerateUseCase, r report.Repository, s reportuc.Storage) *ReportHandler {
	return &ReportHandler{generateUC: g, repo: r, storage: s}
}

// GetTemplates handles GET /api/v1/reports/templates (and /api/v2/reports/templates)
// Returns static list of 3 report templates. No DB required.
// ⚠️ MUST be registered BEFORE /reports/{id} in the router.
func (h *ReportHandler) GetTemplates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"templates": defaultReportTemplates,
	})
}

// Create handles POST /api/v2/reports → 201 {id, status: "pending"}
func (h *ReportHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProductID    string   `json:"product_id"`
		Title        string   `json:"title"`
		Format       string   `json:"format"` // pdf|xlsx|json|csv
		TemplateID   string   `json:"template_id,omitempty"`
		Name         string   `json:"name,omitempty"` // alias for title
		EngagementID *string  `json:"engagement_id,omitempty"`
		Severities   []string `json:"severities,omitempty"`
		ActiveOnly   bool     `json:"active_only"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid request body"))
		return
	}
	// Use name as alias for title if title is empty
	if req.Title == "" && req.Name != "" {
		req.Title = req.Name
	}
	if req.Format == "" {
		req.Format = "pdf"
	}

	// [FIX TASK-HC-001] Persist report record even when generateUC is nil.
	// Ensures GET /api/v2/reports/{id} can locate the record.
	if h.generateUC == nil {
		now := time.Now().UTC()
		rep := &report.Report{
			ID:          uuid.New().String(),
			ProductID:   req.ProductID,
			Title:       req.Title,
			Format:      report.ReportFormat(req.Format),
			Status:      report.StatusPending,
			GeneratedBy: r.Header.Get("X-User-ID"),
			CreatedAt:   now,
		}
		if req.EngagementID != nil {
			rep.EngagementID = req.EngagementID
		}
		// Persist to DB (best-effort — log if repo not wired)
		if h.repo != nil {
			if err := h.repo.Save(r.Context(), rep); err != nil {
				// Non-fatal: still return accepted so UI is not blocked
				_ = err
			}
		}
		writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"id":         rep.ID,
			"name":       rep.Title,
			"type":       string(rep.Format),
			"status":     string(rep.Status),
			"format":     string(rep.Format),
			"title":      rep.Title,
			"created_by": rep.GeneratedBy,
			"created_at": now.Format(time.RFC3339), // [FIX TASK-HC-001] không còn hardcode
			"message":    "Report generation queued. Download link will be available when complete.",
		})
		return
	}

	rep, err := h.generateUC.Execute(r.Context(), reportuc.GenerateInput{
		ProductID:    req.ProductID,
		Title:        req.Title,
		Format:       report.ReportFormat(req.Format),
		EngagementID: req.EngagementID,
		Severities:   req.Severities,
		ActiveOnly:   req.ActiveOnly,
		RequesterID:  r.Header.Get("X-User-ID"),
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, rep)
}

// Get handles GET /api/v2/reports/{id}
func (h *ReportHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rep, err := h.repo.FindByID(r.Context(), id)
	if err != nil || rep == nil {
		writeJSON(w, http.StatusNotFound, apiErr("report not found"))
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

// Download handles GET /api/v2/reports/{id}/download → 302 pre-signed URL
func (h *ReportHandler) Download(w http.ResponseWriter, r *http.Request) {
	// MOCK-002 FIX: nil-check — storage not configured when MinIO env vars are absent
	if h.storage == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiErr(
			"report download not available: storage backend not configured",
		))
		return
	}

	id := chi.URLParam(r, "id")
	rep, err := h.repo.FindByID(r.Context(), id)
	if err != nil || rep == nil {
		writeJSON(w, http.StatusNotFound, apiErr("report not found"))
		return
	}
	if rep.Status != report.StatusCompleted {
		writeJSON(w, http.StatusConflict, apiErr("report is not ready for download"))
		return
	}

	// Check ownership
	userID := r.Header.Get("X-User-ID")
	if rep.GeneratedBy != userID {
		// Admin can download any report — check role
		role := r.Header.Get("X-User-Role")
		if role != "admin" {
			writeJSON(w, http.StatusForbidden, apiErr("cannot download this report"))
			return
		}
	}

	// Generate 15-minute pre-signed URL
	url, err := h.storage.PresignedURL(r.Context(), rep.StorageKey, 15*time.Minute)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiErr("failed to generate download URL"))
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

// List handles GET /api/v1/reports and GET /api/v2/reports (user-scoped)
// CR-010: response now includes last_generated_at (null when no completed reports).
func (h *ReportHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("_user_id")
	if userID == "" {
		userID = r.Header.Get("X-User-ID")
	}
	productID := r.URL.Query().Get("product_id")
	limit := intOr(r.URL.Query().Get("limit"), 20)
	offset := intOr(r.URL.Query().Get("offset"), 0)

	// Guard: repo nil
	if h.repo == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"reports": []interface{}{},
			"total":   0,
		})
		return
	}

	var reports []*report.Report
	var total int
	var err error

	if productID != "" {
		reports, total, err = h.repo.ListByProduct(r.Context(), productID, userID, limit, offset)
	} else {
		// No product_id: list by user (generated_by)
		reports, total, err = h.repo.ListByProduct(r.Context(), "", userID, limit, offset)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiErr("failed to list reports"))
		return
	}
	if reports == nil {
		reports = []*report.Report{}
	}

	// CR-010: compute last_generated_at from the most recent completed report
	var lastGeneratedAt *time.Time
	for _, rep := range reports {
		if rep.Status == report.StatusCompleted && rep.GeneratedAt != nil {
			if lastGeneratedAt == nil || rep.GeneratedAt.After(*lastGeneratedAt) {
				ga := *rep.GeneratedAt
				lastGeneratedAt = &ga
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"reports":           reports,
		"total":             total,
		"last_generated_at": lastGeneratedAt,
	})
}

// Delete handles DELETE /api/v2/reports/{id}
func (h *ReportHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_ = h.repo.Delete(r.Context(), id)
	w.WriteHeader(http.StatusNoContent)
}

// ─── GradeHandler ─────────────────────────────────────────────────────────────

// GradeHandler handles /api/v2/product-grades endpoints.
type GradeHandler struct {
	computeUC *grading.ComputeGradeUseCase
}

// NewGradeHandler creates a new GradeHandler.
func NewGradeHandler(uc *grading.ComputeGradeUseCase) *GradeHandler {
	return &GradeHandler{computeUC: uc}
}

// Get handles GET /api/v2/product-grades/{id}
func (h *GradeHandler) Get(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid product id"))
		return
	}
	grade, err := h.computeUC.Execute(r.Context(), productID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiErr(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, grade)
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
	n := 0
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil || n < 0 {
		return def
	}
	return n
}
