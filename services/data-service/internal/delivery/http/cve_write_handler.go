// Package http — cve_write_handler.go
// SEED-004: HTTP handlers for custom CVE creation, triage, and bulk import.
//
// Routes (literal paths BEFORE wildcards):
//   POST /api/v1/cve/custom       → CreateCustomCVE
//   POST /api/v1/cve/bulk-triage  → BulkTriage
//   POST /api/v1/cve/import       → ImportCVEs (multipart, max 50MB)
//   PUT  /api/v1/cve/{id}/triage  → UpsertTriage
package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/osv/data-service/internal/domain/repository"
	cveingest "github.com/osv/data-service/internal/usecase/cveingest"
)

// CVEWriteHandler handles SEED-004 write endpoints for CVEs and triage.
type CVEWriteHandler struct {
	ingestUC *cveingest.UseCase
}

// NewCVEWriteHandler creates a CVEWriteHandler.
func NewCVEWriteHandler(cveRepo repository.CVERepository, triageRepo repository.TriageRepository) *CVEWriteHandler {
	return &CVEWriteHandler{
		ingestUC: cveingest.New(cveRepo, triageRepo),
	}
}

// CreateCustomCVE handles POST /api/v1/cve/custom
// Creates a custom/internal CVE record.
// Returns 201 on success, 409 on duplicate.
func (h *CVEWriteHandler) CreateCustomCVE(w http.ResponseWriter, r *http.Request) {
	var req cveingest.CustomCVEInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	cve, err := h.ingestUC.CreateCustomCVE(r.Context(), req)
	if err != nil {
		if isConflict(err.Error()) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, cve)
}

// UpsertTriage handles PUT /api/v1/cve/{id}/triage
// Creates or updates a triage decision for a CVE.
func (h *CVEWriteHandler) UpsertTriage(w http.ResponseWriter, r *http.Request) {
	cveID := chi.URLParam(r, "id")
	if cveID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cve id is required"})
		return
	}

	actorIDStr := r.Header.Get("X-User-ID")
	actorID, _ := uuid.Parse(actorIDStr)

	var req struct {
		Remarks       string   `json:"remarks"`
		Comments      string   `json:"comments"`
		Justification string   `json:"justification"`
		Response      []string `json:"response"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	record, err := h.ingestUC.UpsertTriage(r.Context(), actorID, cveID, req.Remarks, req.Comments, req.Justification, req.Response)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, record)
}

// BulkTriage handles POST /api/v1/cve/bulk-triage
// Applies the same triage decision to multiple CVEs. Returns 207 Multi-Status.
func (h *CVEWriteHandler) BulkTriage(w http.ResponseWriter, r *http.Request) {
	actorIDStr := r.Header.Get("X-User-ID")
	actorID, _ := uuid.Parse(actorIDStr)

	var req struct {
		CVEIDs        []string `json:"cve_ids"`
		Remarks       string   `json:"remarks"`
		Comments      string   `json:"comments"`
		Justification string   `json:"justification"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	result, err := h.ingestUC.BulkTriage(r.Context(), actorID, req.CVEIDs, req.Remarks, req.Comments, req.Justification)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusMultiStatus, result)
}

// ImportCVEs handles POST /api/v1/cve/import (multipart)
// Imports a JSON array of CustomCVEInput. Max file size: 50MB.
func (h *CVEWriteHandler) ImportCVEs(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20) // 50MB
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file exceeds 50MB limit"})
		return
	}

	overwrite := r.FormValue("overwrite") == "true"

	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file field is required"})
		return
	}
	defer file.Close()

	result, err := h.ingestUC.ImportCVEsFromJSON(r.Context(), file, overwrite)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"imported_count": result.ImportedCount,
		"skipped_count":  result.SkippedCount,
		"failed_count":   result.FailedCount,
		"results":        result.Results,
	})
}

// RegisterCVEWriteRoutes registers SEED-004 routes on the chi router.
// IMPORTANT: literal paths MUST be registered before wildcard /{id} routes.
func RegisterCVEWriteRoutes(r chi.Router, h *CVEWriteHandler) {
	// SEED-004: literal paths BEFORE /cve/{id}
	r.Post("/api/v1/cve/custom",      h.CreateCustomCVE)  // BEFORE /{id}
	r.Post("/api/v1/cve/bulk-triage", h.BulkTriage)       // BEFORE /{id}
	r.Post("/api/v1/cve/import",      h.ImportCVEs)        // BEFORE /{id}
	r.Put("/api/v1/cve/{id}/triage",  h.UpsertTriage)
}

func isConflict(msg string) bool {
	return len(msg) >= 9 && msg[:9] == "conflict:"
}
