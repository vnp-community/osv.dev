# ✅ COMPLETED — Solution: finding-service Extension

> **Covers**: CR-DD-001, CR-DD-004, CR-DD-005, CR-DD-009

## Lý do hợp nhất vào finding-service

`finding-service` hiện đã có domains: `product`, `product_type`, `engagement`, `test`, `finding`, `sla`, `report`, `audit`.  
Các CRs này mở rộng trực tiếp các domain này mà không cần service riêng.

---

## CR-DD-001: Product/Engagement/Test Hierarchy

### Domains cần thêm

```
finding-service/internal/domain/
├── product/           # ĐÃ CÓ — cần thêm fields
├── product_type/      # ĐÃ CÓ — cần thêm fields
├── engagement/        # ĐÃ CÓ — cần thêm fields
├── test/              # ĐÃ CÓ — cần thêm fields
├── member/            # 🆕 THÊM MỚI
│   ├── entity.go      # ProductMember, ProductTypeMember
│   └── repository.go
└── tool/              # 🆕 THÊM MỚI
    ├── entity.go      # ToolConfiguration (scanner credentials)
    └── repository.go
```

### Entities cần thêm/mở rộng

**Product** — thêm fields:
```go
type Product struct {
    // Fields hiện có...
    // THÊM MỚI:
    BusinessCriticality  Criticality // very high|high|medium|low|very low|none
    Platform             Platform    // web|mobile|desktop|api|iot
    Lifecycle            Lifecycle   // construction|production|retirement
    Origin               Origin      // internal|contractor|outsourced|open source|purchased
    SLAConfigurationID   *string     // FK → sla-service
    EnableSimpleRiskAcceptance bool
    EnableFullRiskAcceptance   bool
    EnableProductTagInheritance bool
    Tags                 []string
}
```

**Engagement** — thêm fields:
```go
type Engagement struct {
    // Fields hiện có...
    // THÊM MỚI:
    Type   EngagementType   // Interactive | CI/CD
    Status EngagementStatus // Not Started|In Progress|Completed|Blocked|...
    BuildID    string
    CommitHash string
    BranchTag  string
    SourceCodeManagementURI string
    DeduplicationOnEngagement bool
    BuildServerID           *string   // FK ToolConfiguration
    OrchestrationEngineID   *string   // FK ToolConfiguration
}
```

**Test** — thêm fields:
```go
type Test struct {
    // Fields hiện có...
    // THÊM MỚI:
    ScanType        string  // "Trivy Scan", "Bandit", etc.
    PercentComplete int
    BuildID         string
    CommitHash      string
    BranchTag       string
    Version         string
    Tags            []string
}
```

**ProductMember** — entity mới:
```go
// finding-service/internal/domain/member/entity.go
type ProductMember struct {
    ID        string
    ProductID string
    UserID    string
    RoleID    string  // Owner|Maintainer|Writer|API_Importer|Reader
    CreatedAt time.Time
}

type ProductTypeMember struct {
    ID            string
    ProductTypeID string
    UserID        string
    RoleID        string
    CreatedAt     time.Time
}
```

**ToolConfiguration** — entity mới:
```go
// finding-service/internal/domain/tool/entity.go
type ToolConfiguration struct {
    ID          string
    Name        string
    Description string
    ToolType    string  // "GitHub"|"GitLab"|"Jira"|"Slack"|etc.
    URL         string
    AuthType    string  // api_key|http_basic|ssh|bearer
    Username    string
    PasswordEnc string  // AES-256 encrypted
    APIKeyEnc   string  // AES-256 encrypted
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### Use Cases mới

```
finding-service/internal/usecase/
├── member/
│   ├── add_product_member.go
│   ├── remove_product_member.go
│   └── check_product_permission.go    # RBAC check (dùng bởi gateway)
├── tool/
│   ├── create_tool_config.go
│   ├── update_tool_config.go
│   └── delete_tool_config.go
└── engagement/
    └── auto_close_expired.go           # Cron daily 07:00
```

### gRPC Contract extension (finding.proto)

```protobuf
service FindingService {
    // Hiện có...

    // 🆕 CR-DD-001: Product/Engagement/Test operations
    rpc GetOrCreateEngagement(GetOrCreateEngagementRequest) returns (GetOrCreateEngagementResponse);
    rpc GetOrCreateTest(GetOrCreateTestRequest) returns (GetOrCreateTestResponse);
    rpc UpdateEngagementTimestamps(UpdateEngagementTimestampsRequest) returns (UpdateEngagementTimestampsResponse);
    rpc UpdateTestMetadata(UpdateTestMetadataRequest) returns (UpdateTestMetadataResponse);

    // 🆕 RBAC
    rpc GetProductMembers(GetProductMembersRequest) returns (GetProductMembersResponse);
    rpc CheckProductPermission(CheckProductPermissionRequest) returns (CheckProductPermissionResponse);

    // 🆕 SLA link
    rpc GetSLAConfiguration(GetSLAConfigurationRequest) returns (GetSLAConfigurationResponse);
}

message CheckProductPermissionRequest {
    string user_id    = 1;
    string product_id = 2;
    string permission = 3;  // "product:view"|"finding:add"|"scan:import"
}

message GetOrCreateEngagementRequest {
    string product_id    = 1;
    string name          = 2;
    optional string engagement_type = 3;
    optional string build_id        = 4;
    optional string branch_tag      = 5;
    optional string commit_hash     = 6;
}
```

### REST API mới (finding-service)

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET/POST` | `/api/v2/product-types` | JWT | CRUD ProductType |
| `GET/POST` | `/api/v2/products` | JWT | CRUD Product |
| `POST` | `/api/v2/products/{id}/members` | JWT/Owner | Add member |
| `DELETE` | `/api/v2/products/{id}/members/{uid}` | JWT/Owner | Remove member |
| `GET/POST` | `/api/v2/engagements` | JWT | CRUD Engagement |
| `POST` | `/api/v2/engagements/{id}/close` | JWT | Close |
| `POST` | `/api/v2/engagements/{id}/reopen` | JWT | Reopen |
| `GET/POST` | `/api/v2/tests` | JWT | CRUD Test |
| `GET/POST` | `/api/v2/tool-configurations` | JWT/Admin | CRUD Tool Config |

### Database Migrations

```sql
-- Mở rộng bảng products (đã tồn tại)
ALTER TABLE products ADD COLUMN IF NOT EXISTS platform VARCHAR(50);
ALTER TABLE products ADD COLUMN IF NOT EXISTS lifecycle VARCHAR(50);
ALTER TABLE products ADD COLUMN IF NOT EXISTS origin VARCHAR(50);
ALTER TABLE products ADD COLUMN IF NOT EXISTS business_criticality VARCHAR(50);
ALTER TABLE products ADD COLUMN IF NOT EXISTS sla_configuration_id UUID;
ALTER TABLE products ADD COLUMN IF NOT EXISTS enable_simple_risk_acceptance BOOLEAN DEFAULT FALSE;
ALTER TABLE products ADD COLUMN IF NOT EXISTS enable_full_risk_acceptance BOOLEAN DEFAULT FALSE;
ALTER TABLE products ADD COLUMN IF NOT EXISTS enable_product_tag_inheritance BOOLEAN DEFAULT FALSE;
ALTER TABLE products ADD COLUMN IF NOT EXISTS tags TEXT[] DEFAULT '{}';

-- Mở rộng bảng engagements
ALTER TABLE engagements ADD COLUMN IF NOT EXISTS engagement_type VARCHAR(50) DEFAULT 'Interactive';
ALTER TABLE engagements ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'In Progress';
ALTER TABLE engagements ADD COLUMN IF NOT EXISTS build_id VARCHAR(255);
ALTER TABLE engagements ADD COLUMN IF NOT EXISTS commit_hash VARCHAR(255);
ALTER TABLE engagements ADD COLUMN IF NOT EXISTS branch_tag VARCHAR(255);
ALTER TABLE engagements ADD COLUMN IF NOT EXISTS source_code_management_uri TEXT;
ALTER TABLE engagements ADD COLUMN IF NOT EXISTS deduplication_on_engagement BOOLEAN DEFAULT FALSE;

-- Mở rộng bảng tests
ALTER TABLE tests ADD COLUMN IF NOT EXISTS scan_type VARCHAR(255);
ALTER TABLE tests ADD COLUMN IF NOT EXISTS percent_complete INTEGER DEFAULT 0;
ALTER TABLE tests ADD COLUMN IF NOT EXISTS build_id VARCHAR(255);
ALTER TABLE tests ADD COLUMN IF NOT EXISTS commit_hash VARCHAR(255);
ALTER TABLE tests ADD COLUMN IF NOT EXISTS branch_tag VARCHAR(255);
ALTER TABLE tests ADD COLUMN IF NOT EXISTS version VARCHAR(100);
ALTER TABLE tests ADD COLUMN IF NOT EXISTS tags TEXT[] DEFAULT '{}';

-- Bảng mới: product_members
CREATE TABLE IF NOT EXISTS product_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    role_id VARCHAR(50) NOT NULL,  -- Owner|Maintainer|Writer|API_Importer|Reader
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (product_id, user_id)
);

-- Bảng mới: tool_configurations
CREATE TABLE IF NOT EXISTS tool_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(200) NOT NULL,
    description TEXT,
    tool_type VARCHAR(100),
    url TEXT,
    auth_type VARCHAR(50),
    username VARCHAR(200),
    password_enc TEXT,
    api_key_enc TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### NATS Events (publish)

```
product.type.created    {product_type_id, name, created_by}
product.created         {product_id, product_type_id, name, sla_config_id, created_by}
product.updated         {product_id, changes}
product.member.added    {product_id, user_id, role}
product.member.removed  {product_id, user_id}
engagement.created      {engagement_id, product_id, type, name}
engagement.closed       {engagement_id, product_id, reason}
test.created            {test_id, engagement_id, product_id, scan_type}
```

---

## CR-DD-004: Finding State Machine & Bulk Operations

### Domain changes (finding entity extension)

```go
// finding-service/internal/domain/finding/entity.go — THÊM fields
type Finding struct {
    // Đã có...
    // THÊM MỚI:
    CVSSv4         string
    CVSSv4Score    *float64
    FalsePositive  bool   // false_p (rename/alias)
    OutOfScope     bool
    RiskAccepted   bool
    FindingGroupID *string
    DuplicateFindingID *string  // self-referential FK
    SourceCode     string
    VulnIDFromTool string
    HashCode       string
    InheritedTags  []string
}

// State machine
func (f *Finding) CurrentState() FindingState {
    switch {
    case f.Duplicate:     return StateDuplicate
    case f.FalsePositive: return StateFalsePositive
    case f.OutOfScope:    return StateOutOfScope
    case f.RiskAccepted:  return StateRiskAccepted
    case f.IsMitigated:   return StateMitigated
    default:              return StateActive
    }
}
```

### Use Cases mới

```
finding-service/internal/usecase/finding/
├── close.go                  # Active → Mitigated
├── reopen.go                 # Mitigated → Active
├── mark_false_positive.go    # Active → FalsePositive
├── undo_false_positive.go    # FalsePositive → Active
├── accept_risk.go            # Active → RiskAccepted
├── mark_out_of_scope.go      # Active → OutOfScope
├── bulk.go                   # Bulk operations (close/reopen/tag/delete)
├── note_add.go               # Add note to finding
└── cvss_compute.go           # CVSS v3/v4 score computation
```

### New domains

```
finding-service/internal/domain/
├── group/
│   ├── entity.go       # FindingGroup
│   └── repository.go
├── note/
│   ├── entity.go       # FindingNote (private/public)
│   └── repository.go
└── attachment/
    ├── entity.go       # FileAttachment
    └── repository.go
```

### gRPC Extensions

```protobuf
service FindingService {
    // 🆕 State transitions
    rpc CloseFinding(CloseFindingRequest) returns (CloseFindingResponse);
    rpc ReopenFinding(ReopenFindingRequest) returns (ReopenFindingResponse);
    rpc MarkFalsePositive(MarkFPRequest) returns (MarkFPResponse);
    rpc MarkOutOfScope(MarkOutOfScopeRequest) returns (MarkOutOfScopeResponse);
    rpc MarkRiskAccepted(MarkRiskAcceptedRequest) returns (MarkRiskAcceptedResponse);
    rpc ReactivateRiskAccepted(ReactivateRiskAcceptedRequest) returns (ReactivateRiskAcceptedResponse);

    // 🆕 Dedup support (used by scan-service)
    rpc FindByHashCode(FindByHashCodeRequest) returns (FindByHashCodeResponse);
    rpc FindByUniqueID(FindByUniqueIDRequest) returns (FindByUniqueIDResponse);
    rpc ListDuplicates(ListDuplicatesRequest) returns (ListDuplicatesResponse);
    rpc ExistsFalsePositiveByHash(ExistsFPByHashRequest) returns (ExistsFPByHashResponse);

    // 🆕 Bulk
    rpc BulkUpdateFindings(BulkUpdateRequest) returns (BulkUpdateResponse);

    // 🆕 Notes
    rpc AddNote(AddNoteRequest) returns (AddNoteResponse);
    rpc ListNotes(ListNotesRequest) returns (ListNotesResponse);

    // 🆕 SLA
    rpc BatchUpdateSLADates(BatchUpdateSLADatesRequest) returns (BatchUpdateSLADatesResponse);
    rpc ListFindingsForSLACheck(ListFindingsForSLACheckRequest) returns (stream Finding);

    // 🆕 Report/Metrics
    rpc GetSeverityCounts(GetSeverityCountsRequest) returns (GetSeverityCountsResponse);
    rpc CountFindings(CountFindingsRequest) returns (CountFindingsResponse);
    rpc ListFindingsForReport(ListFindingsForReportRequest) returns (stream Finding);
    rpc CloseOldFindings(CloseOldFindingsRequest) returns (CloseOldFindingsResponse);
}
```

### Database Migrations

```sql
-- Mở rộng findings table
ALTER TABLE findings ADD COLUMN IF NOT EXISTS cvss_v4 VARCHAR(255);
ALTER TABLE findings ADD COLUMN IF NOT EXISTS cvss_v4_score DECIMAL(4,1);
ALTER TABLE findings ADD COLUMN IF NOT EXISTS false_p BOOLEAN DEFAULT FALSE;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS out_of_scope BOOLEAN DEFAULT FALSE;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS risk_accepted BOOLEAN DEFAULT FALSE;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS finding_group_id UUID;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS duplicate_finding_id UUID REFERENCES findings(id);
ALTER TABLE findings ADD COLUMN IF NOT EXISTS source_code TEXT;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS vuln_id_from_tool VARCHAR(255);
ALTER TABLE findings ADD COLUMN IF NOT EXISTS hash_code VARCHAR(64);
ALTER TABLE findings ADD COLUMN IF NOT EXISTS inherited_tags TEXT[] DEFAULT '{}';

-- finding_groups
CREATE TABLE IF NOT EXISTS finding_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    test_id UUID NOT NULL,
    jira_issue_key VARCHAR(100),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- finding_notes
CREATE TABLE IF NOT EXISTS finding_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    author_id UUID NOT NULL,
    content TEXT NOT NULL,
    edit_count INTEGER DEFAULT 0,
    is_private BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- finding_files
CREATE TABLE IF NOT EXISTS finding_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100),
    size_bytes BIGINT,
    storage_key TEXT NOT NULL,
    uploaded_by_id UUID,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_findings_hash ON findings(hash_code);
CREATE INDEX IF NOT EXISTS idx_findings_vuln_id ON findings(vuln_id_from_tool, product_id);
```

### NATS Events

```
finding.status_changed       {finding_id, product_id, old_state, new_state, by_user_id}
finding.bulk_updated         {finding_ids, operation, product_id}
finding.false_positive_marked {finding_id, product_id}
finding.duplicate_detected   {finding_id, duplicate_of_id}
```

---

## CR-DD-005: Risk Acceptance Management

### Domain mới

```
finding-service/internal/domain/risk_acceptance/
├── entity.go       # RiskAcceptance
└── repository.go
```

```go
// finding-service/internal/domain/risk_acceptance/entity.go
type RiskAcceptance struct {
    ID               string
    Name             string
    ProductID        string
    AcceptedByID     string
    ExpirationDate   *time.Time
    Notes            string
    ProofFileID      *string
    ReactivateExpired bool
    ReactivateNoteText string
    RestartSLAOnReactivation bool
    IsExpired        bool
    FindingIDs       []string
    CreatedAt        time.Time
    UpdatedAt        time.Time
}
```

### Use Cases mới

```
finding-service/internal/usecase/risk_acceptance/
├── create.go             # Create RA + mark findings risk_accepted
├── delete.go             # Delete RA + reactivate findings
├── remove_finding.go     # Remove single finding from RA
└── check_expiry.go       # Cron: daily 06:00 check expired RAs
```

### Scheduler (finding-service)

```go
// finding-service/internal/scheduler/cron.go — THÊM:
scheduler.AddFunc("0 6 * * *", uc.CheckRiskAcceptanceExpiry.Execute)   // Daily 06:00
scheduler.AddFunc("30 6 * * *", uc.NotifyRiskAcceptanceExpiringSoon.Execute) // Daily 06:30
scheduler.AddFunc("0 7 * * *", uc.AutoCloseExpiredEngagements.Execute) // Daily 07:00
```

### Database Migrations

```sql
CREATE TABLE IF NOT EXISTS risk_acceptances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(300) NOT NULL,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    accepted_by_id UUID NOT NULL,
    expiration_date DATE,
    notes TEXT,
    proof_file_id TEXT,
    reactivate_expired BOOLEAN DEFAULT FALSE,
    reactivate_note_text TEXT,
    restart_sla_on_reactivation BOOLEAN DEFAULT FALSE,
    is_expired BOOLEAN DEFAULT FALSE,
    finding_ids UUID[] DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ra_product ON risk_acceptances(product_id);
CREATE INDEX IF NOT EXISTS idx_ra_expiry ON risk_acceptances(expiration_date)
    WHERE is_expired = FALSE AND expiration_date IS NOT NULL;
```

### NATS Events

```
risk_acceptance.created       {id, product_id, finding_ids, expiration_date, accepted_by}
risk_acceptance.updated       {id, product_id, changes}
risk_acceptance.expired       {id, product_id, finding_ids, expired_at}
risk_acceptance.expiring_soon {id, product_id, expiration_date, days_remaining}
```

---

## CR-DD-009: Report Service + Product Grading

### Domains mới/mở rộng

```
finding-service/internal/domain/
└── report/             # ĐÃ CÓ — mở rộng
    ├── entity.go       # Report, ProductGrade, ProductMetrics
    └── repository.go
```

**ProductGrade** — entity mới:
```go
type ProductGrade struct {
    ProductID      string
    Grade          Grade   // A|B|C|D|F
    Score          float64
    CriticalCount  int
    HighCount      int
    MediumCount    int
    LowCount       int
    SLABreachCount int
    SLACompliance  float64
    ComputedAt     time.Time
}

// computeGrade — weighted scoring
// Score = 100 - (Critical*50) - (High*10) - (Medium*3) - (Low*1)
// A:>=90, B:>=80, C:>=70, D:>=60, F:<60
```

### Use Cases mới

```
finding-service/internal/usecase/
├── generatereport/          # ĐÃ CÓ — mở rộng
│   ├── generate_pdf.go      # HTML→PDF via Gotenberg
│   ├── generate_xlsx.go     # Excelize
│   ├── generate_json.go
│   └── generate_csv.go
└── grade/                   # 🆕 MỚI
    ├── compute_product_grade.go
    └── get_product_metrics.go
```

### Infrastructure mới

```
finding-service/internal/infra/
├── pdf/
│   └── gotenberg.go    # HTTP→Gotenberg service for HTML→PDF
├── xlsx/
│   └── generator.go    # Excelize
└── storage/
    └── minio.go        # Report file storage (30-day TTL)
```

### REST API mới

| Method | Path | Mô tả |
|--------|------|-------|
| `POST` | `/api/v2/reports` | Request report (async) |
| `GET` | `/api/v2/reports/{id}` | Poll status |
| `GET` | `/api/v2/reports/{id}/download` | Download file |
| `DELETE` | `/api/v2/reports/{id}` | Delete report |
| `GET` | `/api/v2/metrics/products` | All products metrics |
| `GET` | `/api/v2/metrics/products/{id}` | Specific product metrics |
| `GET` | `/api/v2/metrics/findings/trends` | 30-day trend |
| `GET` | `/api/v2/metrics/sla-compliance` | SLA compliance |
| `GET` | `/api/v2/product-grades` | All products grades |
| `GET` | `/api/v2/product-grades/{id}` | Specific product grade |

### NATS Events

```
report.generated    {report_id, product_id, format, file_url}
report.failed       {report_id, product_id, error}
```

---

## Acceptance Criteria tổng hợp

### CR-DD-001
- [x] `POST /api/v2/products` tạo Product với SLA Config assignment
- [x] `POST /api/v2/engagements` tạo Engagement linked to Product
- [x] gRPC `GetOrCreateEngagement` hoạt động (idempotent)
- [x] gRPC `CheckProductPermission` trả về đúng RBAC
- [x] Auto-close expired engagements chạy daily 07:00

### CR-DD-004
- [x] `POST /api/v2/findings/{id}/close` → Active→Mitigated
- [x] `POST /api/v2/findings/{id}/reopen` → Mitigated→Active
- [x] Transition không hợp lệ → 409 Conflict
- [x] Bulk close 100 findings trong 1 request
- [x] CVSS v3 vector → cvssv3_score tính tự động

### CR-DD-005
- [x] `POST /api/v2/risk-acceptances` tạo RA + mark findings risk_accepted
- [x] `DELETE /api/v2/risk-acceptances/{id}` xóa RA + reactivate findings
- [x] Scheduled daily check expired RAs

### CR-DD-009
- [x] `POST /api/v2/reports` với format=pdf → returns pending status
- [x] Poll status → eventually shows completed với download URL
- [x] Grade A: 0 Critical + 0 High → score ≥ 90
- [x] Grade F: 2+ Critical → score ≤ 0

## Implementation Status: ✅ DONE

> `finding-service/internal/domain/{member,tool,group,note,riskacceptance,report}/` — all new domains implemented
> `finding-service/internal/domain/product,engagement,test/` — extended with new fields (SLA link, business_criticality, platform, lifecycle, etc.)
> `finding-service/internal/domain/finding/` — State Machine: 6 states (Active/Mitigated/FalsePositive/OutOfScope/RiskAccepted/Duplicate) + CVSS v3/v4 scoring
> `finding-service/internal/usecase/riskacceptance/risk_acceptance.go` — Create, Delete, Remove finding, daily expiry check
> `finding-service/internal/usecase/grading/{compute_grade,report_grading}.go` — weighted scoring A-F (Critical×-10, High×-5, etc.)
> `finding-service/internal/usecase/report/generate.go` — async PDF/XLSX/CSV/JSON, 5min timeout, MinIO storage
> `finding-service/migrations/{001..013}*.sql` — all schema changes: products extension, engagements, tests, product_members, tool_configurations, risk_acceptances, finding_groups, finding_notes, finding_files, reports
> `finding-service/internal/delivery/grpc/server/finding_server.go` — gRPC: GetOrCreateEngagement, CheckProductPermission, ExistsFalsePositiveByHash, GetSeverityCounts, CountFindings, ReactivateFindings
