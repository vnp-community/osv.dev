# ✅ COMPLETED — CR-DD-002 — Scan Import Pipeline & Parser Factory (150+ Parsers)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DD-002 |
| **Tiêu đề** | Scan Orchestrator — Import Pipeline, 150+ Parser Factory, Auto-Create Context |
| **Nguồn tham chiếu** | `django-DefectDojo/specs/services/04-scan-orchestrator-service.md`, `SRS.md §3.3 FR-IMP-01 to FR-IMP-05` |
| **Target Service** | `scan-service` (extend) hoặc **MỚI**: `scan-orchestrator-service` |
| **Ưu tiên** | 🔴 High |
| **Loại** | New Service / Major Feature |
| **Ngày tạo** | 2026-06-13 |

---

## 1. Tổng quan

OSV `scan-service` hiện có chức năng quản lý assets và schedule scans nhưng **không có**:
- **Import pipeline** — nhận file scan result → parse → save findings
- **Parser factory** — 150+ scanner parsers (Trivy, Bandit, Semgrep, ZAP, Nessus...)
- **Auto-create context** — tự động tạo Product/Engagement/Test nếu chưa tồn tại
- **Reimport logic** — diff old vs new findings (new/closed/reactivated/untouched)
- **Import history** — tracking TestImport records

---

## 2. Gap Analysis

### DefectDojo Scan Pipeline Flow

```
POST /api/v2/import-scan/
  │
  ├── 1. Validate permissions (product/engagement access)
  ├── 2. Store scan file → Minio/S3
  ├── 3. Auto-create context (Product/Engagement/Test) if needed
  ├── 4. Get parser by scan_type (ParserFactory)
  ├── 5. Parse file → ParsedFindings[]
  ├── 6. Filter by minimum_severity
  ├── 7. Deduplicate (hash_code | unique_id | legacy)
  ├── 8. BatchCreate new findings → Finding Service (gRPC)
  ├── 9. CloseOldFindings (if close_old_findings=true)
  ├── 10. Record import history (TestImport entity)
  ├── 11. Apply tags to findings/endpoints
  └── 12. Publish "scan.completed" event → NATS
```

### OSV scan-service hiện tại

```
scan-service/
├── assets/      ✅ Asset management
├── scans/       ✅ Scan scheduling
├── findings/    ⚠️ Basic (no dedup, no state machine)
└── parsers/     ❌ THIẾU
```

---

## 3. Service Architecture

```
scan-orchestrator/
├── cmd/server/main.go
│
├── internal/
│   ├── domain/
│   │   ├── scan/
│   │   │   ├── entity.go        # ScanImport, ImportOptions, ImportResult
│   │   │   ├── value_objects.go # ScanType, Severity
│   │   │   └── repository.go
│   │   ├── parser/
│   │   │   ├── interface.go     # Parser interface: GetFindings(), ScanType()
│   │   │   ├── factory.go       # ParserFactory port
│   │   │   └── entity.go        # ParsedFinding (pre-save)
│   │   ├── dedup/               # → CR-DD-003
│   │   │   └── service.go
│   │   └── importhistory/
│   │       ├── entity.go        # TestImport, TestImportAction
│   │       └── repository.go
│   │
│   ├── usecase/
│   │   ├── import/
│   │   │   ├── import_scan.go          # Full import pipeline
│   │   │   ├── reimport_scan.go        # Diff-based reimport
│   │   │   └── auto_create_context.go  # Get-or-create Product/Eng/Test
│   │   └── parser/
│   │       └── list_parsers.go         # List available scan types
│   │
│   ├── delivery/
│   │   ├── http/
│   │   │   ├── import_handler.go   # POST /api/v2/import-scan
│   │   │   ├── reimport_handler.go # POST /api/v2/reimport-scan
│   │   │   └── parsers_handler.go  # GET /api/v2/parsers
│   │   └── grpc/
│   │       └── scan_server.go
│   │
│   └── infra/
│       ├── parser/
│       │   ├── factory.go       # ParserFactory implementation
│       │   ├── registry.go      # Auto-registration
│       │   │
│       │   # 150+ parsers — one directory per scanner
│       │   ├── trivy/           # Container, SCA
│       │   ├── grype/           # Container SCA
│       │   ├── bandit/          # Python SAST
│       │   ├── semgrep/         # Multi-language SAST
│       │   ├── gosec/           # Go SAST
│       │   ├── checkov/         # IaC
│       │   ├── snyk/            # SCA
│       │   ├── owasp_zap/       # DAST
│       │   ├── burp/            # DAST
│       │   ├── nessus/          # Network scanner
│       │   ├── nuclei/          # Vulnerability scanner
│       │   ├── sonarqube/       # Code quality + SAST
│       │   ├── dependency_check/ # OWASP Dependency-Check
│       │   ├── cyclonedx/       # SBOM format
│       │   ├── sarif/           # Generic SARIF (SAST universal)
│       │   ├── retire_js/       # JS SCA
│       │   ├── aws_security_hub/ # Cloud
│       │   ├── qualys/          # Network scanner
│       │   └── ... (150+ total parsers)
│       │
│       ├── storage/
│       │   └── minio.go         # Scan file storage
│       ├── postgres/
│       │   └── scan_import_repo.go
│       └── grpc/client/
│           ├── product.go       # → product-service
│           └── finding.go       # → finding-service
```

---

## 4. Domain Entities

### 4.1 ImportOptions

```go
// domain/scan/entity.go
// Mirrors Python: dojo/importers/base_importer.py::BaseImporter options
type ImportOptions struct {
    // Required
    ScanType string    // "Trivy Scan", "Bandit", "OWASP ZAP Scan", etc.
    File     io.Reader

    // Context resolution
    TestID          *string
    EngagementID    *string
    ProductID       *string
    ProductName     string  // for auto-create
    EngagementName  string  // for auto-create
    TestTitle       string  // for auto-create

    AutoCreateContext      bool
    EngagementType        string  // Interactive | CI/CD

    // Import behavior
    Active                      bool    // default: true
    Verified                    bool    // default: false
    MinimumSeverity             Severity // filter below this severity
    CloseOldFindings            bool
    CloseOldFindingsProductScope bool
    DoNotReactivate             bool
    DeduplicationOnEngagement   bool

    // CI/CD metadata
    Version    string
    BuildID    string
    CommitHash string
    BranchTag  string
    Service    string

    // Finding groups
    GroupBy                           string // "component_name+version"|"file_path"
    CreateFindingGroupsForAllFindings bool

    // Tags
    Tags                []string
    ApplyTagsToFindings bool
    ApplyTagsToEndpoints bool

    // Extra endpoints
    EndpointsToAdd []string

    // Requestor
    RequestorUserID string
}

// ImportResult — mirrors Django importer return value
type ImportResult struct {
    TestID         string
    TestImportID   string
    TotalFindings  int
    NewFindings    int
    ClosedFindings int
    Reactivated    int
    Untouched      int
    ScanType       string
    ImportType     string // "import" | "reimport"
}

// ParsedFinding — output từ parser, chưa lưu DB
// Mirrors Python: dojo/models.py::Finding (pre-save representation)
type ParsedFinding struct {
    Title         string
    Description   string
    Mitigation    string
    Impact        string
    References    string
    Severity      Severity

    // CVE info
    CVE            string
    CWE            int
    VulnIDFromTool string

    // CVSS
    CVSSv3      string
    CVSSv3Score *float64
    CVSSv4      string
    CVSSv4Score *float64

    // Status
    Active        bool
    Verified      bool
    FalsePositive bool

    // Component
    ComponentName    string
    ComponentVersion string
    Service          string

    // File location (SAST)
    FilePath   string
    LineNumber  int
    SourceCode string

    // Network/DAST
    UnsavedEndpoints       []string  // URLs to create
    UnsavedRequestResponse *RequestResponse

    // Metadata
    Date           *time.Time
    ScannerConfidence string
    Tags           []string
    UniqueIDFromTool string

    // Computed during dedup
    HashCode string
}

type RequestResponse struct {
    Request  string
    Response string
}

// TestImport — import history record
// Mirrors Python: dojo/models.py::Test_Import
type TestImport struct {
    ID          string
    TestID      string
    ImportType  string // "import" | "reimport"
    Version     string
    BranchTag   string
    BuildID     string
    CommitHash  string

    // Statistics
    NewFindings     int
    ClosedFindings  int
    Reactivated     int
    Untouched       int

    // File storage
    ScanFileID string

    // Settings snapshot
    ImportSettings map[string]interface{}

    CreatedAt time.Time
}
```

### 4.2 Parser Interface

```go
// domain/parser/interface.go
// Mirrors Python: dojo/tools/*/parser.py interface
type Parser interface {
    // GetFindings parses the file and returns findings
    GetFindings(ctx context.Context, file io.Reader, test *TestContext) ([]*ParsedFinding, error)

    // ScanType returns the scanner name (must match DefectDojo scan type list)
    ScanType() string
}

// DynamicParser supports multiple tests from one file (e.g., sonarqube)
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
```

---

## 5. Parser Factory

```go
// infra/parser/factory.go
type ParserFactory struct {
    parsers map[string]domain.Parser
}

func NewParserFactory() *ParserFactory {
    f := &ParserFactory{parsers: make(map[string]domain.Parser)}

    // Register all parsers (150+)
    f.Register(trivy.NewParser())
    f.Register(grype.NewParser())
    f.Register(bandit.NewParser())
    f.Register(semgrep.NewParser())
    f.Register(gosec.NewParser())
    f.Register(checkov.NewParser())
    f.Register(snyk.NewParser())
    f.Register(owasp_zap.NewParser())
    f.Register(burp.NewParser())
    f.Register(nessus.NewParser())
    f.Register(nuclei.NewParser())
    f.Register(sonarqube.NewParser())
    f.Register(dependency_check.NewParser())
    f.Register(cyclonedx.NewParser())     // CycloneDX SBOM
    f.Register(sarif.NewParser())          // Generic SARIF
    f.Register(retire_js.NewParser())
    f.Register(aws_security_hub.NewParser())
    f.Register(qualys.NewParser())
    // ... (150+ total, một file per scanner)

    return f
}
```

---

## 6. Import Use Case

```go
// usecase/import/import_scan.go
// Core import pipeline — mirrors Python: dojo/importers/importer.py::Importer.process_findings()

func (uc *ImportScanUseCase) Execute(ctx context.Context, opts *domain.ImportOptions) (*domain.ImportResult, error) {
    // 1. Permission check
    if err := uc.checkPermission(ctx, opts); err != nil {
        return nil, err
    }

    // 2. Store scan file async (non-blocking)
    go uc.storeFile(opts.File, opts.ScanType)

    // 3. Auto-create context (Product/Engagement/Test)
    testCtx, err := uc.resolveContext(ctx, opts)
    if err != nil {
        return nil, err
    }

    // 4. Parse
    parser, err := uc.parserFactory.GetParser(opts.ScanType)
    parsedFindings, err := parser.GetFindings(ctx, opts.File, testCtx)

    // 5. Filter by minimum severity
    parsedFindings = uc.filterBySeverity(parsedFindings, opts.MinimumSeverity)

    // 6. Deduplicate (→ CR-DD-003)
    dedupResults, err := uc.dedupService.Deduplicate(ctx, parsedFindings, &DedupContext{...})

    // 7. BatchCreate new findings via gRPC → finding-service
    newFindingIDs, err := uc.findingClient.BatchCreateFindings(ctx, dedupResults.NewFindings, testCtx)

    // 8. Close old findings (if configured)
    closedIDs, err := uc.findingClient.CloseOldFindings(ctx, testCtx.TestID, newFindingIDs, opts.CloseOldFindings)

    // 9. Record import history
    testImport, _ := uc.importHistRepo.CreateTestImport(ctx, &domain.TestImport{
        TestID:         testCtx.TestID,
        ImportType:     "import",
        NewFindings:    len(newFindingIDs),
        ClosedFindings: len(closedIDs),
        Reactivated:    len(dedupResults.Reactivated),
        Untouched:      len(dedupResults.Untouched),
    })

    // 10. Apply tags
    if opts.ApplyTagsToFindings && len(opts.Tags) > 0 {
        uc.findingClient.ApplyTags(ctx, newFindingIDs, opts.Tags)
    }

    // 11. Publish event
    uc.eventPub.Publish(ctx, &events.ScanCompleted{...})

    return &domain.ImportResult{
        TestID:        testCtx.TestID,
        TestImportID:  testImport.ID,
        NewFindings:   len(newFindingIDs),
        ClosedFindings: len(closedIDs),
        Reactivated:   len(dedupResults.Reactivated),
        Untouched:     len(dedupResults.Untouched),
    }, nil
}
```

---

## 7. Sample Parser: Trivy

```go
// infra/parser/trivy/parser.go
// Mirrors Python: dojo/tools/trivy/parser.py

type TrivyParser struct{}

func (p *TrivyParser) ScanType() string { return "Trivy Scan" }

func (p *TrivyParser) GetFindings(ctx context.Context, file io.Reader, test *domain.TestContext) ([]*domain.ParsedFinding, error) {
    var report TrivyReport
    if err := json.NewDecoder(file).Decode(&report); err != nil {
        return nil, fmt.Errorf("trivy: decode: %w", err)
    }

    var findings []*domain.ParsedFinding
    for _, result := range report.Results {
        for _, vuln := range result.Vulnerabilities {
            findings = append(findings, &domain.ParsedFinding{
                Title:            fmt.Sprintf("%s in %s %s", vuln.VulnerabilityID, vuln.PkgName, vuln.InstalledVersion),
                Description:      vuln.Description,
                Severity:         mapSeverity(vuln.Severity),
                CVE:              vuln.VulnerabilityID,
                VulnIDFromTool:   vuln.VulnerabilityID,
                ComponentName:    vuln.PkgName,
                ComponentVersion: vuln.InstalledVersion,
                Mitigation:       fmt.Sprintf("Upgrade to %s", vuln.FixedVersion),
                References:       strings.Join(vuln.References, "\n"),
                Active:           true,
            })
        }
    }
    return findings, nil
}

// mapSeverity: Trivy → DefectDojo severity
func mapSeverity(s string) domain.Severity {
    switch strings.ToUpper(s) {
    case "CRITICAL": return domain.SeverityCritical
    case "HIGH":     return domain.SeverityHigh
    case "MEDIUM":   return domain.SeverityMedium
    case "LOW":      return domain.SeverityLow
    default:         return domain.SeverityInfo
    }
}
```

---

## 8. REST API

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `POST` | `/api/v2/import-scan` | JWT | Import scan file |
| `POST` | `/api/v2/reimport-scan` | JWT | Reimport scan (diff) |
| `GET` | `/api/v2/parsers` | JWT | List available scan types |
| `GET` | `/api/v2/test-imports` | JWT | List import history |
| `GET` | `/api/v2/test-imports/{id}` | JWT | Get import history |

### Import Request

```
POST /api/v2/import-scan
Content-Type: multipart/form-data
Authorization: Bearer <jwt>

Fields:
  scan_type: "Trivy Scan"           (required)
  file: <binary scan file>          (required)
  product_name: "Portal"            (required if no product_id)
  engagement_name: "Q2 2026 Review" (required if no engagement_id)
  test_title: "Trivy Container Scan" (optional)
  active: true
  verified: false
  close_old_findings: true
  auto_create_context: true
  minimum_severity: "Medium"
  tags: "ci,trivy,container"

Response: {
  "test": { "id": "uuid", "title": "...", "scan_type": "Trivy Scan" },
  "statistics": {
    "created": 15,
    "closed": 3,
    "reactivated": 2,
    "untouched": 120
  }
}
```

---

## 9. NATS Events

```
scan.import.started    {job_id, scan_type, product_id}
scan.import.completed  {test_import_id, test_id, product_id, new: N, closed: N, reactivated: N}
scan.import.failed     {job_id, error, scan_type}
```

---

## 10. Acceptance Criteria

- [x] `POST /api/v2/import-scan` với file Trivy JSON tạo findings thành công
- [x] `auto_create_context=true` tạo Product/Engagement/Test nếu chưa tồn tại
- [x] `close_old_findings=true` đóng findings cũ không còn trong scan mới
- [x] `minimum_severity=High` lọc bỏ Medium/Low/Info findings
- [x] `GET /api/v2/parsers` trả về danh sách ≥ 20 scan types
- [x] Bandit SAST, Semgrep, OWASP ZAP parsers hoạt động
- [x] CycloneDX SBOM parser hoạt động
- [x] SARIF generic parser hoạt động (Microsoft SARIF format)
- [x] Import history được lưu (TestImport với statistics N/C/R/U)
- [x] `POST /api/v2/reimport-scan` với cùng file → `untouched=N, created=0`
- [x] NATS `scan.import.completed` event được publish
- [x] Scan file được upload lên Minio/S3 async

## Implementation Status: ✅ DONE

> Implemented in `scan-service` (not a separate `scan-orchestrator` — consolidated per solution design)
> `scan-service/internal/usecase/import/import_scan.go` — full 12-step pipeline
> `scan-service/internal/infra/secparser/factory.go` — 21+ parsers (Trivy, Bandit, Semgrep, Gosec, Checkov, Snyk, ZAP, Burp, Nessus, Nuclei, SonarQube, DependencyCheck, CycloneDX, SARIF, RetireJS, AWS Security Hub, Qualys, Grype, etc.)
> `scan-service/internal/domain/scan/import_history.go` — TestImport: new/closed/reactivated/untouched counts
> `scan-service/migrations/002_test_imports.sql` — test_imports table + 2 indexes
