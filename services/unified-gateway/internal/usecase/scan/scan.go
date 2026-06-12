// Package scan implements the core gateway scan orchestration use case.
package scan

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"

	gc "github.com/osv/unified-gateway/internal/adapter/grpcclient"
	cvedbpb "github.com/osv/proto/gen/go/cvedb/v1"
	reporterpb "github.com/osv/proto/gen/go/reporter/v1"
)

// ─────────────────────────────────────────────────────────────────────────────
// Port interfaces (Dependency Inversion)
// ─────────────────────────────────────────────────────────────────────────────

// ScannerPort defines what the use case needs from the scanner service.
type ScannerPort interface {
	ScanFile(ctx context.Context, fileBytes []byte, fileName string, opts gc.ScanOptions) ([]gc.ScanResult, error)
	ScanPackageList(ctx context.Context, fileBytes []byte, fileName string) ([]gc.ScanResult, string, error)
}

// CVEDBPort defines what the use case needs from the cvedb service.
type CVEDBPort interface {
	LookupCVEs(ctx context.Context, req gc.LookupRequest) (*gc.LookupResult, error)
}

// ReporterPort defines what the use case needs from the reporter service.
type ReporterPort interface {
	GenerateReport(ctx context.Context, req gc.GenerateReportRequest) (*gc.GenerateReportResult, error)
}

// SBOMVEXPort defines what the use case needs from the sbomvex service.
type SBOMVEXPort interface {
	ParseVEX(ctx context.Context, fileBytes []byte, fileName, format string) (*gc.ParseVEXResult, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Input / Output
// ─────────────────────────────────────────────────────────────────────────────

// Input configures the scan orchestration.
type Input struct {
	FileBytes       []byte
	FileName        string
	Formats         []string // default: ["json"]
	MinCVSS         float32
	MinSeverity     string
	CheckExploits   bool
	DisabledSources []string
	VEXFileBytes    []byte
	VEXFileName     string
	Extract         bool
	SkipCheckers    []string
	RunCheckers     []string
}

// CVEResult is the Go-friendly CVE entry returned to the HTTP layer.
type CVEResult struct {
	CVENumber  string
	Severity   string
	Score      float32
	Product    string
	Version    string
	Vendor     string
	DataSource string
	IsExploit  bool
}

// Output holds the orchestration result.
type Output struct {
	CVEData   []CVEResult
	Reports   map[string][]byte
	TotalCVEs int
	ExitCode  int // 0=clean, 1=CVEs found
}

// ─────────────────────────────────────────────────────────────────────────────
// UseCase
// ─────────────────────────────────────────────────────────────────────────────

// UseCase orchestrates scanner → cvedb → reporter.
type UseCase struct {
	scannerClient  ScannerPort
	cvedbClient    CVEDBPort
	reporterClient ReporterPort
	sbomvexClient  SBOMVEXPort
	log            zerolog.Logger
}

// NewUseCase creates a ScanUseCase.
func NewUseCase(
	scanner ScannerPort,
	cvedb CVEDBPort,
	reporter ReporterPort,
	sbomvex SBOMVEXPort,
	log zerolog.Logger,
) *UseCase {
	return &UseCase{
		scannerClient:  scanner,
		cvedbClient:    cvedb,
		reporterClient: reporter,
		sbomvexClient:  sbomvex,
		log:            log,
	}
}

// Execute runs the full scan pipeline.
func (uc *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
	formats := in.Formats
	if len(formats) == 0 {
		formats = []string{"json"}
	}

	// Step 1: Scan file → product list
	scanResults, err := uc.scan(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	if len(scanResults) == 0 {
		return &Output{Reports: map[string][]byte{"json": []byte(`{"results":[]}`)}, ExitCode: 0}, nil
	}

	// Step 2: Parse VEX triage if provided
	var triageData map[string]interface{}
	if len(in.VEXFileBytes) > 0 && uc.sbomvexClient != nil {
		vtr, err := uc.sbomvexClient.ParseVEX(ctx, in.VEXFileBytes, in.VEXFileName, "")
		if err != nil {
			uc.log.Warn().Err(err).Msg("ParseVEX failed, continuing without triage")
		} else {
			_ = vtr // will be converted below
			triageData = map[string]interface{}{} // placeholder
		}
	}
	_ = triageData

	// Step 3: Build cvedb product list
	cvedbProducts := buildCVEDBProducts(scanResults)

	// Step 4: LookupCVEs
	lookupResult, err := uc.cvedbClient.LookupCVEs(ctx, gc.LookupRequest{
		Products:        cvedbProducts,
		MinScore:        in.MinCVSS,
		CheckExploits:   in.CheckExploits,
		DisabledSources: in.DisabledSources,
	})
	if err != nil {
		return nil, fmt.Errorf("lookup CVEs: %w", err)
	}

	// Step 5: Generate report
	reportResult, err := uc.reporterClient.GenerateReport(ctx, gc.GenerateReportRequest{
		Products:    toReporterProductCVEs(lookupResult.Results),
		Formats:     formats,
		ScanTarget:  in.FileName,
		MinSeverity: in.MinSeverity,
		MinScore:    in.MinCVSS,
	})
	if err != nil {
		return nil, fmt.Errorf("generate report: %w", err)
	}

	// Step 6: Build output
	cveResults := extractCVEResults(lookupResult.Results)
	exitCode := 0
	if len(cveResults) > 0 {
		exitCode = 1
	}

	return &Output{
		CVEData:   cveResults,
		Reports:   reportResult.Reports,
		TotalCVEs: len(cveResults),
		ExitCode:  exitCode,
	}, nil
}

// scan dispatches to the correct scanner method based on file type.
func (uc *UseCase) scan(ctx context.Context, in Input) ([]gc.ScanResult, error) {
	base := strings.ToLower(filepath.Base(in.FileName))

	// Detect package manifest files
	manifestFiles := map[string]bool{
		"go.mod": true, "go.sum": true,
		"requirements.txt": true, "package.json": true, "package-lock.json": true,
		"cargo.lock": true, "cargo.toml": true, "pom.xml": true,
		"build.gradle": true, "gemfile.lock": true, "gemfile": true, "conanfile.txt": true,
	}
	if manifestFiles[base] {
		results, _, err := uc.scannerClient.ScanPackageList(ctx, in.FileBytes, in.FileName)
		return results, err
	}

	// Default: binary scan
	return uc.scannerClient.ScanFile(ctx, in.FileBytes, in.FileName, gc.ScanOptions{
		Extract:      in.Extract,
		SkipCheckers: in.SkipCheckers,
		RunCheckers:  in.RunCheckers,
	})
}

// buildCVEDBProducts converts scan results to cvedb ProductInfo protos.
func buildCVEDBProducts(results []gc.ScanResult) []*cvedbpb.ProductInfo {
	products := make([]*cvedbpb.ProductInfo, 0, len(results))
	for _, r := range results {
		products = append(products, &cvedbpb.ProductInfo{
			Vendor:  r.Vendor,
			Product: r.Product,
			Version: r.Version,
			Purl:    r.PURL,
		})
	}
	return products
}

// toReporterProductCVEs converts lookup results to reporter ProductCVEs protos.
func toReporterProductCVEs(results []*cvedbpb.ProductCVEs) []*reporterpb.ProductCVEs {
	out := make([]*reporterpb.ProductCVEs, 0, len(results))
	for _, r := range results {
		var cves []*reporterpb.CVEData
		for _, c := range r.GetCves() {
			cves = append(cves, &reporterpb.CVEData{
				CveNumber:  c.GetCveNumber(),
				Severity:   c.GetSeverity(),
				Score:      c.GetScore(),
				DataSource: c.GetDataSource(),
				IsExploit:  c.GetIsExploit(),
			})
		}
		if r.GetProduct() != nil {
			out = append(out, &reporterpb.ProductCVEs{
				Product: &reporterpb.ProductInfo{
					Vendor:  r.GetProduct().GetVendor(),
					Product: r.GetProduct().GetProduct(),
					Version: r.GetProduct().GetVersion(),
					Purl:    r.GetProduct().GetPurl(),
				},
				Cves: cves,
			})
		}
	}
	return out
}

// extractCVEResults flattens the cvedb lookup result into CVEResult slice.
func extractCVEResults(results []*cvedbpb.ProductCVEs) []CVEResult {
	var out []CVEResult
	for _, pc := range results {
		p := pc.GetProduct()
		for _, c := range pc.GetCves() {
			out = append(out, CVEResult{
				CVENumber:  c.GetCveNumber(),
				Severity:   c.GetSeverity(),
				Score:      c.GetScore(),
				Product:    p.GetProduct(),
				Vendor:     p.GetVendor(),
				Version:    p.GetVersion(),
				DataSource: c.GetDataSource(),
				IsExploit:  c.GetIsExploit(),
			})
		}
	}
	return out
}
