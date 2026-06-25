// Package csv implements the CSV report formatter.
package csv

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/osv/finding-service/internal/domain/report"
	"github.com/osv/finding-service/internal/domain/report/service"
	"github.com/osv/finding-service/internal/formatters"
)

// CSVFormatter writes CVE data as RFC 4180 CSV.
type CSVFormatter struct{}

// New creates a new CSVFormatter.
func New() *CSVFormatter { return &CSVFormatter{} }

// ContentType returns text/csv.
func (f *CSVFormatter) ContentType() string { return "text/csv" }

// FileExtension returns ".csv".
func (f *CSVFormatter) FileExtension() string { return ".csv" }

// csvHeader is the canonical CSV header row.
var csvHeader = []string{
	"Vendor", "Product", "Version",
	"CVE_number", "Severity", "Score",
	"CVSS_version", "CVSS_vector",
	"Data_source", "Remarks", "IsExploit",
}

// Format renders CVE findings to CSV bytes.
func (f *CSVFormatter) Format(_ context.Context, data report.ReportInput, _ formatters.FormatOptions) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if err := w.Write(csvHeader); err != nil {
		return nil, err
	}

	for product, cves := range data.CVEData {
		filtered := service.FilterAndSort(cves, data.MinSeverity, data.MinScore)
		for _, cve := range filtered {
			row := []string{
				product.Vendor,
				product.Product,
				product.Version,
				cve.CVENumber,
				cve.Severity,
				formatScore(cve.Score),
				strconv.Itoa(cve.CVSSVersion),
				cve.CVSSVector,
				cve.DataSource,
				strconv.Itoa(cve.Remarks),
				boolStr(cve.IsExploit),
			}
			if err := w.Write(row); err != nil {
				return nil, err
			}
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func formatScore(s float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", s), "0"), ".")
}

func boolStr(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// Ensure interface compliance.
var _ formatters.Formatter = (*CSVFormatter)(nil)
