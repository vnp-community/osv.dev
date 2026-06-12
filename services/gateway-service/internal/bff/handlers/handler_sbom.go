// Package http provides HTTP handlers for SBOM/VEX operations.
package http

import (
	"io"
	"net/http"

	grpcclient "github.com/osv/gateway-service/internal/adapter/grpcclient"
)

// HandleSBOMParse handles POST /api/v1/sbom/parse
// Accepts a multipart form with "file" field containing SBOM bytes.
func (h *CVEHandler) HandleSBOMParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := r.ParseMultipartForm(50 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "failed to parse form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	format := r.FormValue("format") // optional hint

	result, err := h.sbomvexClient.ParseSBOM(r.Context(), fileBytes, header.Filename, format)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "parse SBOM failed: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"products":         result.Products,
		"detected_format":  result.Format,
		"component_count":  len(result.Products),
	})
}

// HandleSBOMGenerate handles POST /api/v1/sbom/generate
// Body JSON: {"products": [...], "format": "spdx|cyclonedx", "package_name": "..."}
func (h *CVEHandler) HandleSBOMGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "spdx"
	}

	req := grpcclient.GenerateSBOMRequest{
		Format:     format,
		ToolName:   "cve-bin-tool",
		ToolVersion: "2.0",
	}

	doc, err := h.sbomvexClient.GenerateSBOM(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "generate SBOM failed: "+err.Error())
		return
	}

	contentType := "text/plain"
	if format == "cyclonedx" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=sbom."+format)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(doc)
}

// HandleVEXParse handles POST /api/v1/vex/parse
// Accepts multipart form with "file" field containing VEX bytes.
func (h *CVEHandler) HandleVEXParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "failed to parse form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	format := r.FormValue("format")

	result, err := h.sbomvexClient.ParseVEX(r.Context(), fileBytes, header.Filename, format)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "parse VEX failed: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"triage_data":    result.TriageData,
		"statement_count": len(result.TriageData),
	})
}

// HandleVEXGenerate handles POST /api/v1/vex/generate
// Query param: format=openvex|cyclonedx|csaf (default: openvex)
func (h *CVEHandler) HandleVEXGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "openvex"
	}

	req := grpcclient.GenerateVEXRequest{Format: format}

	doc, err := h.sbomvexClient.GenerateVEX(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "generate VEX failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=vex."+format+".json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(doc)
}

// HandleSBOMDetect handles POST /api/v1/sbom/detect
// Detects the SBOM format without full parsing.
func (h *CVEHandler) HandleSBOMDetect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	// Read first 4KB for format detection
	buf := make([]byte, 4096)
	n, _ := file.Read(buf)

	format, err := h.sbomvexClient.DetectSBOMFormat(r.Context(), buf[:n])
	if err != nil {
		respondError(w, http.StatusInternalServerError, "detect format failed: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"format": format})
}
