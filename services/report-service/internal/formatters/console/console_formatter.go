// Package console implements the ANSI colored console reporter.
package console

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/osv/report-service/internal/domain/entity"
	"github.com/osv/report-service/internal/domain/service"
	"github.com/osv/report-service/internal/formatters"
)

// ANSI color codes
const (
	reset     = "\033[0m"
	bold      = "\033[1m"
	red       = "\033[31m"
	boldRed   = "\033[1;31m"
	bgRed     = "\033[41m"
	yellow    = "\033[33m"
	green     = "\033[32m"
	cyan      = "\033[36m"
	white     = "\033[37m"
	dim       = "\033[2m"
)

// ConsoleFormatter renders colorized terminal output.
type ConsoleFormatter struct{}

// New creates a new ConsoleFormatter.
func New() *ConsoleFormatter { return &ConsoleFormatter{} }

// ContentType returns the MIME type.
func (f *ConsoleFormatter) ContentType() string { return "text/plain; charset=utf-8" }

// FileExtension returns ".txt" for console output.
func (f *ConsoleFormatter) FileExtension() string { return ".txt" }

// Format renders CVE findings to ANSI-colored text.
func (f *ConsoleFormatter) Format(_ context.Context, data entity.ReportInput, opts formatters.FormatOptions) ([]byte, error) {
	var buf bytes.Buffer

	// Header
	buf.WriteString(bold + "CVE Binary Tool Scan Results" + reset + "\n")
	buf.WriteString(strings.Repeat("=", 80) + "\n\n")

	if data.ScanTarget != "" {
		buf.WriteString(dim + "Target: " + data.ScanTarget + reset + "\n\n")
	}

	totalCVEs := 0
	exploitCount := 0

	// Sort products for deterministic output
	for product, cves := range data.CVEData {
		if opts.Quiet && len(cves) == 0 {
			continue
		}

		filtered := service.FilterAndSort(cves, data.MinSeverity, data.MinScore)
		if len(filtered) == 0 {
			continue
		}

		productLabel := fmt.Sprintf("[%s] %s %s", product.Vendor, product.Product, product.Version)
		buf.WriteString(cyan + productLabel + reset + "\n")

		for _, cve := range filtered {
			totalCVEs++
			color := severityColor(cve.Severity)

			exploitMark := ""
			if cve.IsExploit {
				exploitCount++
				exploitMark = boldRed + " ⚠ EXPLOIT" + reset
			}

			line := fmt.Sprintf("  %-20s%s  Score: %.1f | %s",
				color+cve.CVENumber+reset,
				exploitMark,
				cve.Score,
				dim+cve.Severity+reset,
			)
			if cve.CVSSVector != "" {
				line += " | " + dim + cve.CVSSVector + reset
			}
			buf.WriteString(line + "\n")
		}
		buf.WriteString("\n")
	}

	// Summary
	buf.WriteString(strings.Repeat("-", 80) + "\n")
	summaryColor := green
	if totalCVEs > 0 {
		summaryColor = red
	}
	summary := fmt.Sprintf("Summary: %d CVE(s) found", totalCVEs)
	if exploitCount > 0 {
		summary += fmt.Sprintf(" (%d exploitable)", exploitCount)
	}
	buf.WriteString(summaryColor + bold + summary + reset + "\n")

	return buf.Bytes(), nil
}

func severityColor(sev string) string {
	switch sev {
	case "CRITICAL":
		return boldRed + bgRed
	case "HIGH":
		return red
	case "MEDIUM":
		return yellow
	case "LOW":
		return green
	default:
		return white
	}
}

// Ensure interface compliance.
var _ formatters.Formatter = (*ConsoleFormatter)(nil)
