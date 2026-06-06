// Package handler provides the HTTP REST API for the scan orchestrator.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/defectdojo/scan-orchestrator/internal/domain/scan"
	import_uc "github.com/defectdojo/scan-orchestrator/internal/usecase/import"
	infparser "github.com/defectdojo/scan-orchestrator/internal/infrastructure/parser"
)

const maxUploadBytes = 100 << 20 // 100 MB

// ImportHandler handles POST /api/v2/import-scan.
type ImportHandler struct {
	importUC      *import_uc.ImportScanUseCase
	parserFactory *infparser.Factory
	log           zerolog.Logger
}

func NewImportHandler(uc *import_uc.ImportScanUseCase, factory *infparser.Factory, log zerolog.Logger) *ImportHandler {
	return &ImportHandler{importUC: uc, parserFactory: factory, log: log}
}

// HandleImportScan processes multipart form scan uploads.
func (h *ImportHandler) HandleImportScan(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not parse form: " + err.Error()})
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file field required"})
		return
	}
	defer file.Close()

	opts := scan.ImportOptions{
		ScanType:                  r.FormValue("scan_type"),
		ProductName:               r.FormValue("product_name"),
		ProductTypeName:           r.FormValue("product_type_name"),
		EngagementName:            r.FormValue("engagement_name"),
		TestTitle:                 r.FormValue("test_title"),
		MinimumSeverity:           formOr(r, "minimum_severity", "Info"),
		Active:                    formBool(r, "active", true),
		Verified:                  formBool(r, "verified", false),
		CloseOldFindings:          formBool(r, "close_old_findings", false),
		DeduplicationOnEngagement: formBool(r, "deduplication_on_engagement", true),
		Version:                   r.FormValue("version"),
		BuildID:                   r.FormValue("build_id"),
		CommitHash:                r.FormValue("commit_hash"),
		BranchTag:                 r.FormValue("branch_tag"),
		RequestorUserID:           r.Header.Get("X-User-ID"),
	}

	if opts.ScanType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scan_type is required"})
		return
	}

	result, err := h.importUC.Execute(r.Context(), import_uc.ImportScanInput{
		ScanType: opts.ScanType,
		File:     file,
		Options:  opts,
	})
	if err != nil {
		h.log.Error().Err(err).Str("scan_type", opts.ScanType).Msg("import scan failed")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// HandleListParsers returns the list of all registered scan types.
func (h *ImportHandler) HandleListParsers(w http.ResponseWriter, r *http.Request) {
	types := h.parserFactory.ListScanTypes()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":    len(types),
		"results": types,
	})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func formOr(r *http.Request, key, fallback string) string {
	if v := r.FormValue(key); v != "" {
		return v
	}
	return fallback
}

func formBool(r *http.Request, key string, defaultVal bool) bool {
	v := r.FormValue(key)
	switch v {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	}
	return defaultVal
}
