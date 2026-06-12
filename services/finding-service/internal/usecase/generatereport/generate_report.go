// Package generatereport implements the GenerateReport use case.
package generatereport

import (
	"context"
	"fmt"
	"time"

	"github.com/osv/finding-service/internal/domain/entity"
	"github.com/osv/finding-service/internal/domain/service"
	"github.com/osv/finding-service/internal/formatters"
)

// Input configures report generation.
type Input struct {
	// CVEData is the raw CVE data keyed by product.
	CVEData map[entity.ProductInfo][]entity.CVEData
	// Formats is the list of output formats to generate.
	Formats []entity.OutputFormat
	// ScanTarget is the scanned binary/directory path.
	ScanTarget  string
	// MinSeverity filters CVEs below this severity level.
	MinSeverity string
	// MinScore filters CVEs below this CVSS score.
	MinScore float64
	// Theme is "light" or "dark".
	Theme string
	// Quiet suppresses informational output.
	Quiet bool
}

// Output holds the generated report bytes per format.
type Output struct {
	Reports   map[entity.OutputFormat][]byte
	ExitCode  int // 0 = no CVEs after filtering, 1 = CVEs found
	TotalCVEs int
}

// UseCase orchestrates filtering + multi-format report generation.
type UseCase struct {
	formatters formatters.Registry
}

// NewUseCase creates a new GenerateReport use case.
func NewUseCase(registry formatters.Registry) *UseCase {
	return &UseCase{formatters: registry}
}

// Execute applies filters and generates reports in all requested formats.
func (uc *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
	if len(in.Formats) == 0 {
		in.Formats = []entity.OutputFormat{entity.FormatJSON}
	}

	// Apply filters across all products
	filtered := make(map[entity.ProductInfo][]entity.CVEData, len(in.CVEData))
	totalCVEs := 0
	for product, cves := range in.CVEData {
		filteredCVEs := service.FilterAndSort(cves, in.MinSeverity, in.MinScore)
		filtered[product] = filteredCVEs
		totalCVEs += len(filteredCVEs)
	}

	reportInput := entity.ReportInput{
		CVEData:     filtered,
		ScanTarget:  in.ScanTarget,
		GeneratedAt: time.Now().UTC(),
		Formats:     in.Formats,
		MinSeverity: in.MinSeverity,
		MinScore:    in.MinScore,
		Theme:       in.Theme,
	}

	opts := formatters.FormatOptions{
		Theme: in.Theme,
		Quiet: in.Quiet,
	}

	reports := make(map[entity.OutputFormat][]byte, len(in.Formats))
	for _, format := range in.Formats {
		formatter, ok := uc.formatters.Get(format)
		if !ok {
			return nil, fmt.Errorf("generatereport: no formatter for format %q", format)
		}
		data, err := formatter.Format(ctx, reportInput, opts)
		if err != nil {
			return nil, fmt.Errorf("generatereport: format %q: %w", format, err)
		}
		reports[format] = data
	}

	exitCode := 0
	if totalCVEs > 0 {
		exitCode = 1
	}

	return &Output{
		Reports:   reports,
		ExitCode:  exitCode,
		TotalCVEs: totalCVEs,
	}, nil
}
