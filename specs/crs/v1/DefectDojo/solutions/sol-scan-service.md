# ✅ COMPLETED — Solution: scan-service Extension

> **Covers**: CR-DD-002, CR-DD-003

## Lý do hợp nhất vào scan-service

`scan-service` hiện đã có:
- `internal/parsers/` — golang, java, nodejs, python, rust parsers
- `internal/adapters/` — adapter pattern
- `internal/infra/` — infra layer (MinIO, DB)
- `internal/domain/scan/`, `internal/domain/asset/`

→ Extend scan-service với security parser factory + import pipeline thay vì tạo scan-orchestrator riêng.

---

## CR-DD-002: Scan Import Pipeline & Parser Factory

### Cấu trúc mới

```
scan-service/internal/
├── domain/
│   ├── scan/              # ĐÃ CÓ — mở rộng
│   │   ├── import_options.go     # ImportOptions, ImportResult
│   │   ├── import_history.go     # TestImport entity
│   │   └── repository.go
│   └── parser/            # 🆕 MỚI
│       ├── interface.go   # Parser interface: GetFindings(), ScanType()
│       ├── factory.go     # ParserFactory port
│       └── entity.go      # ParsedFinding (pre-save)
│
├── usecase/
│   ├── import/            # 🆕 MỚI
│   │   ├── import_scan.go          # Full import pipeline (12 steps)
│   │   ├── reimport_scan.go        # Diff-based reimport
│   │   └── auto_create_context.go  # Get-or-create Product/Engagement/Test
│   └── parser/
│       └── list_parsers.go         # List available scan types
│
├── infra/
│   └── parser/            # 🆕 MỚI — Security parsers
│       ├── factory.go     # ParserFactory implementation
│       ├── registry.go    # Auto-registration via init()
│       │
│       # Container/SCA scanners
│       ├── trivy/         # Trivy Container, SCA
│       ├── grype/         # Grype Container SCA
│       ├── snyk/          # Snyk SCA
│       ├── cyclonedx/     # CycloneDX SBOM
│       │
│       # SAST scanners
│       ├── bandit/        # Python SAST
│       ├── semgrep/       # Multi-language SAST
│       ├── gosec/         # Go SAST
│       ├── spotbugs/      # Java SAST
│       ├── eslint/        # JavaScript SAST
│       ├── sonarqube/     # Code quality + SAST
│       ├── checkmarx/     # Enterprise SAST
│       ├── sarif/         # Generic SARIF (universal)
│       │
│       # DAST scanners
│       ├── owasp_zap/     # OWASP ZAP DAST
│       ├── burp/          # Burp Suite
│       ├── nikto/         # Web vulnerability
│       │
│       # Infrastructure/Cloud
│       ├── checkov/       # IaC (Terraform, K8s)
│       ├── tfsec/         # Terraform security
│       ├── trivy_iac/     # Trivy IaC mode
│       ├── aws_security_hub/ # AWS Security Hub
│       ├── gcp_security/  # GCP Security Command Center
│       │
│       # Network scanners
│       ├── nessus/        # Nessus
│       ├── nuclei/        # Nuclei
│       ├── openvas/       # OpenVAS
│       ├── qualys/        # Qualys
│       │
│       # Dependency checkers
│       ├── dependency_check/ # OWASP Dependency-Check
│       ├── retire_js/     # RetireJS
│       ├── npm_audit/     # npm audit
│       └── ... (150+ total)
```

### Domain Entities mới

```go
// scan-service/internal/domain/parser/interface.go
type Parser interface {
    GetFindings(ctx context.Context, file io.Reader, test *TestContext) ([]*ParsedFinding, error)
    ScanType() string
}

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

// scan-service/internal/domain/parser/entity.go
type ParsedFinding struct {
    Title         string
    Description   string
    Mitigation    string
    Impact        string
    References    string
    Severity      Severity

    CVE              string
    CWE              int
    VulnIDFromTool   string

    CVSSv3       string
    CVSSv3Score  *float64
    CVSSv4       string
    CVSSv4Score  *float64

    Active       bool
    Verified     bool
    FalsePositive bool

    ComponentName    string
    ComponentVersion string
    Service          string

    FilePath   string
    LineNumber  int
    SourceCode string

    UnsavedEndpoints       []string
    UnsavedRequestResponse *RequestResponse

    Date            *time.Time
    Tags            []string
    UniqueIDFromTool string

    // Computed during dedup
    HashCode string
}

// scan-service/internal/domain/scan/import_history.go
type TestImport struct {
    ID         string
    TestID     string
    ImportType string  // "import" | "reimport"
    Version    string
    BranchTag  string
    BuildID    string
    CommitHash string

    NewFindings    int
    ClosedFindings int
    Reactivated    int
    Untouched      int

    ScanFileID     string
    ImportSettings map[string]interface{}

    CreatedAt time.Time
}

// scan-service/internal/domain/scan/import_options.go
type ImportOptions struct {
    ScanType string
    File     io.Reader

    TestID          *string
    EngagementID    *string
    ProductID       *string
    ProductName     string
    EngagementName  string
    TestTitle       string

    AutoCreateContext      bool
    EngagementType        string

    Active                      bool
    Verified                    bool
    MinimumSeverity             Severity
    CloseOldFindings            bool
    CloseOldFindingsProductScope bool
    DoNotReactivate             bool
    DeduplicationOnEngagement   bool

    Version    string
    BuildID    string
    CommitHash string
    BranchTag  string
    Service    string

    GroupBy                           string
    CreateFindingGroupsForAllFindings bool

    Tags                []string
    ApplyTagsToFindings bool
    ApplyTagsToEndpoints bool

    EndpointsToAdd []string
    RequestorUserID string
}

type ImportResult struct {
    TestID         string
    TestImportID   string
    TotalFindings  int
    NewFindings    int
    ClosedFindings int
    Reactivated    int
    Untouched      int
    ScanType       string
    ImportType     string
}
```

### Import Pipeline Use Case

```go
// scan-service/internal/usecase/import/import_scan.go
// 12-step pipeline mirrors Django DefectDojo importer

func (uc *ImportScanUseCase) Execute(ctx context.Context, opts *domain.ImportOptions) (*domain.ImportResult, error) {
    // Step 1: Permission check (via finding-service gRPC CheckProductPermission)
    // Step 2: Store scan file async → MinIO
    // Step 3: Auto-create context (Product/Engagement/Test) via finding-service gRPC
    // Step 4: Get parser by scan_type (ParserFactory)
    // Step 5: Parse file → ParsedFindings[]
    // Step 6: Filter by minimum_severity
    // Step 7: Deduplicate (→ CR-DD-003 logic)
    // Step 8: BatchCreate new findings via gRPC → finding-service
    // Step 9: CloseOldFindings (if close_old_findings=true)
    // Step 10: Record import history (TestImport entity)
    // Step 11: Apply tags to findings
    // Step 12: Publish "scan.import.completed" event → NATS
}
```

### Parser Factory

```go
// scan-service/internal/infra/parser/factory.go
func NewParserFactory() *ParserFactory {
    f := &ParserFactory{parsers: make(map[string]domain.Parser)}

    // Register security parsers
    f.Register(trivy.NewParser())        // "Trivy Scan"
    f.Register(grype.NewParser())        // "Grype"
    f.Register(bandit.NewParser())       // "Bandit"
    f.Register(semgrep.NewParser())      // "Semgrep JSON Report"
    f.Register(gosec.NewParser())        // "Gosec Scanner"
    f.Register(checkov.NewParser())      // "Checkov Scan"
    f.Register(snyk.NewParser())         // "Snyk Scan"
    f.Register(owasp_zap.NewParser())    // "ZAP Scan"
    f.Register(burp.NewParser())         // "Burp Scan"
    f.Register(nessus.NewParser())       // "Nessus Scan"
    f.Register(nuclei.NewParser())       // "Nuclei Scan"
    f.Register(sonarqube.NewParser())    // "SonarQube Scan"
    f.Register(dependency_check.NewParser()) // "Dependency Check Scan"
    f.Register(cyclonedx.NewParser())    // "CycloneDX Scan"
    f.Register(sarif.NewParser())        // "SARIF"
    f.Register(retire_js.NewParser())    // "RetireJS Scan"
    f.Register(aws_security_hub.NewParser()) // "AWS Security Hub Scan"
    f.Register(qualys.NewParser())       // "Qualys Scan"
    // ... 150+ parsers total

    return f
}
```

### gRPC Client mới (calling finding-service)

```
scan-service/internal/infra/grpc/client/
├── finding.go    # BatchCreateFindings, CloseOldFindings, FindByHashCode, ...
└── product.go    # GetOrCreateEngagement, GetOrCreateTest, CheckPermission
```

### REST API mới

| Method | Path | Auth | Config |
|--------|------|------|--------|
| `POST` | `/api/v2/import-scan` | JWT | timeout=300s, maxBody=500MB |
| `POST` | `/api/v2/reimport-scan` | JWT | timeout=300s, maxBody=500MB |
| `GET` | `/api/v2/parsers` | JWT | cached 1h |
| `GET` | `/api/v2/test-imports` | JWT | — |
| `GET` | `/api/v2/test-imports/{id}` | JWT | — |

### Database Migration

```sql
-- test_imports (import history)
CREATE TABLE IF NOT EXISTS test_imports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_id UUID NOT NULL,
    import_type VARCHAR(20) NOT NULL,  -- import|reimport
    version VARCHAR(100),
    branch_tag VARCHAR(255),
    build_id VARCHAR(255),
    commit_hash VARCHAR(255),
    new_findings INTEGER DEFAULT 0,
    closed_findings INTEGER DEFAULT 0,
    reactivated INTEGER DEFAULT 0,
    untouched INTEGER DEFAULT 0,
    scan_file_id TEXT,
    import_settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_test_imports_test ON test_imports(test_id, created_at DESC);
```

### NATS Events

```
scan.import.started    {job_id, scan_type, product_id}
scan.import.completed  {test_import_id, test_id, product_id, new, closed, reactivated, untouched}
scan.import.failed     {job_id, error, scan_type}
finding.batch_created  {finding_ids, test_id, product_id, count}
```

---

## CR-DD-003: Finding Deduplication Engine

### Domain mới

```
scan-service/internal/domain/dedup/
├── service.go      # DeduplicationService interface
└── entity.go       # DedupContext, DedupResult, DedupAlgorithm
```

```go
// scan-service/internal/domain/dedup/service.go
type DeduplicationService interface {
    Deduplicate(
        ctx context.Context,
        newFindings []*parser.ParsedFinding,
        dedupCtx *DedupContext,
    ) (*DedupResult, error)
}

type DedupContext struct {
    TestID               string
    EngagementID         string
    ProductID            string
    OnEngagement         bool
    Algorithm            DedupAlgorithm
    FalsePositiveHistory bool
    MaxDuplicates        int    // default: 10
}

type DedupAlgorithm string
const (
    DedupAlgorithmHashCode DedupAlgorithm = "hash_code"
    DedupAlgorithmUniqueID DedupAlgorithm = "unique_id_from_tool"
    DedupAlgorithmLegacy   DedupAlgorithm = "legacy"
)

type DedupResult struct {
    NewFindings       []*parser.ParsedFinding
    DuplicateFindings []*parser.ParsedFinding
    Reactivated       []*ReactivatedFinding
    Untouched         []*ExistingFinding
}
```

### Infrastructure — Algorithm Implementations

```
scan-service/internal/infra/dedup/
├── hash_code.go     # SHA256(severity|title|cwe|description[:256]|sorted_endpoints)
├── unique_id.go     # Match by vuln_id_from_tool
├── legacy.go        # Endpoint URL + CWE/title matching
└── manager.go       # DuplicateManager (max_dupes enforcement)
```

**Hash computation** (mirrors Django dojo/utils.py):
```go
// computeHashCode: SHA256(severity|title|cwe|description[:256]|sorted_endpoints)
func computeHashCode(f *parser.ParsedFinding) string {
    h := sha256.New()
    parts := []string{
        strings.ToLower(string(f.Severity)),
        strings.ToLower(f.Title),
    }
    if f.CWE != 0 {
        parts = append(parts, fmt.Sprintf("%d", f.CWE))
    }
    if len(f.Description) > 0 {
        desc := f.Description
        if len(desc) > 256 {
            desc = desc[:256]
        }
        parts = append(parts, desc)
    }
    endpoints := append([]string{}, f.UnsavedEndpoints...)
    sort.Strings(endpoints)
    parts = append(parts, endpoints...)
    io.WriteString(h, strings.Join(parts, "|"))
    return fmt.Sprintf("%x", h.Sum(nil))
}
```

**Algorithm selection per scanner** (mirrors Django DEDUPLICATION_ALGORITHM_PER_PARSER):
```go
var dedupAlgorithmPerParser = map[string]DedupAlgorithm{
    "Snyk Scan":             DedupAlgorithmUniqueID,
    "SonarQube Scan":        DedupAlgorithmUniqueID,
    "Checkmarx Scan":        DedupAlgorithmUniqueID,
    "Veracode Scan":         DedupAlgorithmUniqueID,
    "Nuclei Scan":           DedupAlgorithmUniqueID,
    "Dependency Check Scan": DedupAlgorithmUniqueID,
    // Default: DedupAlgorithmHashCode
}
```

### Config

```yaml
# scan-service/config.yaml — THÊM section:
deduplication:
  default_algorithm: "hash_code"
  false_positive_history: false
  max_duplicates: 10
  delete_duplicates: false
```

### Acceptance Criteria

- [x] `POST /api/v2/import-scan` với file Trivy JSON tạo findings thành công
- [x] `auto_create_context=true` tạo Product/Engagement/Test nếu chưa tồn tại
- [x] `close_old_findings=true` đóng findings cũ không còn trong scan mới
- [x] `minimum_severity=High` lọc bỏ Medium/Low/Info
- [x] `GET /api/v2/parsers` trả về ≥ 20 scan types
- [x] Import cùng scan file 2 lần → lần 2 tất cả là `untouched`
- [x] Finding đã mitigated nhưng xuất hiện lại → `reactivated=1`
- [x] `false_positive_history=true`: finding hash trùng với FP → auto-mark FP
- [x] `max_duplicates=5`: khi có 6 duplicates, oldest 1 bị xóa
- [x] Hash computation đồng nhất: cùng data → cùng hash

## Implementation Status: ✅ DONE

> `scan-service/internal/usecase/import/import_scan.go` — 12-step import pipeline (permission check → MinIO → auto-create context → parser → parse → filter severity → dedup → batch create → close old → TestImport record → apply tags → NATS event)
> `scan-service/internal/infra/secparser/factory.go` — 21+ parsers registered (Trivy, Bandit, Semgrep, Gosec, Checkov, Snyk, ZAP, Burp, Nessus, Nuclei, SonarQube, DependencyCheck, CycloneDX, SARIF, RetireJS, AWS Security Hub, Qualys, etc.)
> `scan-service/internal/infra/dedup/engine.go` — 3 algorithms: hash_code (SHA256), unique_id_from_tool, legacy endpoint matching
> `scan-service/migrations/002_test_imports.sql` — test_imports table + 2 indexes
> `scan-service/internal/domain/scan/import_history.go` — TestImport entity: new/closed/reactivated/untouched counts
> `scan-service/internal/domain/parser/interface.go` — Parser interface + ParserFactory
