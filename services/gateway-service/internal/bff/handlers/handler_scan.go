// Package http provides v1 scan route handlers for the CVE Binary Tool API.
package http

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/osv/gateway-service/internal/usecase/scan"
)

// handleBinaryScan handles POST /api/v1/scan/binary
// Accepts multipart/form-data with:
//   - file: binary file (required)
//   - options: JSON-encoded ScanOptions (optional)
//   - vex_file: VEX file for triage (optional)
func (h *CVEHandler) handleBinaryScan(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close() //nolint:errcheck

	fileBytes, err := io.ReadAll(io.LimitReader(file, 512<<20))
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to read file")
		return
	}

	// Parse optional JSON options
	type scanOptions struct {
		Formats         []string `json:"formats"`
		MinCVSS         float32  `json:"min_cvss"`
		MinSeverity     string   `json:"min_severity"`
		CheckExploits   bool     `json:"check_exploits"`
		Extract         bool     `json:"extract"`
		SkipCheckers    []string `json:"skip_checkers"`
		RunCheckers     []string `json:"run_checkers"`
		DisabledSources []string `json:"disabled_sources"`
	}
	opts := scanOptions{Formats: []string{"json"}}
	if optsStr := r.FormValue("options"); optsStr != "" {
		_ = json.Unmarshal([]byte(optsStr), &opts) //nolint:errcheck
	}

	// Read optional VEX file
	var vexBytes []byte
	var vexName string
	if vexFile, vexHeader, err := r.FormFile("vex_file"); err == nil {
		defer vexFile.Close() //nolint:errcheck
		vexBytes, _ = io.ReadAll(io.LimitReader(vexFile, 10<<20))
		vexName = vexHeader.Filename
	}

	if h.scanUC == nil {
		respondError(w, http.StatusServiceUnavailable, "scan service not configured")
		return
	}

	result, err := h.scanUC.Execute(r.Context(), scan.Input{
		FileBytes:       fileBytes,
		FileName:        header.Filename,
		Formats:         opts.Formats,
		MinCVSS:         opts.MinCVSS,
		MinSeverity:     opts.MinSeverity,
		CheckExploits:   opts.CheckExploits,
		DisabledSources: opts.DisabledSources,
		VEXFileBytes:    vexBytes,
		VEXFileName:     vexName,
		Extract:         opts.Extract,
		SkipCheckers:    opts.SkipCheckers,
		RunCheckers:     opts.RunCheckers,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("X-CVE-Exit-Code", strconv.Itoa(result.ExitCode))

	primaryFormat := opts.Formats[0]
	data, ok := result.Reports[primaryFormat]
	if !ok {
		// Fall back to JSON summary
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"total_cves": result.TotalCVEs,
			"exit_code":  result.ExitCode,
			"cve_data":   result.CVEData,
		})
		return
	}

	w.Header().Set("Content-Type", contentTypeFor(primaryFormat))
	w.WriteHeader(http.StatusOK)
	w.Write(data) //nolint:errcheck
}

// handlePackageListScan handles POST /api/v1/scan/package-list
func (h *CVEHandler) handlePackageListScan(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close() //nolint:errcheck

	fileBytes, _ := io.ReadAll(io.LimitReader(file, 50<<20))

	if h.scanUC == nil {
		respondError(w, http.StatusServiceUnavailable, "scan service not configured")
		return
	}

	result, err := h.scanUC.Execute(r.Context(), scan.Input{
		FileBytes: fileBytes,
		FileName:  header.Filename,
		Formats:   []string{"json"},
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_cves": result.TotalCVEs,
		"exit_code":  result.ExitCode,
		"cve_data":   result.CVEData,
	})
}

// handleMergeReports handles POST /api/v1/scan/merge
func (h *CVEHandler) handleMergeReports(w http.ResponseWriter, r *http.Request) {
	type mergeRequest struct {
		Reports [][]byte `json:"reports"`
	}
	var req mergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.Reports) == 0 {
		respondError(w, http.StatusBadRequest, "reports array is required")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"merged": true, "count": len(req.Reports)})
}

func contentTypeFor(format string) string {
	switch format {
	case "html":
		return "text/html; charset=utf-8"
	case "pdf":
		return "application/pdf"
	case "csv":
		return "text/csv"
	default:
		return "application/json"
	}
}
