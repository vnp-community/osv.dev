package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/osv/scan-service/internal/domain/importdomain"
	importuc "github.com/osv/scan-service/internal/usecase/import"
)

type ImportHandler struct {
	uc  *importuc.ImportScanUseCase
	rep importuc.ImportHistoryRepository
	log zerolog.Logger
}

func NewImportHandler(uc *importuc.ImportScanUseCase, rep importuc.ImportHistoryRepository, log zerolog.Logger) *ImportHandler {
	return &ImportHandler{
		uc:  uc,
		rep: rep,
		log: log,
	}
}

// ReimportScan handles POST /api/v2/reimport-scan
// Expects multipart/form-data.
func (h *ImportHandler) ReimportScan(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100 MB max memory
		respondError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	scanType := r.FormValue("scan_type")
	if scanType == "" {
		respondError(w, http.StatusBadRequest, "scan_type is required")
		return
	}

	autoCreateCtx := parseBool(r.FormValue("auto_create_context"), false)
	
	var testIDPtr, engagementIDPtr, productIDPtr *string
	
	if t := r.FormValue("test"); t != "" {
		testIDPtr = &t
	}
	if e := r.FormValue("engagement"); e != "" {
		engagementIDPtr = &e
	}
	if p := r.FormValue("product"); p != "" {
		productIDPtr = &p
	}

	opts := &importdomain.ImportOptions{
		File:                         file,
		Filename:                     header.Filename,
		ScanType:                     scanType,
		TestID:                       testIDPtr,
		EngagementID:                 engagementIDPtr,
		ProductID:                    productIDPtr,
		RequestorUserID:              r.Header.Get("X-User-ID"), // Assuming injected by API Gateway
		AutoCreateContext:            autoCreateCtx,
		EngagementName:               r.FormValue("engagement_name"),
		EngagementType:               r.FormValue("engagement_type"),
		TestTitle:                    r.FormValue("test_title"),
		Version:                      r.FormValue("version"),
		BranchTag:                    r.FormValue("branch_tag"),
		BuildID:                      r.FormValue("build_id"),
		CommitHash:                   r.FormValue("commit_hash"),
		CloseOldFindings:             parseBool(r.FormValue("close_old_findings"), false),
		CloseOldFindingsProductScope: parseBool(r.FormValue("close_old_findings_product_scope"), false),
		DoNotReactivate:              parseBool(r.FormValue("do_not_reactivate"), false),
		ApplyTagsToFindings:          parseBool(r.FormValue("apply_tags_to_findings"), false),
		Tags:                         parseTags(r.FormValue("tags")),
		MinimumSeverity:              importdomain.Severity(r.FormValue("minimum_severity")),
	}

	result, err := h.uc.Execute(r.Context(), opts)
	if err != nil {
		h.log.Error().Err(err).Msg("ImportScanUseCase failed")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, result)
}

// GetTestImport handles GET /api/v2/test-imports/{id}
func (h *ImportHandler) GetTestImport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "id is required")
		return
	}

	testImport, err := h.rep.FindByID(r.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Str("id", id).Msg("GetTestImport failed")
		respondError(w, http.StatusNotFound, "not found")
		return
	}

	respondJSON(w, http.StatusOK, testImport)
}

// ── Helpers ──

func parseBool(val string, def bool) bool {
	if val == "" {
		return def
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return def
	}
	return b
}

func parseTags(val string) []string {
	if val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	var tags []string
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
