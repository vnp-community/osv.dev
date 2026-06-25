# SOL-OVS-002 — Giải Pháp: Finding Service (Lifecycle, SLA, Dedup)

| Trường | Giá trị |
|--------|---------|
| **Solution ID** | SOL-OVS-002 |
| **CR tham chiếu** | CR-OVS-002 |
| **Tiêu đề** | Finding Service — Vulnerability Finding Lifecycle, SLA Policy, Hash-based Deduplication, Audit Trail |
| **Ngày tạo** | 2026-06-16 |
| **Ngày implement** | 2026-06-17 |
| **Trạng thái** | ✅ Implemented |

---

## 0. Implementation Status

> **Trạng thái**: ✅ **IMPLEMENTED** — 2026-06-17

| Task | File | Trạng thái |
|------|------|------------|
| T-FIND-001 | `finding-service/internal/domain/finding/entity.go` | ✅ Done |
| T-FIND-002 | `finding-service/internal/usecase/dedup/service.go` | ✅ Done |
| T-FIND-003 | `finding-service/internal/domain/sla/service.go` | ✅ Done |
| T-FIND-004 | `finding-service/internal/domain/audit/logger.go` | ✅ Done |
| T-FIND-005 | `finding-service/internal/delivery/grpc/server.go` | ✅ Done |
| T-FIND-006 | `finding-service/internal/delivery/nats/consumer.go` | ✅ Done |

**Chi tiết implementation**:
- **State Machine**: `Close/Reopen/MarkFalsePositive/MarkRiskAccepted/MarkOutOfScope` transitions, Duplicate là terminal state
- **SHA-256 Dedup**: key = `SHA256(title|component|version|cve)`, batch import với dedup check
- **SLA**: Critical=7d, High=30d, Medium=90d, Low=180d, background cron 1h check breach
- **Audit Trail**: Append-only log structured `{action, before_data, after_data, user_id, comment}`
- **gRPC Server**: `GetFinding/UpdateFindingStatus/ListFindings` với logging interceptor
- **NATS Consumer**: JetStream consumer `scan.scan.completed`, batch import findings với dedup

---

## 1. Tổng Quan Giải Pháp

### 1.1 Bối Cảnh

OSV.dev chỉ lưu trữ CVE database. Khi scan phát hiện một lỗ hổng trên một host cụ thể, **Finding Service** là nơi quản lý vòng đời của phát hiện đó — từ khi phát hiện đến khi được vá, bị đánh dấu false positive, hoặc rủi ro được chấp nhận.

### 1.2 Vai Trò Trung Tâm

`finding-service` là **hub trung tâm** của OpenVulnScan:
- Nhận kết quả từ `scan-service` (qua NATS)
- Cung cấp context cho `ai-service` (triage)
- Cung cấp data cho `report-service` (xuất báo cáo)
- Gắn với `product-service` hierarchy (Product/Engagement/Test)

---

## 2. Kiến Trúc

### 2.1 Vị Trí Trong Hệ Thống

```
scan-service (NATS: scan.scan.completed)
        │
        ▼
finding-service ──── ScanCompletedConsumer
        │              (import findings)
        │
        ├── finding-service gRPC ──▶ report-service
        │
        ├── finding.sla.breached NATS ──▶ notification-service  
        │
        └── finding.created NATS ──▶ ai-service (triage)

product-service (provides TestID/EngagementID/ProductID context)
```

### 2.2 Cấu Trúc Thư Mục

```
services/finding-service/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── finding/
│   │   │   ├── entity.go              # Finding entity
│   │   │   ├── state_machine.go       # State transitions
│   │   │   └── repository.go          # FindingRepository interface
│   │   └── sla/
│   │       ├── entity.go              # SLAConfiguration
│   │       └── repository.go
│   ├── usecase/
│   │   ├── finding/
│   │   │   ├── create.go
│   │   │   ├── update.go
│   │   │   ├── close.go
│   │   │   ├── reopen.go
│   │   │   ├── false_positive.go
│   │   │   ├── risk_accept.go
│   │   │   └── list.go
│   │   └── sla/
│   │       └── check_breaches.go      # Background cron job
│   ├── adapter/
│   │   ├── repository/postgres/
│   │   │   ├── finding_repo.go
│   │   │   ├── sla_repo.go
│   │   │   └── audit_repo.go
│   │   └── messaging/nats/
│   │       ├── scan_completed_consumer.go
│   │       └── publisher.go
│   └── delivery/
│       ├── http/
│       │   ├── finding_handler.go
│       │   └── sla_handler.go
│       └── grpc/
│           └── finding_server.go
├── migrations/
│   └── 001_create_finding_tables.sql
└── config/config.yaml
```

---

## 3. Domain Model Chi Tiết

### 3.1 Finding State Machine

```
                  ┌─────────────────────────────────────────┐
                  │             active                      │
                  │  (active=true, all flags=false)         │
                  └──┬────────┬──────────┬──────────────────┘
                     │        │          │         │
                  Close()  MarkFP()  AcceptRisk() MarkOutOfScope()
                     │        │          │         │
              ┌──────▼──┐ ┌───▼──┐ ┌───▼──┐ ┌───▼──────┐
              │mitigated│ │false │ │risk  │ │out_of_   │
              │         │ │posit.│ │accept│ │scope     │
              └──────┬──┘ └──────┘ └──────┘ └──────────┘
                     │
                  Reopen()
                     │
              ┌──────▼──┐
              │  active  │ (reset tất cả flags)
              └──────────┘

Duplicate: Trạng thái đặc biệt, set khi CreateFinding phát hiện
           cùng hash_code tồn tại. Không thể Reopen.

Priority trong CurrentState():
Duplicate > FalsePositive > OutOfScope > RiskAccepted > Mitigated > Active
```

### 3.2 Hash-based Deduplication

```go
// Dedup key = SHA-256(title | component_name | component_version | cve)
// Ví dụ:
// title          = "[CVE-2021-44228] Apache Log4j2 RCE"
// component_name = "192.168.1.10"
// component_version = ""
// cve            = "CVE-2021-44228"
// → hash_code    = SHA256("...|...|...|...")

// Khi có duplicate:
// - Finding mới: duplicate=true, active=false, duplicate_finding_id = <original_id>
// - Finding gốc: không thay đổi
// - Audit log ghi: "duplicate_detected", duplicate_of: <original_id>
```

### 3.3 SLA Policy

```
Severity   | Default Days | Business Criticality Override (very high)
-----------|-------------|------------------------------------------
Critical   | 7 days      | 3 days
High       | 30 days     | 14 days
Medium     | 90 days     | 45 days
Low        | 180 days    | 90 days
Info       | no deadline | no deadline
```

**SLA Expiration Date** = `finding.Date + SLADays`

**SLA Breach** = `finding.Active == true AND time.Now() > SLAExpirationDate`

---

## 4. Use Cases Chi Tiết

### 4.1 CreateFinding — Full Flow

```
CreateFindingInput
      │
      ├─ 1. Validate: title, severity, testID, engagementID, productID required
      │
      ├─ 2. Create Finding entity
      │      - NumericalSeverity = severity.Numerical()
      │      - Active = true, date = now
      │      - computeHash(): SHA256(title|component|version|cve)
      │
      ├─ 3. Deduplication check:
      │      findingRepo.FindByHash(hash_code, product_id)
      │      if found:
      │        finding.Duplicate = true
      │        finding.Active = false  
      │        finding.DuplicateFindingID = &existing.ID
      │
      ├─ 4. SLA computation:
      │      sla = slaRepo.FindByProduct(product_id) OR defaultSLA
      │      finding.SLAExpirationDate = sla.ComputeExpirationDate(severity, date)
      │
      ├─ 5. Persist to DB
      │
      ├─ 6. Audit log: {action="created", after=finding, user_id}
      │
      └─ 7. Publish NATS: finding.created
              {finding_id, cve, severity, product_id}
```

### 4.2 Import từ scan-service (NATS Consumer)

```go
// Nhận scan.scan.completed
// → Fetch scan findings từ scan-service gRPC
// → Create findings trong finding-service

func (c *ScanCompletedConsumer) Handle(ctx context.Context, msg *ScanCompletedEvent) error {
    // 1. Fetch raw findings from scan-service
    rawFindings, err := c.scanSvcClient.GetFindings(ctx, msg.ScanID)
    if err != nil { return err }
    
    // 2. Enrich với CVE data từ vulnerability-service
    for _, rf := range rawFindings {
        for _, cveID := range rf.CVEIDs {
            cveDetail, _ := c.vulnSvcClient.GetCVE(ctx, cveID)
            
            // 3. Map to finding (product/engagement/test context từ msg)
            c.createFindingUC.Execute(ctx, CreateFindingInput{
                Title:            fmt.Sprintf("[%s] %s on %s", cveID, cveDetail.Summary, rf.IPAddress),
                Description:      cveDetail.Details,
                CVE:              cveID,
                Severity:         mapCVSSToCategorical(cveDetail.CVSSScore),
                CVSSv3Score:      &cveDetail.CVSSScore,
                ComponentName:    rf.IPAddress,
                VulnIDFromTool:   "nmap-vulners",
                TestID:           msg.TestID,
                EngagementID:     msg.EngagementID,
                ProductID:        msg.ProductID,
            })
        }
    }
    return nil
}
```

### 4.3 SLA Breach Check — Cron Job

```go
// Chạy mỗi 1 giờ qua background goroutine hoặc cron
// SELECT * FROM findings 
//   WHERE active = TRUE 
//     AND sla_expiration_date < NOW() 
//     AND NOT duplicate AND NOT false_positive

func (uc *CheckSLABreachesUseCase) Execute(ctx context.Context) error {
    breached, err := uc.findingRepo.FindSLABreached(ctx)
    if err != nil { return err }
    
    for _, finding := range breached {
        // Publish NATS event để notification-service gửi alert
        uc.eventBus.Publish(ctx, "finding.sla.breached", &SLABreachedEvent{
            FindingID:  finding.ID,
            CVE:        finding.CVE,
            Severity:   string(finding.Severity),
            ProductID:  finding.ProductID,
            ExpiresAt:  *finding.SLAExpirationDate,
            DaysOverdue: int(time.Since(*finding.SLAExpirationDate).Hours() / 24),
        })
    }
    
    return nil
}
```

---

## 5. Repository Interface

```go
// finding-service/internal/domain/finding/repository.go

type FindingRepository interface {
    // CRUD
    Save(ctx context.Context, f *Finding) error
    FindByID(ctx context.Context, id uuid.UUID) (*Finding, error)
    Update(ctx context.Context, f *Finding) error
    Delete(ctx context.Context, id uuid.UUID) error
    
    // List với filters
    List(ctx context.Context, filter FindingFilter) ([]*Finding, int64, error)
    
    // Deduplication
    FindByHash(ctx context.Context, hashCode string, productID uuid.UUID) (*Finding, error)
    
    // SLA
    FindSLABreached(ctx context.Context) ([]*Finding, error)
    
    // By context
    FindByProduct(ctx context.Context, productID uuid.UUID, filter FindingFilter) ([]*Finding, int64, error)
    FindByTest(ctx context.Context, testID uuid.UUID) ([]*Finding, error)
    FindByCVE(ctx context.Context, cve string) ([]*Finding, error)
    
    // Stats
    CountBySeverity(ctx context.Context, productID uuid.UUID) (map[string]int64, error)
}

type FindingFilter struct {
    ProductID   *uuid.UUID
    Severity    *Severity
    Active      *bool
    CVE         string
    Tags        []string
    DateFrom    *time.Time
    DateTo      *time.Time
    Page        int
    Limit       int
    SortBy      string  // "date"|"severity"|"sla_expiration"
    SortOrder   string  // "asc"|"desc"
}
```

---

## 6. API Response Design

### 6.1 Finding Detail Response

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "title": "[CVE-2021-44228] Apache Log4j2 RCE on 192.168.1.10",
  "description": "A vulnerability in Log4j...",
  "mitigation": "Update to Log4j 2.15.0 or later...",
  "severity": "Critical",
  "numerical_severity": 4,
  "cve": "CVE-2021-44228",
  "cwe": 502,
  "vuln_id_from_tool": "nmap-vulners",
  "cvss_v3": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H",
  "cvss_v3_score": 10.0,
  
  "active": true,
  "verified": false,
  "false_positive": false,
  "duplicate": false,
  "out_of_scope": false,
  "is_mitigated": false,
  "risk_accepted": false,
  "current_state": "active",
  
  "date": "2026-06-16T00:00:00Z",
  "sla_expiration_date": "2026-06-23T00:00:00Z",
  "sla_breached": false,
  "days_until_sla": 7,
  
  "component_name": "192.168.1.10",
  "component_version": "",
  "service": "api-gateway",
  
  "test_id": "uuid",
  "engagement_id": "uuid",
  "product_id": "uuid",
  "duplicate_finding_id": null,
  
  "hash_code": "a1b2c3...",
  "tags": ["nmap", "production"],
  "inherited_tags": ["infrastructure"],
  
  "created_at": "2026-06-16T00:00:00Z",
  "updated_at": "2026-06-16T00:00:00Z"
}
```

### 6.2 Stats Response

```json
// GET /api/v1/findings/stats?product_id=uuid
{
  "product_id": "uuid",
  "total": 42,
  "active": 15,
  "mitigated": 20,
  "false_positive": 5,
  "risk_accepted": 2,
  "by_severity": {
    "Critical": 3,
    "High": 8,
    "Medium": 20,
    "Low": 11
  },
  "sla_breached": 2,
  "sla_breaching_soon": 3  // expiring within 7 days
}
```

---

## 7. Database Schema Chi Tiết

```sql
-- migrations/001_create_finding_tables.sql

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE findings (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title               TEXT NOT NULL,
    description         TEXT,
    mitigation          TEXT,
    impact              TEXT,
    "references"        TEXT,
    severity            VARCHAR(10) NOT NULL 
                        CHECK (severity IN ('Critical','High','Medium','Low','Info')),
    numerical_severity  INT NOT NULL CHECK (numerical_severity BETWEEN 0 AND 4),
    cve                 VARCHAR(30),
    cwe                 INT,
    vuln_id_from_tool   VARCHAR(100),
    cvss_v3             TEXT,
    cvss_v3_score       FLOAT CHECK (cvss_v3_score BETWEEN 0.0 AND 10.0),
    cvss_v4             TEXT,
    cvss_v4_score       FLOAT CHECK (cvss_v4_score BETWEEN 0.0 AND 10.0),
    
    -- State flags
    active              BOOLEAN NOT NULL DEFAULT TRUE,
    verified            BOOLEAN NOT NULL DEFAULT FALSE,
    false_positive      BOOLEAN NOT NULL DEFAULT FALSE,
    duplicate           BOOLEAN NOT NULL DEFAULT FALSE,
    out_of_scope        BOOLEAN NOT NULL DEFAULT FALSE,
    is_mitigated        BOOLEAN NOT NULL DEFAULT FALSE,
    risk_accepted       BOOLEAN NOT NULL DEFAULT FALSE,
    
    -- Timestamps
    date                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    mitigated_at        TIMESTAMPTZ,
    mitigated_by_id     UUID,
    last_reviewed       TIMESTAMPTZ,
    last_status_update  TIMESTAMPTZ,
    sla_expiration_date TIMESTAMPTZ,
    
    -- Context hierarchy
    test_id             UUID NOT NULL,
    engagement_id       UUID NOT NULL,
    product_id          UUID NOT NULL,
    duplicate_finding_id UUID REFERENCES findings(id) ON DELETE SET NULL,
    finding_group_id    UUID,
    
    -- Location
    component_name      VARCHAR(255),
    component_version   VARCHAR(100),
    service             VARCHAR(255),
    file_path           TEXT,
    line_number         INT,
    
    -- Dedup
    hash_code           VARCHAR(64),
    
    -- Tags
    tags                TEXT[] DEFAULT '{}',
    inherited_tags      TEXT[] DEFAULT '{}',
    
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_findings_product       ON findings(product_id);
CREATE INDEX idx_findings_engagement    ON findings(engagement_id);
CREATE INDEX idx_findings_test          ON findings(test_id);
CREATE INDEX idx_findings_severity      ON findings(severity);
CREATE INDEX idx_findings_active        ON findings(active) WHERE active = TRUE;
CREATE INDEX idx_findings_hash          ON findings(hash_code, product_id);
CREATE INDEX idx_findings_cve           ON findings(cve) WHERE cve IS NOT NULL;
CREATE INDEX idx_findings_sla           ON findings(sla_expiration_date) WHERE active = TRUE;
CREATE INDEX idx_findings_date          ON findings(date DESC);
CREATE INDEX idx_findings_tags          ON findings USING GIN(tags);

-- SLA configurations
CREATE TABLE sla_configurations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID UNIQUE,  -- NULL = global default
    critical    INT NOT NULL DEFAULT 7,
    high        INT NOT NULL DEFAULT 30,
    medium      INT NOT NULL DEFAULT 90,
    low         INT NOT NULL DEFAULT 180,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert global default SLA
INSERT INTO sla_configurations (product_id, critical, high, medium, low)
VALUES (NULL, 7, 30, 90, 180);

-- Audit trail
CREATE TABLE finding_audit (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id  UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    user_id     UUID,
    action      VARCHAR(50) NOT NULL,
                -- created|closed|reopened|false_positive|risk_accepted
                -- out_of_scope|duplicate_detected|tag_updated|severity_changed
    before_data JSONB,
    after_data  JSONB,
    comment     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_finding    ON finding_audit(finding_id, created_at DESC);
CREATE INDEX idx_audit_user       ON finding_audit(user_id, created_at DESC);
CREATE INDEX idx_audit_action     ON finding_audit(action);
```

---

## 8. gRPC Service Definition

```protobuf
// finding-service/proto/finding/v1/finding.proto
syntax = "proto3";

package finding.v1;

service FindingService {
  // Called by report-service, product-service
  rpc GetFindingsByProduct(GetByProductRequest) returns (FindingList);
  rpc GetFindingsByTest(GetByTestRequest) returns (FindingList);
  rpc GetFindingsByScan(GetByScanRequest) returns (FindingList);
  rpc GetFindingStats(GetStatsRequest) returns (FindingStats);
  
  // Called by product-service orchestrator
  rpc CreateFinding(CreateFindingRequest) returns (FindingResponse);
  rpc BulkCreateFindings(BulkCreateRequest) returns (BulkCreateResponse);
}

message FindingResponse {
  string id = 1;
  bool duplicate = 2;
  string duplicate_finding_id = 3;
  string hash_code = 4;
}

message BulkCreateResponse {
  int32 created = 1;
  int32 duplicates = 2;
  repeated string finding_ids = 3;
}

message FindingStats {
  int32 total = 1;
  int32 active = 2;
  int32 mitigated = 3;
  map<string, int32> by_severity = 4;
  int32 sla_breached = 5;
}
```

---

## 9. Configuration

```yaml
# config/config.yaml
server:
  grpc_port: 50060
  http_port: 8060

database:
  host: "${DB_HOST}"
  port: 5432
  name: "finding_service"
  user: "${DB_USER}"
  password: "${DB_PASSWORD}"
  max_open_conns: 25

nats:
  url: "${NATS_URL}"
  stream: "FINDING_EVENTS"
  subjects:
    subscribe:
      - "scan.scan.completed"
      - "impact.sbom.analyzed"

sla:
  breach_check_interval: "1h"   # Cron: every 1 hour
  
clients:
  scan_service:
    grpc_address: "scan-service:50058"
  vulnerability_service:
    grpc_address: "vulnerability-service:50055"

auth:
  jwt_public_key_path: "/secrets/jwt_public.pem"

logging:
  level: "info"
  format: "json"
```

---

## 10. Background Jobs

### 10.1 SLA Breach Check Scheduler

```go
// finding-service/internal/scheduler/sla_check.go

type SLACheckScheduler struct {
    checkSLAUC  *CheckSLABreachesUseCase
    interval    time.Duration
    logger      zerolog.Logger
}

func (s *SLACheckScheduler) Start(ctx context.Context) {
    ticker := time.NewTicker(s.interval)  // 1 hour
    go func() {
        // Run immediately on start
        s.checkSLAUC.Execute(ctx)
        
        for {
            select {
            case <-ticker.C:
                if err := s.checkSLAUC.Execute(ctx); err != nil {
                    s.logger.Error().Err(err).Msg("SLA breach check failed")
                }
            case <-ctx.Done():
                ticker.Stop()
                return
            }
        }
    }()
}
```

---

## 11. Inter-Service Dependencies

```
finding-service
├── DEPENDS ON:
│   ├── auth-service (gRPC: ValidateToken)                [CR-OVS-003]
│   ├── scan-service (gRPC: GetFindings, GetFindingsByScan)  [CR-OVS-001]
│   ├── vulnerability-service (gRPC: GetCVE)              [OSV core]
│   └── product-service (provides Product/Engagement/Test context)  [CR-OVS-004]
│
├── PUBLISHES TO (NATS):
│   ├── finding.created → ai-service (triage)             [CR-OVS-005]
│   ├── finding.status.changed → notification-service
│   └── finding.sla.breached → notification-service
│
└── CONSUMED BY (gRPC):
    ├── report-service (GetFindingsByProduct/Scan)         [CR-OVS-006]
    └── product-service (CreateFinding, GetStats)          [CR-OVS-004]
```

---

## 12. Implementation Roadmap

### Phase 1 — Core Domain (Sprint 1)
- [ ] Finding entity + state machine
- [ ] Database migrations
- [ ] FindingRepository (PostgreSQL)
- [ ] CreateFinding use case (with deduplication)
- [ ] SLAConfiguration entity + repo

### Phase 2 — State Transitions (Sprint 2)
- [ ] Close/Reopen/FalsePositive/RiskAccepted use cases
- [ ] AuditRepository + audit logging
- [ ] HTTP REST handlers (CRUD + transitions)
- [ ] NATS publisher

### Phase 3 — Integration (Sprint 3)
- [ ] ScanCompletedConsumer (NATS → import findings)
- [ ] SLA breach check cron job
- [ ] gRPC server (FindingService)
- [ ] scan-service gRPC client integration

### Phase 4 — Polish (Sprint 4)
- [ ] Finding stats endpoint
- [ ] Pagination + filtering
- [ ] Integration tests
- [ ] Prometheus metrics

---

## 13. Acceptance Criteria Mapping

| Criterion | Implementation |
|-----------|---------------|
| `POST /api/v1/findings` → hash computed | `NewFinding()` calls `computeHash()` |
| Same hash → duplicate=true, duplicate_finding_id set | `CreateFindingUseCase` → `FindByHash()` |
| `POST /{id}/close` → is_mitigated=true, active=false | `Finding.Close()` state machine |
| `POST /{id}/reopen` → active=true, flags reset | `Finding.Reopen()` |
| `POST /{id}/false-positive` → false_positive=true | `Finding.MarkFalsePositive()` |
| Invalid transition on duplicate → 400 | `ErrCannotReopenDuplicate` |
| SLAExpirationDate = Date + 7d for Critical | `SLAConfiguration.ComputeExpirationDate()` |
| `GET /api/v1/sla/breaches` → active findings with SLA expired | `FindingRepo.FindSLABreached()` |
| Audit log: action, before_data, after_data, user_id | `AuditRepository.Log()` |
| `GET /{id}/audit` → ordered trail | `idx_audit_finding (finding_id, created_at DESC)` |
| NATS scan.scan.completed → findings imported | `ScanCompletedConsumer.Handle()` |
| `GET /api/v1/findings?severity=Critical&active=true` → filters | `FindingFilter` struct |
| `finding.sla.breached` NATS event published | `CheckSLABreachesUseCase.Execute()` |
