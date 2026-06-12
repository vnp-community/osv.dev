// Package report implements the standalone report generation use case.
package report

import (
	"context"
	"fmt"

	gc "github.com/osv/gateway-service/internal/adapter/grpcclient"
	reporterpb "github.com/osv/shared/proto/gen/go/reporter/v1"
)

// ReporterPort defines what the use case needs from the reporter service.
type ReporterPort interface {
	GenerateReport(ctx context.Context, req gc.GenerateReportRequest) (*gc.GenerateReportResult, error)
	GetSupportedFormats(ctx context.Context) ([]gc.FormatInfo, error)
}

// Input configures report generation.
type Input struct {
	CVEData     []*reporterpb.ProductCVEs
	Formats     []string
	MinSeverity string
	MinScore    float32
	Theme       string
	ScanTarget  string
}

// Output holds the report result.
type Output struct {
	Reports   map[string][]byte
	TotalCVEs int
	ExitCode  int
}

// UseCase generates reports from raw CVE data.
type UseCase struct {
	reporterClient ReporterPort
}

// NewUseCase creates a Report use case.
func NewUseCase(client ReporterPort) *UseCase {
	return &UseCase{reporterClient: client}
}

// Execute generates reports.
func (uc *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
	if len(in.Formats) == 0 {
		in.Formats = []string{"json"}
	}

	result, err := uc.reporterClient.GenerateReport(ctx, gc.GenerateReportRequest{
		Products:    in.CVEData,
		Formats:     in.Formats,
		ScanTarget:  in.ScanTarget,
		MinSeverity: in.MinSeverity,
		MinScore:    in.MinScore,
		Theme:       in.Theme,
	})
	if err != nil {
		return nil, fmt.Errorf("GenerateReport: %w", err)
	}

	totalCVEs := 0
	for _, pc := range in.CVEData {
		totalCVEs += len(pc.GetCves())
	}

	exitCode := 0
	if int(result.ExitCode) != 0 {
		exitCode = int(result.ExitCode)
	} else if totalCVEs > 0 {
		exitCode = 1
	}

	return &Output{
		Reports:   result.Reports,
		TotalCVEs: totalCVEs,
		ExitCode:  exitCode,
	}, nil
}

// ListFormats returns all supported output formats.
func (uc *UseCase) ListFormats(ctx context.Context) ([]gc.FormatInfo, error) {
	return uc.reporterClient.GetSupportedFormats(ctx)
}
