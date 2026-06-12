// Package http — report route handlers for the API gateway.
package http

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/osv/unified-gateway/internal/usecase/report"
	reporterpb "github.com/osv/proto/gen/go/reporter/v1"
)

// handleGenerateReport handles POST /api/v1/report
func (h *CVEHandler) handleGenerateReport(w http.ResponseWriter, r *http.Request) {
	type reportRequest struct {
		CVEData     []*reporterpb.ProductCVEs `json:"cve_data"`
		Formats     []string                  `json:"formats"`
		MinSeverity string                    `json:"min_severity"`
		MinScore    float32                   `json:"min_score"`
		Theme       string                    `json:"theme"`
		ScanTarget  string                    `json:"scan_target"`
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 50<<20))
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	var req reportRequest
	if err := json.Unmarshal(body, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if h.reportUC == nil {
		respondError(w, http.StatusServiceUnavailable, "report service not configured")
		return
	}

	result, err := h.reportUC.Execute(r.Context(), report.Input{
		CVEData:     req.CVEData,
		Formats:     req.Formats,
		MinSeverity: req.MinSeverity,
		MinScore:    req.MinScore,
		Theme:       req.Theme,
		ScanTarget:  req.ScanTarget,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	primaryFormat := "json"
	if len(req.Formats) > 0 {
		primaryFormat = req.Formats[0]
	}

	if data, ok := result.Reports[primaryFormat]; ok {
		w.Header().Set("Content-Type", contentTypeFor(primaryFormat))
		w.WriteHeader(http.StatusOK)
		w.Write(data) //nolint:errcheck
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_cves": result.TotalCVEs,
		"exit_code":  result.ExitCode,
	})
}

// handleListFormats handles GET /api/v1/report/formats
func (h *CVEHandler) handleListFormats(w http.ResponseWriter, r *http.Request) {
	if h.reportUC == nil {
		respondError(w, http.StatusServiceUnavailable, "report service not configured")
		return
	}
	formats, err := h.reportUC.ListFormats(r.Context())
	if err != nil {
		// Return static list as fallback
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"formats": []string{"console", "csv", "json", "json2", "html", "pdf"},
		})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"formats": formats})
}
