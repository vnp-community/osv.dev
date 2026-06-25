# TASK-DD-008 — Scan Import Pipeline (12 Steps)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-008 |
| **Service** | `scan-service` |
| **CR** | CR-DD-002 |
| **Phase** | 1 — Foundation |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-DD-006 (finding-service gRPC), TASK-DD-009 (parsers) |
| **Estimated effort** | 2 ngày |

## Context

Implement use case `ImportScanUseCase` với 12 bước pipeline, bao gồm: permission check, file upload to MinIO, auto-create context (Product/Engagement/Test via finding-service gRPC), parse scan file, filter severity, deduplicate, batch-create findings, close old findings, record history, tag application, publish NATS event. Cũng implement `ReimportScanUseCase` (diff-based).

## Reference

- Solution: [`sol-scan-service.md § CR-DD-002`](../solutions/sol-scan-service.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/
```

## Files to Create

```
internal/domain/scan/
├── import_options.go       # ImportOptions, ImportResult structs
└── import_history.go       # TestImport entity

internal/domain/parser/
├── interface.go            # Parser interface
├── factory.go              # ParserFactory port (interface)
└── entity.go               # ParsedFinding, TestContext

internal/usecase/import/
├── import_scan.go          # Main 12-step pipeline
├── reimport_scan.go        # Diff-based reimport
└── auto_create_context.go  # Get-or-create Product/Engagement/Test

internal/infra/grpc/client/
├── finding.go              # gRPC client for finding-service
└── config.go               # gRPC client config

internal/delivery/http/
├── import_handler.go       # POST /api/v2/import-scan
└── parser_handler.go       # GET /api/v2/parsers

internal/infra/storage/
└── minio.go                # Scan file upload/download
```

## Implementation Spec

### `internal/domain/scan/import_options.go`

```go
package scan

import (
    "io"
    "time"
)

type Severity string
const (
    SeverityCritical Severity = "Critical"
    SeverityHigh     Severity = "High"
    SeverityMedium   Severity = "Medium"
    SeverityLow      Severity = "Low"
    SeverityInfo     Severity = "Info"
)

var SeverityOrder = map[Severity]int{
    SeverityCritical: 5,
    SeverityHigh:     4,
    SeverityMedium:   3,
    SeverityLow:      2,
    SeverityInfo:     1,
}

type ImportOptions struct {
    ScanType string
    File     io.Reader
    Filename string

    // Context identifiers (at least one needed if AutoCreateContext=false)
    TestID          *string
    EngagementID    *string
    ProductID       *string

    // Auto-create context
    AutoCreateContext    bool
    ProductName         string  // used when auto-creating
    EngagementName      string
    TestTitle           string
    EngagementType      string  // "Interactive"|"CI/CD"

    // Filter
    Active              bool
    Verified            bool
    MinimumSeverity     Severity
    CloseOldFindings    bool
    CloseOldFindingsProductScope bool
    DoNotReactivate     bool
    DeduplicationOnEngagement bool

    // Metadata
    Version    string
    BuildID    string
    CommitHash string
    BranchTag  string
    Service    string

    // Grouping
    GroupBy                           string  // "" | "component_name" | "file_path" | "finding_title"
    CreateFindingGroupsForAllFindings bool

    // Tags
    Tags                []string
    ApplyTagsToFindings bool

    // Endpoints
    EndpointsToAdd []string

    // Auth
    RequestorUserID string
}

type ImportResult struct {
    TestID        string
    TestImportID  string
    TotalFindings int
    NewFindings   int
    Closed        int
    Reactivated   int
    Untouched     int
    ScanType      string
    ImportType    string  // "import" | "reimport"
}
```

### `internal/domain/scan/import_history.go`

```go
package scan

import "time"

type TestImport struct {
    ID         string
    TestID     string
    ImportType string // "import"|"reimport"
    Version    string
    BranchTag  string
    BuildID    string
    CommitHash string

    NewFindings    int
    ClosedFindings int
    Reactivated    int
    Untouched      int

    ScanFileKey    string // MinIO object key
    ImportSettings map[string]interface{}
    CreatedAt      time.Time
}
```

### `internal/domain/parser/interface.go`

```go
package parser

import (
    "context"
    "io"
)

// Parser parses a security scan output file into structured findings
type Parser interface {
    // GetFindings parses file and returns ParsedFindings
    GetFindings(ctx context.Context, file io.Reader, test *TestContext) ([]*ParsedFinding, error)
    // ScanType returns the name of this scanner (e.g., "Trivy Scan")
    ScanType() string
}

// DynamicParser can also extract test/sub-scan info from the file
type DynamicParser interface {
    Parser
    GetTests(ctx context.Context, scanType string, file io.Reader) ([]*ParserTest, error)
}

type TestContext struct {
    TestID       string
    EngagementID string
    ProductID    string
    ScanType     string
}

type ParserTest struct {
    Name        string
    ScanType    string
    Description string
}
```

### `internal/domain/parser/entity.go`

```go
package parser

import "time"

type ParsedFinding struct {
    Title         string
    Description   string
    Mitigation    string
    Impact        string
    References    string
    Severity      string

    CVE  string
    CWE  int

    CVSSv3      string
    CVSSv3Score *float64
    CVSSv4      string
    CVSSv4Score *float64

    Active        bool
    Verified      bool
    FalsePositive bool
    Duplicate     bool

    ComponentName    string
    ComponentVersion string
    Service          string

    FilePath   string
    LineNumber  int
    SourceCode string

    UnsavedEndpoints []string

    Date             *time.Time
    Tags             []string
    VulnIDFromTool   string
    UniqueIDFromTool string

    // Computed during dedup
    HashCode string
}
```

### `internal/usecase/import/import_scan.go`

```go
package importuc

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "log/slog"
    "time"

    "github.com/google/uuid"
    "github.com/osv/services/scan-service/internal/domain/parser"
    "github.com/osv/services/scan-service/internal/domain/scan"
)

type ImportScanUseCase struct {
    parserFactory ParserFactory
    findingClient FindingServiceClient
    storage       ScanFileStorage
    dedupEngine   DedupEngine
    importRepo    ImportHistoryRepository
    eventPub      EventPublisher
}

// Execute runs the 12-step import pipeline
func (uc *ImportScanUseCase) Execute(ctx context.Context, opts *scan.ImportOptions) (*scan.ImportResult, error) {
    jobID := uuid.New().String()
    slog.InfoContext(ctx, "import scan started", "job_id", jobID, "scan_type", opts.ScanType)

    uc.eventPub.Publish(ctx, "scan.import.started", map[string]any{
        "job_id": jobID, "scan_type": opts.ScanType,
    })

    // ═══════════════════════════════════════════
    // Step 1: Permission check
    // ═══════════════════════════════════════════
    if opts.ProductID != nil {
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
    // Step 2: Store scan file in MinIO (async-safe: store first before processing)
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
    // Step 3: Auto-create Product/Engagement/Test context
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
        return nil, fmt.Errorf("unknown scan type %q: %w", opts.ScanType, err)
    }

    // ═══════════════════════════════════════════
    // Step 5: Parse file
    // ═══════════════════════════════════════════
    testCtx := &parser.TestContext{
        TestID: testID, EngagementID: engagementID, ProductID: productID, ScanType: opts.ScanType,
    }
    parsedFindings, err := p.GetFindings(ctx, bytes.NewReader(fileBytes), testCtx)
    if err != nil {
        return nil, fmt.Errorf("parsing scan file: %w", err)
    }

    // ═══════════════════════════════════════════
    // Step 6: Filter by minimum_severity
    // ═══════════════════════════════════════════
    if opts.MinimumSeverity != "" {
        parsedFindings = filterBySeverity(parsedFindings, opts.MinimumSeverity)
    }

    // ═══════════════════════════════════════════
    // Step 7: Deduplicate (compute hash codes + check existing)
    // ═══════════════════════════════════════════
    dedupCtx := &DedupContext{
        TestID: testID, EngagementID: engagementID, ProductID: productID,
        OnEngagement: opts.DeduplicationOnEngagement,
    }
    dedupResult, err := uc.dedupEngine.Deduplicate(ctx, parsedFindings, dedupCtx)
    if err != nil {
        return nil, fmt.Errorf("deduplication: %w", err)
    }

    // ═══════════════════════════════════════════
    // Step 8: Batch-create new findings via gRPC → finding-service
    // ═══════════════════════════════════════════
    createdIDs, skipped, err := uc.findingClient.BatchCreateFindings(ctx, &BatchCreateRequest{
        Findings:  toNewFindingData(dedupResult.NewFindings, opts),
        TestID:    testID,
        ProductID: productID,
    })
    if err != nil {
        return nil, fmt.Errorf("batch creating findings: %w", err)
    }
    _ = skipped

    // ═══════════════════════════════════════════
    // Step 9: Close old findings (if enabled)
    // ═══════════════════════════════════════════
    closedCount := 0
    if opts.CloseOldFindings {
        activeHashes := collectHashCodes(parsedFindings)
        closedIDs, _ := uc.findingClient.CloseOldFindings(ctx, &CloseOldRequest{
            TestID:          testID,
            ActiveHashCodes: activeHashes,
            ProductScope:    opts.CloseOldFindingsProductScope,
            ProductID:       productID,
            DoNotReactivate: opts.DoNotReactivate,
        })
        closedCount = len(closedIDs)
    }

    // ═══════════════════════════════════════════
    // Step 10: Record import history
    // ═══════════════════════════════════════════
    testImport := &scan.TestImport{
        ID:             uuid.New().String(),
        TestID:         testID,
        ImportType:     "import",
        Version:        opts.Version,
        BranchTag:      opts.BranchTag,
        BuildID:        opts.BuildID,
        CommitHash:     opts.CommitHash,
        NewFindings:    len(createdIDs),
        ClosedFindings: closedCount,
        ScanFileKey:    fileKey,
        CreatedAt:      time.Now(),
    }
    uc.importRepo.Save(ctx, testImport)

    // ═══════════════════════════════════════════
    // Step 11: Apply tags to findings
    // ═══════════════════════════════════════════
    if opts.ApplyTagsToFindings && len(opts.Tags) > 0 {
        uc.findingClient.BulkAddTags(ctx, createdIDs, opts.Tags)
    }

    // ═══════════════════════════════════════════
    // Step 12: Publish scan.import.completed NATS event
    // ═══════════════════════════════════════════
    result := &scan.ImportResult{
        TestID: testID, TestImportID: testImport.ID,
        TotalFindings: len(parsedFindings),
        NewFindings:   len(createdIDs),
        Closed:        closedCount,
        ScanType:      opts.ScanType,
        ImportType:    "import",
    }
    uc.eventPub.Publish(ctx, "scan.import.completed", map[string]any{
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

// resolveContext handles auto-create Product/Engagement/Test
func (uc *ImportScanUseCase) resolveContext(ctx context.Context, opts *scan.ImportOptions) (testID, engagementID, productID string, err error) {
    if !opts.AutoCreateContext {
        if opts.TestID == nil {
            return "", "", "", ErrTestIDRequired
        }
        return *opts.TestID, *opts.EngagementID, *opts.ProductID, nil
    }
    // Get or create Engagement
    engResp, err := uc.findingClient.GetOrCreateEngagement(ctx, &GetOrCreateEngagementRequest{
        ProductID:      *opts.ProductID,
        Name:           opts.EngagementName,
        EngagementType: opts.EngagementType,
        BuildID:        opts.BuildID,
        BranchTag:      opts.BranchTag,
        CommitHash:     opts.CommitHash,
    })
    if err != nil {
        return "", "", "", err
    }
    // Get or create Test
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
        return "", "", "", err
    }
    return testResp.TestID, engResp.EngagementID, *opts.ProductID, nil
}

func filterBySeverity(findings []*parser.ParsedFinding, minimum scan.Severity) []*parser.ParsedFinding {
    minOrder := scan.SeverityOrder[minimum]
    var filtered []*parser.ParsedFinding
    for _, f := range findings {
        if scan.SeverityOrder[scan.Severity(f.Severity)] >= minOrder {
            filtered = append(filtered, f)
        }
    }
    return filtered
}
```

### `internal/delivery/http/import_handler.go`

```go
package http

// POST /api/v2/import-scan
// Content-Type: multipart/form-data
// Fields:
//   file          (required) — scan output file
//   scan_type     (required) — e.g. "Trivy Scan"
//   product_id    (required unless auto_create_context=true with product_name)
//   engagement_id (optional)
//   test_id       (optional)
//   minimum_severity   (optional) — Critical|High|Medium|Low|Info
//   close_old_findings (optional, bool)
//   active             (optional, bool, default true)
//   verified           (optional, bool, default false)
//   tags               (optional, comma-separated)
//   version            (optional)
//   build_id           (optional)
//   commit_hash        (optional)
//   branch_tag         (optional)
// Response 201: ImportResult JSON
```

## Acceptance Criteria

- [x] `POST /api/v2/import-scan` với Trivy JSON file → 201 Created với ImportResult
- [x] `auto_create_context=true` tạo Engagement+Test mới nếu chưa tồn tại
- [x] `minimum_severity=High` → Info/Low/Medium findings bị lọc bỏ (filterBySeverity)
- [x] `close_old_findings=true` → findings cũ (không có trong scan mới) bị close
- [x] `POST /api/v2/import-scan` không có product_id và test_id → 400 Bad Request
- [x] NATS `scan.import.completed` published với đúng fields
- [x] Scan file được upload lên MinIO trước khi parse (file bảo toàn ngay cả khi parse fail)
- [x] `POST /api/v2/reimport-scan` → ImportResult có reactivated count _(implemented)_
- [x] `GET /api/v2/parsers` → list có ít nhất 20 scan types _(implemented)_
- [x] `GET /api/v2/test-imports/{id}` → ImportHistory object _(implemented)_
- [x] File size 100MB xử lý thành công _(verified)_

## Implementation Status: ✅ DONE (core pipeline)

> `internal/usecase/import/import_scan.go` — 12-step pipeline đầy đủ
> Bước 1-12: permission, file storage, context auto-create, parse, filter, dedup, batch-create, close-old, history, tags, event
