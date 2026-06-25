# CR-OVS-002 — Finding Service: Vulnerability Lifecycle Management, SLA, Deduplication

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OVS-002 |
| **Tiêu đề** | Finding Service — Vulnerability Finding Lifecycle (Active/Mitigated/FalsePositive/RiskAccepted), SLA Policy, Hash-based Deduplication, Audit Trail |
| **Nguồn tham chiếu** | `OpenVulnScan/specs/services/06-finding-service.md`, `OpenVulnScan/docs/PRD.md §F-006` |
| **Target Service** | **MỚI**: `finding-service` (port gRPC: 50060) |
| **Ưu tiên** | 🔴 High |
| **Loại** | New Service |
| **Ngày tạo** | 2026-06-14 |
| **Ngày implement** | 2026-06-17 |
| **Trạng thái** | ✅ Implemented |

---

## 0. Implementation Status

> **Trạng thái**: ✅ **IMPLEMENTED** — 2026-06-17

| Task | File | Trạng thái |
|------|------|------------|
| Finding domain entity + state machine | `finding-service/internal/domain/finding/entity.go` | ✅ Done |
| Finding state machine transitions | `finding-service/internal/domain/finding/state_machine.go` | ✅ Done |
| Finding CVSS helpers | `finding-service/internal/domain/finding/cvss.go` | ✅ Done |
| Audit entity + logger | `finding-service/internal/domain/audit/entity.go`, `audit/logger.go` | ✅ Done |
| Engagement entity | `finding-service/internal/domain/engagement/entity.go` | ✅ Done |
| Finding HTTP handler | `finding-service/internal/delivery/http/finding_handler.go` | ✅ Done |
| Bulk operations handler | `finding-service/internal/delivery/http/bulk_handler.go` | ✅ Done |
| SLA handler | `finding-service/internal/delivery/http/sla_handler.go` | ✅ Done |
| Note handler | `finding-service/internal/delivery/http/note_handler.go` | ✅ Done |
| Risk acceptance handler | `finding-service/internal/delivery/http/risk_acceptance_handler.go` | ✅ Done |
| Product handler | `finding-service/internal/delivery/http/product_handler.go` | ✅ Done |
| Engagement handler | `finding-service/internal/delivery/http/engagement_handler.go` | ✅ Done |
| NATS finding consumer | `finding-service/internal/delivery/nats/consumer.go` | ✅ Done |
| Finding gRPC server | `finding-service/internal/delivery/grpc/server/finding_server.go` | ✅ Done |
| Finding PostgreSQL repo | `finding-service/internal/infra/postgres/finding_repo.go` | ✅ Done |
| SLA queries | `finding-service/internal/infra/postgres/sla_queries.go` | ✅ Done |
| Audit PostgreSQL repo | `finding-service/internal/infra/persistence/postgres/audit_repo.go` | ✅ Done |

**Chi tiết implementation**:
- **State Machine**: 6 trạng thái (active/mitigated/false_positive/risk_accepted/out_of_scope/duplicate), `VALID_TRANSITIONS` map
- **SLA Policy**: Critical=7d, High=30d, Medium=90d, Low=180d; `sla_expiration_date` auto-set khi import
- **Hash Dedup**: SHA-256 từ `(title+cve_id+product_id+component)`, `is_duplicate` flag
- **Audit Trail**: mọi state change → `FindingAudit` record với before/after state snapshot
- **Bulk ops**: `bulk/close`, `bulk/reopen`, `bulk/assign` với transaction atomicity
- **NATS consumer**: subscribe `scan.scan.completed` → import findings từ scan-service qua gRPC

---

## 1. Tổng quan

OSV chỉ track CVE database. OpenVulnScan thêm **Finding Service** — trung tâm quản lý vulnerability findings sau khi scan, với:
- **Finding Lifecycle**: active → mitigated → false_positive → risk_accepted → out_of_scope
- **SLA Policy**: Critical=7 ngày, High=30 ngày, Medium=90 ngày, Low=180 ngày
- **Deduplication**: Hash-based, tự động detect duplicate findings
- **Audit Trail**: Mọi thay đổi trạng thái đều được ghi lại
- **Finding Grouping**: Gom nhóm related findings

---

## 2. Gap Analysis

| Feature | OSV | OpenVulnScan |
|---------|-----|-------------|
| Vulnerability finding CRUD | ❌ | ✅ |
| Finding state machine | ❌ | ✅ 6 states |
| SLA deadlines per severity | ❌ | ✅ Critical:7d, High:30d |
| Hash-based deduplication | ❌ | ✅ SHA-256(title+component+cve) |
| Audit trail per finding | ❌ | ✅ |
| Mark false positive | ❌ | ✅ |
| Accept risk | ❌ | ✅ |
| CVSSv3 + CVSSv4 tracking | ❌ | ✅ |
| Finding tagging | ❌ | ✅ |
| Product/Engagement context | ❌ | ✅ DefectDojo hierarchy |
| Finding groups | ❌ | ✅ |

---

## 3. Domain Model

### 3.1 Finding Entity

```go
// finding-service/internal/domain/finding/entity.go

type Severity string
const (
    SeverityCritical Severity = "Critical"  // Numerical: 4
    SeverityHigh     Severity = "High"      // Numerical: 3
    SeverityMedium   Severity = "Medium"    // Numerical: 2
    SeverityLow      Severity = "Low"       // Numerical: 1
    SeverityInfo     Severity = "Info"      // Numerical: 0
)

func (s Severity) Numerical() int {
    switch s {
    case SeverityCritical: return 4
    case SeverityHigh:     return 3
    case SeverityMedium:   return 2
    case SeverityLow:      return 1
    default:               return 0
    }
}

type Finding struct {
    ID                uuid.UUID
    Title             string
    Description       string
    Mitigation        string
    Impact            string
    References        string
    Severity          Severity
    NumericalSeverity int     // derived from Severity

    // CVE/CWE
    CVE            string    // "CVE-2021-44228"
    CWE            int       // 20 (for CWE-20)
    VulnIDFromTool string    // tool-specific ID (ZAP alert ID, etc.)
    CVSSv3         string    // "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H"
    CVSSv3Score    *float64  // 10.0
    CVSSv4         string
    CVSSv4Score    *float64

    // State flags — CurrentState() derives logical state
    Active        bool  // true = vulnerability present
    Verified      bool  // true = confirmed by human
    FalsePositive bool
    Duplicate     bool
    OutOfScope    bool
    IsMitigated   bool
    RiskAccepted  bool

    // Timestamps
    Date               time.Time  // When finding was created/discovered
    MitigatedAt        *time.Time
    MitigatedByID      *uuid.UUID
    LastReviewed       *time.Time
    LastStatusUpdate   *time.Time
    SLAExpirationDate  *time.Time

    // Context (Product/Engagement/Test hierarchy)
    TestID             uuid.UUID
    EngagementID       uuid.UUID
    ProductID          uuid.UUID
    DuplicateFindingID *uuid.UUID  // Points to original if Duplicate=true
    FindingGroupID     *uuid.UUID

    // Location
    ComponentName    string  // "apache-log4j2"
    ComponentVersion string  // "2.14.1"
    Service          string  // "api-gateway"
    FilePath         string  // "/src/main/java/..."
    LineNumber       int

    // Deduplication
    HashCode string  // SHA-256(title + component + version + cve_id)

    // Tags
    Tags          []string
    InheritedTags []string

    CreatedAt time.Time
    UpdatedAt time.Time
}

func NewFinding(title string, severity Severity, testID, engagementID, productID uuid.UUID) *Finding {
    f := &Finding{
        ID:                uuid.New(),
        Title:             title,
        Severity:          severity,
        NumericalSeverity: severity.Numerical(),
        Active:            true,
        Date:              time.Now().UTC(),
        TestID:            testID,
        EngagementID:      engagementID,
        ProductID:         productID,
        Tags:              []string{},
        InheritedTags:     []string{},
        CreatedAt:         time.Now().UTC(),
        UpdatedAt:         time.Now().UTC(),
    }
    f.computeHash()
    return f
}

// computeHash — SHA-256 based deduplication hash
func (f *Finding) computeHash() {
    h := sha256.New()
    fmt.Fprintf(h, "%s|%s|%s|%s", f.Title, f.ComponentName, f.ComponentVersion, f.CVE)
    f.HashCode = hex.EncodeToString(h.Sum(nil))
}
```

### 3.2 State Machine

```go
// finding-service/internal/domain/finding/state_machine.go

type FindingState string
const (
    StateActive        FindingState = "active"
    StateMitigated     FindingState = "mitigated"
    StateFalsePositive FindingState = "false_positive"
    StateRiskAccepted  FindingState = "risk_accepted"
    StateOutOfScope    FindingState = "out_of_scope"
    StateDuplicate     FindingState = "duplicate"
)

// CurrentState derives state from boolean flags
// Priority: Duplicate > FalsePositive > OutOfScope > RiskAccepted > Mitigated > Active
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

// State transition methods
func (f *Finding) Close(mitigatedByID *uuid.UUID) error {
    if f.CurrentState() != StateActive { return ErrInvalidTransition }
    now := time.Now().UTC()
    f.IsMitigated = true
    f.Active = false
    f.MitigatedAt = &now
    f.MitigatedByID = mitigatedByID
    f.LastStatusUpdate = &now
    f.UpdatedAt = now
    return nil
}

func (f *Finding) Reopen() error {
    if f.CurrentState() == StateDuplicate { return ErrCannotReopenDuplicate }
    now := time.Now().UTC()
    f.IsMitigated = false
    f.FalsePositive = false
    f.RiskAccepted = false
    f.OutOfScope = false
    f.Active = true
    f.MitigatedAt = nil
    f.MitigatedByID = nil
    f.LastStatusUpdate = &now
    f.UpdatedAt = now
    return nil
}

func (f *Finding) MarkFalsePositive() error {
    if f.CurrentState() != StateActive { return ErrInvalidTransition }
    now := time.Now().UTC()
    f.FalsePositive = true
    f.Active = false
    f.LastStatusUpdate = &now
    f.UpdatedAt = now
    return nil
}

func (f *Finding) AcceptRisk() error {
    if f.CurrentState() != StateActive { return ErrInvalidTransition }
    now := time.Now().UTC()
    f.RiskAccepted = true
    f.LastStatusUpdate = &now
    f.UpdatedAt = now
    return nil
}

func (f *Finding) MarkOutOfScope() error {
    if f.CurrentState() != StateActive { return ErrInvalidTransition }
    now := time.Now().UTC()
    f.OutOfScope = true
    f.Active = false
    f.LastStatusUpdate = &now
    return nil
}
```

### 3.3 SLA Configuration

```go
// finding-service/internal/domain/sla/entity.go

type SLAConfiguration struct {
    ID        uuid.UUID
    ProductID *uuid.UUID  // nil = global default
    Critical  int         // days (default: 7)
    High      int         // days (default: 30)
    Medium    int         // days (default: 90)
    Low       int         // days (default: 180)
    CreatedAt time.Time
    UpdatedAt time.Time
}

var DefaultSLADays = map[string]int{
    "Critical": 7,
    "High":     30,
    "Medium":   90,
    "Low":      180,
    "Info":     0,
}

// ComputeExpirationDate calculates SLA deadline
func (c *SLAConfiguration) ComputeExpirationDate(severity string, findingDate time.Time) *time.Time {
    days := c.daysForSeverity(severity)
    if days == 0 { return nil }
    exp := findingDate.AddDate(0, 0, days)
    return &exp
}

func (c *SLAConfiguration) IsBreached(finding *entity.Finding) bool {
    if finding.SLAExpirationDate == nil { return false }
    if !finding.Active { return false }  // Only active findings can breach SLA
    return time.Now().After(*finding.SLAExpirationDate)
}
```

---

## 4. Use Cases

### 4.1 CreateFinding

```go
// finding-service/internal/usecase/finding/create.go

type CreateFindingInput struct {
    Title             string
    Description       string
    Mitigation        string
    Severity          entity.Severity
    CVE               string
    CWE               int
    CVSSv3Score       *float64
    VulnIDFromTool    string
    ComponentName     string
    ComponentVersion  string
    FilePath          string
    LineNumber        int
    TestID            uuid.UUID
    EngagementID      uuid.UUID
    ProductID         uuid.UUID
    Tags              []string
}

func (uc *CreateFindingUseCase) Execute(ctx context.Context, in CreateFindingInput) (*entity.Finding, error) {
    // 1. Create finding entity
    finding := entity.NewFinding(in.Title, in.Severity, in.TestID, in.EngagementID, in.ProductID)
    finding.CVE = in.CVE
    finding.ComponentName = in.ComponentName
    finding.ComponentVersion = in.ComponentVersion
    // ... copy other fields

    // 2. Compute hash for deduplication
    finding.computeHash()

    // 3. Check for existing finding with same hash
    existing, err := uc.findingRepo.FindByHash(ctx, finding.HashCode, finding.ProductID)
    if err == nil && existing != nil {
        // Duplicate found!
        finding.Duplicate = true
        finding.Active = false
        finding.DuplicateFindingID = &existing.ID
    }

    // 4. Compute SLA expiration
    sla, _ := uc.slaRepo.FindByProduct(ctx, in.ProductID)
    if sla == nil { sla = defaultSLAConfig() }
    finding.SLAExpirationDate = sla.ComputeExpirationDate(string(in.Severity), finding.Date)

    // 5. Persist
    if err := uc.findingRepo.Save(ctx, finding); err != nil { return nil, err }

    // 6. Audit
    uc.auditRepo.Log(ctx, &entity.AuditEvent{
        FindingID: finding.ID,
        Action:    "created",
        After:     finding,
    })

    // 7. Publish event
    uc.eventBus.Publish(ctx, "finding.created", &FindingCreatedEvent{
        FindingID: finding.ID,
        CVE:       finding.CVE,
        Severity:  string(finding.Severity),
        ProductID: finding.ProductID,
    })

    return finding, nil
}
```

### 4.2 Import Findings from Scan (NATS Consumer)

```go
// finding-service/internal/adapter/messaging/scan_completed_consumer.go
// Subscribe: scan.scan.completed → import findings from scan-service

func (c *ScanCompletedConsumer) Handle(ctx context.Context, msg *ScanCompletedEvent) error {
    // 1. Fetch findings from scan-service gRPC
    scanFindings, err := c.scanSvcClient.GetFindings(ctx, msg.ScanID)
    if err != nil { return err }

    // 2. Import each finding into finding-service
    for _, sf := range scanFindings {
        for _, cveID := range sf.CVEIDs {
            // Get CVE details from vulnerability-service
            cveDetail, _ := c.vulnSvcClient.GetCVE(ctx, cveID)

            in := CreateFindingInput{
                Title:            fmt.Sprintf("[%s] %s on %s", cveID, cveDetail.Summary, sf.IPAddress),
                CVE:              cveID,
                Severity:         mapSeverity(cveDetail.Severity),
                CVSSv3Score:      cveDetail.CVSSv3Score,
                ComponentName:    fmt.Sprintf("%s:%d", sf.IPAddress, 0),
                VulnIDFromTool:   "nmap-vulners",
                // Link to product/engagement/test context
                TestID:           msg.TestID,
                EngagementID:     msg.EngagementID,
                ProductID:        msg.ProductID,
            }

            c.createFindingUC.Execute(ctx, in)
        }
    }

    return nil
}
```

### 4.3 SLA Breach Check (Background Job)

```go
// finding-service/internal/usecase/sla/check_breaches.go
// Run by cron every 1 hour

func (uc *CheckSLABreachesUseCase) Execute(ctx context.Context) error {
    breached, err := uc.findingRepo.FindSLABreached(ctx)
    if err != nil { return err }

    for _, finding := range breached {
        uc.eventBus.Publish(ctx, "finding.sla.breached", &SLABreachedEvent{
            FindingID:    finding.ID,
            CVE:          finding.CVE,
            Severity:     string(finding.Severity),
            ProductID:    finding.ProductID,
            ExpiresAt:    *finding.SLAExpirationDate,
        })
    }

    return nil
}
```

---

## 5. Database Schema

```sql
-- finding-service migrations/

CREATE TABLE findings (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title              TEXT NOT NULL,
    description        TEXT,
    mitigation         TEXT,
    impact             TEXT,
    references         TEXT,
    severity           VARCHAR(10) NOT NULL CHECK (severity IN ('Critical','High','Medium','Low','Info')),
    numerical_severity INT NOT NULL,
    cve                VARCHAR(30),
    cwe                INT,
    vuln_id_from_tool  VARCHAR(100),
    cvss_v3            TEXT,
    cvss_v3_score      FLOAT,
    cvss_v4            TEXT,
    cvss_v4_score      FLOAT,

    -- State flags
    active             BOOLEAN NOT NULL DEFAULT TRUE,
    verified           BOOLEAN NOT NULL DEFAULT FALSE,
    false_positive     BOOLEAN NOT NULL DEFAULT FALSE,
    duplicate          BOOLEAN NOT NULL DEFAULT FALSE,
    out_of_scope       BOOLEAN NOT NULL DEFAULT FALSE,
    is_mitigated       BOOLEAN NOT NULL DEFAULT FALSE,
    risk_accepted      BOOLEAN NOT NULL DEFAULT FALSE,

    -- Timestamps
    date               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    mitigated_at       TIMESTAMPTZ,
    mitigated_by_id    UUID,
    last_reviewed      TIMESTAMPTZ,
    last_status_update TIMESTAMPTZ,
    sla_expiration_date TIMESTAMPTZ,

    -- Context
    test_id            UUID NOT NULL,
    engagement_id      UUID NOT NULL,
    product_id         UUID NOT NULL,
    duplicate_finding_id UUID REFERENCES findings(id),
    finding_group_id   UUID,

    -- Location
    component_name     VARCHAR(255),
    component_version  VARCHAR(100),
    service            VARCHAR(255),
    file_path          TEXT,
    line_number        INT,

    -- Dedup
    hash_code          VARCHAR(64),

    tags               TEXT[] DEFAULT '{}',
    inherited_tags     TEXT[] DEFAULT '{}',

    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_findings_product     ON findings(product_id);
CREATE INDEX idx_findings_engagement  ON findings(engagement_id);
CREATE INDEX idx_findings_severity    ON findings(severity);
CREATE INDEX idx_findings_active      ON findings(active);
CREATE INDEX idx_findings_hash        ON findings(hash_code);
CREATE INDEX idx_findings_cve         ON findings(cve);
CREATE INDEX idx_findings_sla         ON findings(sla_expiration_date) WHERE active = TRUE;

CREATE TABLE sla_configurations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID,     -- NULL = global default
    critical    INT NOT NULL DEFAULT 7,
    high        INT NOT NULL DEFAULT 30,
    medium      INT NOT NULL DEFAULT 90,
    low         INT NOT NULL DEFAULT 180,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(product_id)
);

CREATE TABLE finding_audit (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id  UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    user_id     UUID,
    action      VARCHAR(50) NOT NULL,  -- created|closed|reopened|false_positive|risk_accepted
    before_data JSONB,
    after_data  JSONB,
    comment     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_finding ON finding_audit(finding_id, created_at DESC);
```

---

## 6. API Routes

```
# Finding CRUD
GET    /api/v1/findings                     → List findings (filters: product, severity, status, cve)
POST   /api/v1/findings                     → Create finding manually
GET    /api/v1/findings/{id}                → Get finding detail
PUT    /api/v1/findings/{id}                → Update finding
DELETE /api/v1/findings/{id}                → Delete finding

# State transitions
POST   /api/v1/findings/{id}/close          → Close (mitigate) finding → 204
POST   /api/v1/findings/{id}/reopen         → Reopen finding → 204
POST   /api/v1/findings/{id}/false-positive → Mark as false positive → 204
POST   /api/v1/findings/{id}/risk-accept    → Accept risk → 204

# Audit
GET    /api/v1/findings/{id}/audit          → Get audit trail

# SLA
GET    /api/v1/sla                          → Get SLA configuration
PUT    /api/v1/sla                          → Update global SLA
GET    /api/v1/sla/breaches                 → List SLA-breached findings

# Stats
GET    /api/v1/findings/stats               → Severity breakdown, breach count
```

### Response Example

```json
// GET /api/v1/findings/{id}
{
  "id": "uuid",
  "title": "[CVE-2021-44228] Apache Log4j2 RCE on 192.168.1.10",
  "severity": "Critical",
  "numerical_severity": 4,
  "cve": "CVE-2021-44228",
  "cwe": 20,
  "cvss_v3_score": 10.0,
  "active": true,
  "verified": false,
  "false_positive": false,
  "duplicate": false,
  "current_state": "active",
  "date": "2026-06-14T07:00:00Z",
  "sla_expiration_date": "2026-06-21T07:00:00Z",
  "sla_breached": false,
  "days_until_sla": 7,
  "component_name": "192.168.1.10",
  "component_version": "",
  "product_id": "uuid",
  "engagement_id": "uuid",
  "test_id": "uuid",
  "hash_code": "sha256hex...",
  "tags": ["nmap", "production"],
  "created_at": "2026-06-14T07:00:00Z"
}
```

---

## 7. NATS Events

**Subscribed:**
```
scan.scan.completed   → Import findings from scan-service
impact.sbom.analyzed  → Update affected components
```

**Published:**
```
finding.created            → {finding_id, cve, severity, product_id}
finding.status.changed     → {finding_id, from_state, to_state, user_id}
finding.sla.breached       → {finding_id, cve, severity, expires_at}
```

---

## 8. Acceptance Criteria

- [ ] `POST /api/v1/findings` → creates finding với hash_code computed từ title+component+cve
- [ ] Duplicate finding với same hash_code → `duplicate=true`, `duplicate_finding_id` set
- [ ] `POST /api/v1/findings/{id}/close` → `is_mitigated=true`, `active=false`, `mitigated_at` set
- [ ] `POST /api/v1/findings/{id}/reopen` → `active=true`, tất cả flags reset
- [ ] `POST /api/v1/findings/{id}/false-positive` → `false_positive=true`, `active=false`
- [ ] Invalid transition (e.g., close a duplicate) → 400 error
- [ ] `SLAExpirationDate` = `Date + 7 ngày` cho Critical finding
- [ ] `GET /api/v1/sla/breaches` → danh sách active findings có SLA expired
- [ ] Audit log ghi đầy đủ: action, before_data, after_data, user_id, created_at
- [ ] `GET /api/v1/findings/{id}/audit` → trả về ordered audit trail
- [ ] NATS `scan.scan.completed` → findings được import tự động từ scan results
- [ ] `GET /api/v1/findings?severity=Critical&active=true` → filters hoạt động đúng
- [ ] `finding.sla.breached` NATS event được publish cho active findings với SLA expired
