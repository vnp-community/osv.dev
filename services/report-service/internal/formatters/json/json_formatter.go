// Package json implements JSON report formatters.
package json

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/osv/report-service/internal/domain/entity"
	"github.com/osv/report-service/internal/domain/service"
	"github.com/osv/report-service/internal/formatters"
)

// ─────────────────────────────────────────────────────────────────────────────
// JSON (flat) formatter
// ─────────────────────────────────────────────────────────────────────────────

// jsonCVEEntry is a single flat CVE entry in JSON output.
type jsonCVEEntry struct {
	Vendor      string  `json:"vendor"`
	Product     string  `json:"product"`
	Version     string  `json:"version"`
	CVENumber   string  `json:"cve_number"`
	Severity    string  `json:"severity"`
	Score       float64 `json:"score"`
	CVSSVersion int     `json:"cvss_version,omitempty"`
	CVSSVector  string  `json:"cvss_vector,omitempty"`
	DataSource  string  `json:"data_source"`
	Remarks     int     `json:"remarks,omitempty"`
	IsExploit   bool    `json:"is_exploit"`
}

// jsonReport is the flat JSON output format.
type jsonReport struct {
	Metadata struct {
		Timestamp  string `json:"timestamp"`
		ScanTarget string `json:"scan_target"`
		TotalCVEs  int    `json:"total_cves"`
	} `json:"metadata"`
	CVEs []jsonCVEEntry `json:"cves"`
}

// JSONFormatter produces a flat JSON report.
type JSONFormatter struct{}

// New creates a new JSONFormatter.
func New() *JSONFormatter { return &JSONFormatter{} }

// ContentType returns application/json.
func (f *JSONFormatter) ContentType() string { return "application/json" }

// FileExtension returns ".json".
func (f *JSONFormatter) FileExtension() string { return ".json" }

// Format renders CVE findings to flat JSON bytes.
func (f *JSONFormatter) Format(_ context.Context, data entity.ReportInput, _ formatters.FormatOptions) ([]byte, error) {
	var report jsonReport
	report.Metadata.Timestamp = timeOrNow(data.GeneratedAt)
	report.Metadata.ScanTarget = data.ScanTarget
	report.CVEs = []jsonCVEEntry{}

	for product, cves := range data.CVEData {
		filtered := service.FilterAndSort(cves, data.MinSeverity, data.MinScore)
		for _, cve := range filtered {
			report.CVEs = append(report.CVEs, jsonCVEEntry{
				Vendor:      product.Vendor,
				Product:     product.Product,
				Version:     product.Version,
				CVENumber:   cve.CVENumber,
				Severity:    cve.Severity,
				Score:       cve.Score,
				CVSSVersion: cve.CVSSVersion,
				CVSSVector:  cve.CVSSVector,
				DataSource:  cve.DataSource,
				Remarks:     cve.Remarks,
				IsExploit:   cve.IsExploit,
			})
		}
	}

	report.Metadata.TotalCVEs = len(report.CVEs)
	return marshalIndent(report)
}

// ─────────────────────────────────────────────────────────────────────────────
// JSON2 (organized by product) formatter
// ─────────────────────────────────────────────────────────────────────────────

// json2ProductEntry is a product entry with its CVEs.
type json2ProductEntry struct {
	Vendor  string         `json:"vendor"`
	Product string         `json:"product"`
	Version string         `json:"version"`
	PURL    string         `json:"purl,omitempty"`
	CVEs    []json2CVEItem `json:"cves"`
}

// json2CVEItem is a CVE entry in JSON2 format.
type json2CVEItem struct {
	CVENumber   string  `json:"cve_number"`
	Severity    string  `json:"severity"`
	Score       float64 `json:"score"`
	CVSSVersion int     `json:"cvss_version,omitempty"`
	CVSSVector  string  `json:"cvss_vector,omitempty"`
	DataSource  string  `json:"data_source"`
	IsExploit   bool    `json:"is_exploit"`
}

// json2Report is the organized JSON2 output format.
type json2Report struct {
	Metadata struct {
		Timestamp  string `json:"timestamp"`
		ScanTarget string `json:"scan_target"`
		TotalCVEs  int    `json:"total_cves"`
	} `json:"metadata"`
	Results map[string]json2ProductEntry `json:"results"`
}

// JSON2Formatter produces an organized JSON report grouped by product.
type JSON2Formatter struct{}

// NewJSON2 creates a new JSON2Formatter.
func NewJSON2() *JSON2Formatter { return &JSON2Formatter{} }

// ContentType returns application/json.
func (f *JSON2Formatter) ContentType() string { return "application/json" }

// FileExtension returns ".json".
func (f *JSON2Formatter) FileExtension() string { return ".json" }

// Format renders CVE findings organized by product key.
func (f *JSON2Formatter) Format(_ context.Context, data entity.ReportInput, _ formatters.FormatOptions) ([]byte, error) {
	var report json2Report
	report.Metadata.Timestamp = timeOrNow(data.GeneratedAt)
	report.Metadata.ScanTarget = data.ScanTarget
	report.Results = make(map[string]json2ProductEntry)

	totalCVEs := 0
	for product, cves := range data.CVEData {
		filtered := service.FilterAndSort(cves, data.MinSeverity, data.MinScore)
		if len(filtered) == 0 {
			continue
		}

		key := product.Vendor + "/" + product.Product + "@" + product.Version
		items := make([]json2CVEItem, 0, len(filtered))
		for _, cve := range filtered {
			items = append(items, json2CVEItem{
				CVENumber:   cve.CVENumber,
				Severity:    cve.Severity,
				Score:       cve.Score,
				CVSSVersion: cve.CVSSVersion,
				CVSSVector:  cve.CVSSVector,
				DataSource:  cve.DataSource,
				IsExploit:   cve.IsExploit,
			})
		}
		totalCVEs += len(items)
		report.Results[key] = json2ProductEntry{
			Vendor:  product.Vendor,
			Product: product.Product,
			Version: product.Version,
			PURL:    product.PURL,
			CVEs:    items,
		}
	}
	report.Metadata.TotalCVEs = totalCVEs

	return marshalIndent(report)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func marshalIndent(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func timeOrNow(t time.Time) string {
	if t.IsZero() {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return t.UTC().Format(time.RFC3339)
}

// Ensure interface compliance.
var _ formatters.Formatter = (*JSONFormatter)(nil)
var _ formatters.Formatter = (*JSON2Formatter)(nil)
