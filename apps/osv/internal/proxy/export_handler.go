// Package proxy — CVE Export endpoint handler for apps/osv gateway.
//
// GET /api/v1/cve/{id}/export?format=json|csv|xml
//
// This is the ONLY non-pure-proxy handler in the gateway.
// It fetches CVE data from data-service then transforms the format.
// Business logic (CVE data retrieval) stays in data-service.
// Gateway only handles format presentation (download conversion).
package proxy

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ExportHandler handles CVE data export in multiple formats.
type ExportHandler struct {
	dataServiceURL string
	httpClient     *http.Client
}

// NewExportHandler creates an ExportHandler.
// dataServiceURL: HTTP base URL for data-service (e.g. "http://data-service:8082")
func NewExportHandler(dataServiceURL string) *ExportHandler {
	return &ExportHandler{
		dataServiceURL: strings.TrimRight(dataServiceURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ServeHTTP handles: GET /api/v1/cve/{id}/export
// Query params:
//   - format: "json" | "csv" | "xml" (default: json)
//
// Path pattern expected: /api/v1/cve/{id}/export
// CVE ID is extracted from the path without requiring chi router.
func (h *ExportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract CVE ID from path: /api/v1/cve/{id}/export
	cveID := extractCVEIDFromPath(r.URL.Path)
	if cveID == "" {
		http.Error(w, `{"error":"missing CVE ID in path"}`, http.StatusBadRequest)
		return
	}

	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" && format != "xml" {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"format must be json, csv, or xml"}`, http.StatusBadRequest)
		return
	}

	// Fetch raw CVE JSON from data-service
	upstream := fmt.Sprintf("%s/cve/%s", h.dataServiceURL, cveID)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstream, nil)
	if err != nil {
		http.Error(w, `{"error":"internal error building request"}`, http.StatusInternalServerError)
		return
	}
	// Forward relevant headers
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		http.Error(w, `{"error":"upstream data-service unavailable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error":"CVE %s not found"}`, cveID)
		return
	}
	if resp.StatusCode != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, `{"error":"upstream error %d"}`, resp.StatusCode)
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB cap
	if err != nil {
		http.Error(w, `{"error":"failed to read upstream response"}`, http.StatusInternalServerError)
		return
	}

	// Decode JSON → map for format conversion
	var cveData map[string]interface{}
	if err := json.Unmarshal(body, &cveData); err != nil {
		http.Error(w, `{"error":"invalid data from upstream"}`, http.StatusInternalServerError)
		return
	}

	// Content-Disposition: attachment with filename
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s.%s"`, cveID, format))

	switch format {
	case "csv":
		h.renderCSV(w, cveData)
	case "xml":
		h.renderXML(w, cveData)
	default: // json
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(body) //nolint:errcheck
	}
}

// renderCSV writes CVE data as a CSV attachment.
// Columns: id, published, modified, summary, cvss, cvss3, cwe
func (h *ExportHandler) renderCSV(w http.ResponseWriter, cve map[string]interface{}) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header row
	writer.Write([]string{"id", "published", "modified", "summary", "cvss", "cvss3", "cwe"}) //nolint:errcheck
	// Data row
	writer.Write([]string{ //nolint:errcheck
		exportMapStr(cve, "id"),
		exportMapStr(cve, "published"),
		exportMapStr(cve, "modified"),
		exportMapStr(cve, "summary"),
		exportMapStr(cve, "cvss"),
		exportMapStr(cve, "cvss3"),
		exportMapStr(cve, "cwe"),
	})
}

// renderXML writes CVE data as an XML attachment.
func (h *ExportHandler) renderXML(w http.ResponseWriter, cve map[string]interface{}) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")

	type CVEExportXML struct {
		XMLName   xml.Name `xml:"cve"`
		ID        string   `xml:"id"`
		Published string   `xml:"published"`
		Modified  string   `xml:"modified"`
		Summary   string   `xml:"summary"`
		CVSS      string   `xml:"cvss,omitempty"`
		CVSS3     string   `xml:"cvss3,omitempty"`
		CWE       string   `xml:"cwe,omitempty"`
	}

	export := CVEExportXML{
		ID:        exportMapStr(cve, "id"),
		Published: exportMapStr(cve, "published"),
		Modified:  exportMapStr(cve, "modified"),
		Summary:   exportMapStr(cve, "summary"),
		CVSS:      exportMapStr(cve, "cvss"),
		CVSS3:     exportMapStr(cve, "cvss3"),
		CWE:       exportMapStr(cve, "cwe"),
	}

	w.Write([]byte(xml.Header)) //nolint:errcheck
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	enc.Encode(export) //nolint:errcheck
}

// extractCVEIDFromPath extracts the CVE ID from a path like /api/v1/cve/{id}/export.
// Returns empty string if the path does not match the expected pattern.
func extractCVEIDFromPath(path string) string {
	// Expected: /api/v1/cve/{id}/export
	// Split: ["", "api", "v1", "cve", "{id}", "export"]
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 5 &&
		parts[0] == "api" &&
		parts[1] == "v1" &&
		parts[2] == "cve" &&
		parts[4] == "export" &&
		parts[3] != "" {
		return parts[3]
	}
	return ""
}

// exportMapStr returns the string representation of a map value.
// Returns empty string if key not found or value is nil.
func exportMapStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}
