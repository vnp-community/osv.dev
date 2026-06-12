// Package import_usecase implements the scan import pipeline use case.
// This is the central orchestrator of a scan result import:
// validate → store file → resolve context → parse → dedup → save → notify.
package import_usecase

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	natsutil "github.com/defectdojo/pkg/nats"
	"github.com/defectdojo/scan-orchestrator/internal/domain/scan"
	"github.com/defectdojo/scan-orchestrator/internal/domain/parser"
	"github.com/defectdojo/scan-orchestrator/internal/infrastructure/dedup"
	infparser "github.com/defectdojo/scan-orchestrator/internal/infrastructure/parser"
	findingv1 "github.com/defectdojo/proto/finding/v1"
	productv1 "github.com/defectdojo/proto/product/v1"
	"github.com/defectdojo/product-management/internal/domain/engagement"
)

// ImportScanInput holds the parameters for an import-scan request.
type ImportScanInput struct {
	ScanType string
	File     io.Reader
	Options  scan.ImportOptions
}

// ImportScanOutput holds the result of a successful import.
type ImportScanOutput struct {
	ImportID      string
	TotalFindings int
	NewFindings   int
	ClosedFindings int
	TestID        string
	EngagementID  string
}

// ImportScanUseCase orchestrates the full import pipeline.
type ImportScanUseCase struct {
	parserFactory  *infparser.Factory
	productClient  productv1.ProductServiceClient
	findingClient  findingv1.FindingServiceClient
	eventPub       *natsutil.Publisher
	log            zerolog.Logger
}

// NewImportScan creates the use case with all required clients.
func NewImportScan(
	factory *infparser.Factory,
	productClient productv1.ProductServiceClient,
	findingClient findingv1.FindingServiceClient,
	pub *natsutil.Publisher,
	log zerolog.Logger,
) *ImportScanUseCase {
	return &ImportScanUseCase{
		parserFactory: factory,
		productClient: productClient,
		findingClient: findingClient,
		eventPub:      pub,
		log:           log,
	}
}

// Execute runs the full 9-step import pipeline.
func (uc *ImportScanUseCase) Execute(ctx context.Context, in ImportScanInput) (*ImportScanOutput, error) {
	importID := uuid.NewString()

	// ── Step 1: Get parser ────────────────────────────────────────────────────
	p, err := uc.parserFactory.Get(in.ScanType)
	if err != nil {
		return nil, fmt.Errorf("import scan: %w", err)
	}

	// ── Step 2: Resolve product/engagement/test via gRPC ─────────────────────
	productID, productTypeID, err := uc.resolveProduct(ctx, in.Options)
	if err != nil {
		return nil, fmt.Errorf("import scan: resolve product: %w", err)
	}

	engResp, err := uc.productClient.GetOrCreateEngagement(ctx, &productv1.GetOrCreateEngagementRequest{
		ProductId:                 productID,
		Name:                      in.Options.EngagementName,
		EngagementType:            "CI/CD",
		Version:                   in.Options.Version,
		BuildId:                   in.Options.BuildID,
		CommitHash:                in.Options.CommitHash,
		BranchTag:                 in.Options.BranchTag,
		DeduplicationOnEngagement: in.Options.DeduplicationOnEngagement,
	})
	if err != nil {
		return nil, fmt.Errorf("import scan: get_or_create_engagement: %w", err)
	}

	testTitle := in.Options.TestTitle
	if testTitle == "" {
		testTitle = fmt.Sprintf("%s %s", in.ScanType, time.Now().Format("2006-01-02 15:04"))
	}
	testResp, err := uc.productClient.GetOrCreateTest(ctx, &productv1.GetOrCreateTestRequest{
		EngagementId: engResp.EngagementId,
		ScanType:     in.ScanType,
		Title:        testTitle,
		Version:      in.Options.Version,
		BuildId:      in.Options.BuildID,
		CommitHash:   in.Options.CommitHash,
		BranchTag:    in.Options.BranchTag,
	})
	if err != nil {
		return nil, fmt.Errorf("import scan: get_or_create_test: %w", err)
	}

	// ── Step 3: Parse findings ─────────────────────────────────────────────────
	testCtx := &parser.TestContext{
		TestID:       testResp.TestId,
		EngagementID: engResp.EngagementId,
		ProductID:    productID,
		ScanType:     in.ScanType,
		Options: &parser.ParserOptions{
			MinimumSeverity: in.Options.MinimumSeverity,
			Service:         in.Options.Service,
		},
	}

	findings, err := p.GetFindings(ctx, in.File, testCtx)
	if err != nil {
		return nil, fmt.Errorf("import scan: parse: %w", err)
	}

	// ── Step 4: Filter by minimum severity ────────────────────────────────────
	if in.Options.MinimumSeverity != "" {
		findings = dedup.FilterBySeverity(findings, in.Options.MinimumSeverity)
	}

	// ── Step 5: Compute hash codes ────────────────────────────────────────────
	dedup.ComputeHashCodes(findings)

	// ── Step 6: Deduplicate — find new vs. existing findings ──────────────────
	var newFindings []*parser.ParsedFinding
	var existingIDs []string
	for _, f := range findings {
		resp, err := uc.findingClient.FindByHashCode(ctx, &findingv1.FindByHashCodeRequest{
			HashCode:                  f.HashCode,
			TestId:                    testResp.TestId,
			EngagementId:              &engResp.EngagementId,
			ProductId:                 &productID,
			DeduplicationOnEngagement: in.Options.DeduplicationOnEngagement,
		})
		if err == nil && resp.FindingId != nil {
			existingIDs = append(existingIDs, *resp.FindingId)
		} else {
			newFindings = append(newFindings, f)
		}
	}

	// ── Step 7: Save new findings via gRPC ────────────────────────────────────
	var newFindingIDs []string
	if len(newFindings) > 0 {
		inputs := make([]*findingv1.FindingInput, 0, len(newFindings))
		for _, f := range newFindings {
			inputs = append(inputs, toFindingInput(f, in.Options))
		}
		batchResp, err := uc.findingClient.BatchCreateFindings(ctx, &findingv1.BatchCreateFindingsRequest{
			TestId:       testResp.TestId,
			EngagementId: engResp.EngagementId,
			ProductId:    productID,
			Findings:     inputs,
		})
		if err != nil {
			return nil, fmt.Errorf("import scan: batch create findings: %w", err)
		}
		newFindingIDs = batchResp.FindingIds
	}

	// ── Step 8: Close old findings if configured ──────────────────────────────
	closedCount := 0
	if in.Options.CloseOldFindings {
		allCurrentIDs := append(newFindingIDs, existingIDs...)
		closeResp, err := uc.findingClient.CloseOldFindings(ctx, &findingv1.CloseOldFindingsRequest{
			TestId:                    testResp.TestId,
			EngagementId:              engResp.EngagementId,
			DeduplicationOnEngagement: in.Options.DeduplicationOnEngagement,
			ExcludeFindingIds:         allCurrentIDs,
			MitigatedById:             in.Options.RequestorUserID,
		})
		if err == nil {
			closedCount = int(closeResp.Closed)
		}
	}

	// ── Step 9: Publish completion event ──────────────────────────────────────
	_ = uc.eventPub.Publish(ctx, "defectdojo.scan.import.completed", map[string]interface{}{
		"import_id":       importID,
		"test_id":         testResp.TestId,
		"engagement_id":   engResp.EngagementId,
		"product_id":      productID,
		"product_type_id": productTypeID,
		"scan_type":       in.ScanType,
		"total":           len(findings),
		"new":             len(newFindingIDs),
		"closed":          closedCount,
	})

	return &ImportScanOutput{
		ImportID:       importID,
		TotalFindings:  len(findings),
		NewFindings:    len(newFindingIDs),
		ClosedFindings: closedCount,
		TestID:         testResp.TestId,
		EngagementID:   engResp.EngagementId,
	}, nil
}

func (uc *ImportScanUseCase) resolveProduct(ctx context.Context, opts scan.ImportOptions) (productID, productTypeID string, err error) {
	resp, err := uc.productClient.GetOrCreateProduct(ctx, &productv1.GetOrCreateProductRequest{
		Name:            opts.ProductName,
		ProductTypeName: opts.ProductTypeName,
		RequestorUserId: opts.RequestorUserID,
	})
	if err != nil {
		return "", "", err
	}
	return resp.ProductId, resp.ProductTypeId, nil
}

func toFindingInput(f *parser.ParsedFinding, opts scan.ImportOptions) *findingv1.FindingInput {
	return &findingv1.FindingInput{
		Title:            f.Title,
		Description:      f.Description,
		Mitigation:       f.Mitigation,
		Impact:           f.Impact,
		References:       f.References,
		Severity:         f.Severity,
		Cve:              f.CVE,
		Cwe:              int32(f.CWE),
		VulnIdFromTool:   f.VulnIDFromTool,
		CvssV3:           f.CVSSv3,
		CvssV3Score:      f.CVSSv3Score,
		Active:           opts.Active,
		Verified:         opts.Verified,
		ComponentName:    f.ComponentName,
		ComponentVersion: f.ComponentVersion,
		FilePath:         f.FilePath,
		LineNumber:       int32(f.LineNumber),
		Service:          opts.Service,
		HashCode:         f.HashCode,
		Tags:             f.Tags,
		Endpoints:        f.Endpoints,
	}
}
