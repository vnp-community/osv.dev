// Package importuc implements the 12-step scan import pipeline.
// This is the core orchestration use case for DefectDojo-compatible scan imports.
package importuc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/osv/scan-service/internal/domain/importdomain"
)

var (
	// ErrForbidden is returned when the user lacks permission to import.
	ErrForbidden = errors.New("permission denied: scan:import required")
	// ErrTestIDRequired is returned when auto_create_context=false and test_id is missing.
	ErrTestIDRequired = errors.New("test_id is required when auto_create_context=false")
	// ErrUnknownScanType is returned when no parser handles the given scan type.
	ErrUnknownScanType = errors.New("unknown scan type")
)

// ─── Ports (interfaces that the use case depends on) ──────────────────────────

// ParserFactory resolves a parser by scan type.
type ParserFactory interface {
	GetParser(scanType string) (Parser, error)
	ListScanTypes() []string
}

// Parser parses a security scan file into normalized findings.
type Parser interface {
	GetFindings(ctx context.Context, file io.Reader, testCtx *ParseTestContext) ([]*ParsedFinding, error)
	ScanType() string
}

// ParseTestContext provides context during parsing.
type ParseTestContext struct {
	TestID       string
	EngagementID string
	ProductID    string
	ScanType     string
}

// ParsedFinding is the normalized output from a parser.
type ParsedFinding struct {
	Title            string
	Description      string
	Mitigation       string
	Impact           string
	References       string
	Severity         string
	CVE              string
	CWE              int
	VulnIDFromTool   string
	CVSSv3           string
	CVSSv3Score      *float64
	Active           bool
	Verified         bool
	FalsePositive    bool
	ComponentName    string
	ComponentVersion string
	FilePath         string
	LineNumber       int
	Service          string
	Tags             []string
	HashCode         string
	Endpoints        []string
}

// FindingServiceClient is the gRPC client for finding-service.
type FindingServiceClient interface {
	CheckProductPermission(ctx context.Context, req *CheckPermRequest) (bool, error)
	GetOrCreateEngagement(ctx context.Context, req *GetOrCreateEngagementRequest) (*GetOrCreateEngagementResponse, error)
	GetOrCreateTest(ctx context.Context, req *GetOrCreateTestRequest) (*GetOrCreateTestResponse, error)
	BatchCreateFindings(ctx context.Context, req *BatchCreateRequest) ([]string, int, error)
	CloseOldFindings(ctx context.Context, req *CloseOldRequest) ([]string, error)
	BulkAddTags(ctx context.Context, findingIDs []string, tags []string) error
}

// CheckPermRequest is the input for permission checks.
type CheckPermRequest struct {
	UserID     string
	ProductID  string
	Permission string
}

// GetOrCreateEngagementRequest is the input for engagement auto-create.
type GetOrCreateEngagementRequest struct {
	ProductID      string
	Name           string
	EngagementType string
	BuildID        string
	BranchTag      string
	CommitHash     string
}

// GetOrCreateEngagementResponse is the output from engagement auto-create.
type GetOrCreateEngagementResponse struct {
	EngagementID string
	Created      bool
}

// GetOrCreateTestRequest is the input for test auto-create.
type GetOrCreateTestRequest struct {
	EngagementID string
	ScanType     string
	Title        string
	Version      string
	BranchTag    string
	BuildID      string
	CommitHash   string
}

// GetOrCreateTestResponse is the output from test auto-create.
type GetOrCreateTestResponse struct {
	TestID  string
	Created bool
}

// BatchCreateRequest is the input for batch creating findings.
type BatchCreateRequest struct {
	Findings     []*ParsedFinding
	TestID       string
	EngagementID string
	ProductID    string
}

// CloseOldRequest is the input for closing old findings.
type CloseOldRequest struct {
	TestID          string
	ActiveHashCodes []string
	ProductScope    bool
	ProductID       string
	DoNotReactivate bool
}

// DedupEngine computes hash codes and identifies duplicate findings.
type DedupEngine interface {
	Deduplicate(ctx context.Context, findings []*ParsedFinding, testCtx *DedupContext) (*DedupResult, error)
}

// DedupContext provides context for deduplication.
type DedupContext struct {
	TestID       string
	EngagementID string
	ProductID    string
	OnEngagement bool
	ScanType     string
}

// DedupResult contains the outcome of deduplication.
type DedupResult struct {
	NewFindings [](*ParsedFinding)
	Duplicates  [](*ParsedFinding)
	Reactivated []string // finding IDs reactivated
	Untouched   []string // finding IDs that were already active
}

// ScanFileStorage uploads and downloads scan files (MinIO/S3).
type ScanFileStorage interface {
	Upload(ctx context.Context, filename string, data io.Reader) (objectKey string, err error)
}

// ImportHistoryRepository persists import history records.
type ImportHistoryRepository interface {
	Save(ctx context.Context, h *importdomain.TestImport) error
	FindByID(ctx context.Context, id string) (*importdomain.TestImport, error)
	ListByTest(ctx context.Context, testID string) ([]*importdomain.TestImport, error)
}

// EventPublisher publishes NATS events.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload map[string]any) error
}

// ─── Use Case ─────────────────────────────────────────────────────────────────

// ImportScanUseCase orchestrates the 12-step scan import pipeline.
type ImportScanUseCase struct {
	parserFactory ParserFactory
	findingClient FindingServiceClient
	storage       ScanFileStorage
	dedupEngine   DedupEngine
	importRepo    ImportHistoryRepository
	eventPub      EventPublisher
}

// New creates a new ImportScanUseCase with all required dependencies.
func New(
	pf ParserFactory,
	fc FindingServiceClient,
	s ScanFileStorage,
	de DedupEngine,
	ir ImportHistoryRepository,
	ep EventPublisher,
) *ImportScanUseCase {
	return &ImportScanUseCase{
		parserFactory: pf,
		findingClient: fc,
		storage:       s,
		dedupEngine:   de,
		importRepo:    ir,
		eventPub:      ep,
	}
}

// Execute runs the 12-step import pipeline.
//
// Steps:
//  1. Permission check
//  2. Read and store scan file in MinIO (before parsing — ensures file is safe even if parse fails)
//  3. Auto-create Product/Engagement/Test context (if auto_create_context=true)
//  4. Resolve parser by scan_type
//  5. Parse file into normalized findings
//  6. Filter by minimum_severity
//  7. Compute hash codes + deduplicate against existing findings
//  8. Batch-create new findings via gRPC → finding-service
//  9. Close old findings (if close_old_findings=true)
// 10. Record import history (TestImport entity)
// 11. Apply tags to new findings (if apply_tags_to_findings=true)
// 12. Publish scan.import.completed NATS event
func (uc *ImportScanUseCase) Execute(ctx context.Context, opts *importdomain.ImportOptions) (*importdomain.ImportResult, error) {
	jobID := uuid.New().String()

	_ = uc.eventPub.Publish(ctx, "scan.import.started", map[string]any{
		"job_id":    jobID,
		"scan_type": opts.ScanType,
		"product_id": safeStr(opts.ProductID),
	})

	// ═══════════════════════════════════════════
	// Step 1: Permission check
	// ═══════════════════════════════════════════
	if opts.ProductID != nil && opts.RequestorUserID != "" {
		allowed, err := uc.findingClient.CheckProductPermission(ctx, &CheckPermRequest{
			UserID:     opts.RequestorUserID,
			ProductID:  *opts.ProductID,
			Permission: "scan:import",
		})
		if err != nil || !allowed {
			return nil, ErrForbidden
		}
	}

	// ═══════════════════════════════════════════
	// Step 2: Read and store scan file in MinIO
	// ═══════════════════════════════════════════
	fileBytes, err := io.ReadAll(opts.File)
	if err != nil {
		return nil, fmt.Errorf("reading scan file: %w", err)
	}
	fileKey, err := uc.storage.Upload(ctx, opts.Filename, bytes.NewReader(fileBytes))
	if err != nil {
		return nil, fmt.Errorf("storing scan file: %w", err)
	}

	// ═══════════════════════════════════════════
	// Step 3: Auto-create Product/Engagement/Test
	// ═══════════════════════════════════════════
	testID, engagementID, productID, err := uc.resolveContext(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("resolving context: %w", err)
	}

	// ═══════════════════════════════════════════
	// Step 4: Get parser by scan_type
	// ═══════════════════════════════════════════
	p, err := uc.parserFactory.GetParser(opts.ScanType)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownScanType, opts.ScanType)
	}

	// ═══════════════════════════════════════════
	// Step 5: Parse scan file
	// ═══════════════════════════════════════════
	testCtx := &ParseTestContext{
		TestID:       testID,
		EngagementID: engagementID,
		ProductID:    productID,
		ScanType:     opts.ScanType,
	}
	parsedFindings, err := p.GetFindings(ctx, bytes.NewReader(fileBytes), testCtx)
	if err != nil {
		return nil, fmt.Errorf("parsing scan file (%s): %w", opts.ScanType, err)
	}

	// ═══════════════════════════════════════════
	// Step 6: Filter by minimum_severity
	// ═══════════════════════════════════════════
	if opts.MinimumSeverity != "" {
		parsedFindings = filterBySeverity(parsedFindings, opts.MinimumSeverity)
	}

	// ═══════════════════════════════════════════
	// Step 7: Deduplicate
	// ═══════════════════════════════════════════
	dedupCtx := &DedupContext{
		TestID:       testID,
		EngagementID: engagementID,
		ProductID:    productID,
		OnEngagement: opts.DeduplicationOnEngagement,
		ScanType:     opts.ScanType,
	}
	dedupResult, err := uc.dedupEngine.Deduplicate(ctx, parsedFindings, dedupCtx)
	if err != nil {
		return nil, fmt.Errorf("deduplication: %w", err)
	}

	// ═══════════════════════════════════════════
	// Step 8: Batch-create new findings via gRPC
	// ═══════════════════════════════════════════
	createdIDs, _, err := uc.findingClient.BatchCreateFindings(ctx, &BatchCreateRequest{
		Findings:     dedupResult.NewFindings,
		TestID:       testID,
		EngagementID: engagementID,
		ProductID:    productID,
	})
	if err != nil {
		return nil, fmt.Errorf("batch creating findings: %w", err)
	}

	// ═══════════════════════════════════════════
	// Step 9: Close old findings (if enabled)
	// ═══════════════════════════════════════════
	closedCount := 0
	if opts.CloseOldFindings {
		activeHashCodes := collectHashCodes(parsedFindings)
		closedIDs, _ := uc.findingClient.CloseOldFindings(ctx, &CloseOldRequest{
			TestID:          testID,
			ActiveHashCodes: activeHashCodes,
			ProductScope:    opts.CloseOldFindingsProductScope,
			ProductID:       productID,
			DoNotReactivate: opts.DoNotReactivate,
		})
		closedCount = len(closedIDs)
	}

	// ═══════════════════════════════════════════
	// Step 10: Record import history
	// ═══════════════════════════════════════════
	testImport := &importdomain.TestImport{
		ID:             uuid.New().String(),
		TestID:         testID,
		ImportType:     "import",
		Version:        opts.Version,
		BranchTag:      opts.BranchTag,
		BuildID:        opts.BuildID,
		CommitHash:     opts.CommitHash,
		NewFindings:    len(createdIDs),
		ClosedFindings: closedCount,
		Reactivated:    len(dedupResult.Reactivated),
		ScanFileKey:    fileKey,
		CreatedAt:      time.Now().UTC(),
	}
	_ = uc.importRepo.Save(ctx, testImport)

	// ═══════════════════════════════════════════
	// Step 11: Apply tags to new findings
	// ═══════════════════════════════════════════
	if opts.ApplyTagsToFindings && len(opts.Tags) > 0 && len(createdIDs) > 0 {
		_ = uc.findingClient.BulkAddTags(ctx, createdIDs, opts.Tags)
	}

	// ═══════════════════════════════════════════
	// Step 12: Publish scan.import.completed
	// ═══════════════════════════════════════════
	result := &importdomain.ImportResult{
		TestID:        testID,
		TestImportID:  testImport.ID,
		TotalFindings: len(parsedFindings),
		NewFindings:   len(createdIDs),
		Closed:        closedCount,
		Reactivated:   len(dedupResult.Reactivated),
		ScanType:      opts.ScanType,
		ImportType:    "import",
	}

	_ = uc.eventPub.Publish(ctx, "scan.import.completed", map[string]any{
		"job_id":         jobID,
		"test_import_id": testImport.ID,
		"test_id":        testID,
		"product_id":     productID,
		"new":            result.NewFindings,
		"closed":         result.Closed,
		"reactivated":    result.Reactivated,
		"scan_type":      opts.ScanType,
	})

	return result, nil
}

// ─── Private helpers ──────────────────────────────────────────────────────────

// resolveContext determines testID, engagementID, productID either from opts
// or by auto-creating via finding-service gRPC.
func (uc *ImportScanUseCase) resolveContext(ctx context.Context, opts *importdomain.ImportOptions) (testID, engagementID, productID string, err error) {
	if !opts.AutoCreateContext {
		if opts.TestID == nil {
			return "", "", "", ErrTestIDRequired
		}
		return *opts.TestID, safeStr(opts.EngagementID), safeStr(opts.ProductID), nil
	}

	// Auto-create engagement
	engResp, err := uc.findingClient.GetOrCreateEngagement(ctx, &GetOrCreateEngagementRequest{
		ProductID:      safeStr(opts.ProductID),
		Name:           opts.EngagementName,
		EngagementType: opts.EngagementType,
		BuildID:        opts.BuildID,
		BranchTag:      opts.BranchTag,
		CommitHash:     opts.CommitHash,
	})
	if err != nil {
		return "", "", "", fmt.Errorf("auto-create engagement: %w", err)
	}

	// Auto-create test
	testResp, err := uc.findingClient.GetOrCreateTest(ctx, &GetOrCreateTestRequest{
		EngagementID: engResp.EngagementID,
		ScanType:     opts.ScanType,
		Title:        opts.TestTitle,
		Version:      opts.Version,
		BranchTag:    opts.BranchTag,
		BuildID:      opts.BuildID,
		CommitHash:   opts.CommitHash,
	})
	if err != nil {
		return "", "", "", fmt.Errorf("auto-create test: %w", err)
	}

	return testResp.TestID, engResp.EngagementID, safeStr(opts.ProductID), nil
}

func filterBySeverity(findings []*ParsedFinding, minimum importdomain.Severity) []*ParsedFinding {
	minOrder := importdomain.SeverityOrder[minimum]
	var filtered []*ParsedFinding
	for _, f := range findings {
		if importdomain.SeverityOrder[importdomain.Severity(f.Severity)] >= minOrder {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func collectHashCodes(findings []*ParsedFinding) []string {
	codes := make([]string, 0, len(findings))
	for _, f := range findings {
		if f.HashCode != "" {
			codes = append(codes, f.HashCode)
		}
	}
	return codes
}

func safeStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
