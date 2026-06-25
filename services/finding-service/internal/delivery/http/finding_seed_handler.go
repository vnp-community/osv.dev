// Package http — finding_seed_handler.go
// SEED-003: Handlers for finding bulk-create and import.
// Routes (literal paths BEFORE /{id} wildcard):
//   POST /api/v2/findings/bulk-create → BulkCreateFindings
//   POST /api/v2/findings/import      → ImportFindings
package http

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	findingbulk "github.com/osv/finding-service/internal/usecase/findingbulk"
	"github.com/osv/finding-service/internal/domain/finding"
	"github.com/osv/finding-service/internal/domain/repository"
)

// FindingSeedHandler handles SEED-003 bulk create and import endpoints.
type FindingSeedHandler struct {
	bulkUC *findingbulk.UseCase
	log    zerolog.Logger
}

// NewFindingSeedHandler creates a FindingSeedHandler.
func NewFindingSeedHandler(
	findingRepo    finding.Repository,
	testRepo       repository.TestRepository,
	engagementRepo repository.EngagementRepository,
	log            zerolog.Logger,
) *FindingSeedHandler {
	return &FindingSeedHandler{
		bulkUC: findingbulk.New(findingRepo, testRepo, engagementRepo),
		log:    log,
	}
}

// BulkCreateFindings handles POST /api/v2/findings/bulk-create
// Creates multiple findings for a given test, with dedup and severity filtering.
// Returns 207 Multi-Status with per-item results.
func (h *FindingSeedHandler) BulkCreateFindings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TestID              string                            `json:"test_id"`
		Findings            []findingbulk.FindingCreateInput `json:"findings"`
		AutoCloseDuplicates bool                             `json:"auto_close_duplicates"`
		AutoEnrichCVE       bool                             `json:"auto_enrich_cve"`
		MinimumSeverity     string                           `json:"minimum_severity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	testID, err := uuid.Parse(req.TestID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid test_id")
		return
	}

	if len(req.Findings) == 0 {
		respondError(w, http.StatusBadRequest, "findings list is required")
		return
	}

	opts := findingbulk.BulkCreateOptions{
		AutoCloseDuplicates: req.AutoCloseDuplicates,
		AutoEnrichCVE:       req.AutoEnrichCVE,
		ComputeSLA:          true,
		MinimumSeverity:     req.MinimumSeverity,
	}

	result, err := h.bulkUC.BulkCreate(r.Context(), testID, req.Findings, opts)
	if err != nil {
		h.log.Error().Err(err).Str("test_id", req.TestID).Msg("BulkCreateFindings failed")
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusMultiStatus, result)
}

// ImportFindings handles POST /api/v2/findings/import (multipart)
// Supports JSON array or CSV file. Field "format" = "json" | "csv".
// Max file size: 10MB. Returns 413 if exceeded.
func (h *FindingSeedHandler) ImportFindings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondError(w, http.StatusRequestEntityTooLarge, "file exceeds 10MB limit")
		return
	}

	testIDStr := r.FormValue("test_id")
	testID, err := uuid.Parse(testIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid test_id")
		return
	}

	format := r.FormValue("format")
	if format == "" {
		format = "json"
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	opts := findingbulk.BulkCreateOptions{
		AutoCloseDuplicates: r.FormValue("auto_close_duplicates") == "true",
		MinimumSeverity:     r.FormValue("minimum_severity"),
		ComputeSLA:          true,
	}

	var result *findingbulk.ImportOutput
	switch format {
	case "csv":
		result, err = h.bulkUC.ImportFromCSV(r.Context(), testID, file, opts)
	default: // "json"
		result, err = h.bulkUC.ImportFromJSON(r.Context(), testID, file, opts)
	}
	if err != nil {
		respondError(w, http.StatusBadRequest, "parse error: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}
