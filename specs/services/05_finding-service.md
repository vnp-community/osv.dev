# finding-service

**Bounded Context**: Vulnerability Findings & Remediation
**Go Module**: `github.com/osv/finding-service`

---

## Merge từ

| Source | Trạng thái |
|--------|-----------|
| `services/finding-service` | ✅ Active — base chính |
| `services/product-service` | ✅ Active — merged |
| `services/report-service` | ✅ Active — merged |
| `archive/finding-management` | 📦 Archive — merged |
| `archive/sla` | 📦 Archive — merged |
| `archive/audit` | 📦 Archive — merged |
| `archive/product-management` | 📦 Archive — merged |
| `archive/report` | 📦 Archive — merged |

---

## Chức năng

| # | Chức năng | Mô tả |
|---|-----------|-------|
| 1 | **Finding Tracking** | Tạo và theo dõi vulnerability findings (CVE + Asset) |
| 2 | **Finding Lifecycle** | Quản lý trạng thái: New → Confirmed → In Progress → Resolved/Accepted |
| 3 | **Risk Scoring** | Tính risk score cho mỗi finding (CVSS × asset criticality) |
| 4 | **Product Management** | CRUD products, engagements, test sessions |
| 5 | **SLA Tracking** | Định nghĩa và theo dõi SLA policy theo severity |
| 6 | **SLA Breach Alert** | Phát sự kiện khi finding vi phạm SLA deadline |
| 7 | **Audit Trail** | Ghi nhật ký mọi thay đổi với actor, timestamp |
| 8 | **Deduplication** | Phát hiện và merge duplicate findings |
| 9 | **Report Generation** | Tạo báo cáo PDF, Excel, JSON cho findings |
| 10 | **Dashboard Data** | Cung cấp metrics cho vulnerability dashboard |

---

## Clean Architecture Layout

```
finding-service/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── domain/                         # ← Business rules
│   │   ├── finding/
│   │   │   ├── entity.go               # Finding aggregate root
│   │   │   ├── repository.go           # FindingRepository interface
│   │   │   ├── events.go               # FindingCreated, FindingResolved, SLABreached
│   │   │   └── service.go              # Deduplication, risk scoring
│   │   ├── product/
│   │   │   ├── entity.go               # Product aggregate root
│   │   │   ├── engagement.go           # Engagement entity
│   │   │   ├── test.go                 # Test session entity
│   │   │   └── repository.go
│   │   ├── sla/
│   │   │   ├── policy.go               # SLAPolicy entity
│   │   │   ├── tracker.go              # SLATracker domain service
│   │   │   └── repository.go
│   │   ├── audit/
│   │   │   ├── entry.go                # AuditEntry entity
│   │   │   └── repository.go
│   │   ├── report/
│   │   │   ├── entity.go               # Report entity
│   │   │   └── repository.go
│   │   └── errors/
│   │       └── errors.go
│   │
│   ├── usecase/                        # ← Application use cases
│   │   ├── create_finding/
│   │   │   ├── usecase.go
│   │   │   └── dto.go
│   │   ├── update_finding/
│   │   │   └── usecase.go              # Status change, comment, assign
│   │   ├── resolve_finding/
│   │   │   └── usecase.go              # Mark resolved/accepted/false_positive
│   │   ├── deduplicate/
│   │   │   └── usecase.go              # Merge duplicate findings
│   │   ├── track_sla/
│   │   │   └── usecase.go              # Check SLA deadlines (cron)
│   │   ├── audit_action/
│   │   │   └── usecase.go              # Record audit entry
│   │   ├── manage_product/
│   │   │   └── usecase.go              # CRUD product
│   │   ├── manage_engagement/
│   │   │   └── usecase.go              # CRUD engagement
│   │   ├── manage_test/
│   │   │   └── usecase.go              # CRUD test session
│   │   ├── generate_report/
│   │   │   ├── usecase.go
│   │   │   └── dto.go
│   │   └── dashboard_metrics/
│   │       └── usecase.go
│   │
│   ├── delivery/                       # ← Transport layer
│   │   ├── grpc/
│   │   │   ├── server.go
│   │   │   ├── finding_handler.go      # FindingService RPC impl
│   │   │   └── product_handler.go      # ProductService RPC impl
│   │   └── http/
│   │       ├── router.go
│   │       ├── finding_handler.go
│   │       ├── product_handler.go
│   │       ├── engagement_handler.go
│   │       ├── sla_handler.go
│   │       ├── audit_handler.go
│   │       └── report_handler.go
│   │
│   ├── infra/                          # ← External systems
│   │   ├── postgres/
│   │   │   ├── finding_repo.go
│   │   │   ├── product_repo.go
│   │   │   ├── sla_repo.go
│   │   │   └── audit_repo.go
│   │   ├── mongo/
│   │   │   └── report_store.go         # Store generated reports
│   │   └── nats/
│   │       ├── publisher.go            # Publish finding events
│   │       └── subscriber.go           # Subscribe scan.job.completed
│   │
│   └── formatters/                     # ← Report formatters
│       ├── pdf.go                      # wkhtmltopdf or chromedp
│       ├── excel.go                    # excelize
│       └── json.go
│
├── migrations/
│   ├── 001_create_products.sql
│   ├── 002_create_engagements.sql
│   ├── 003_create_findings.sql
│   ├── 004_create_sla_policies.sql
│   └── 005_create_audit_logs.sql
│
├── go.mod
└── Dockerfile
```

---

## Domain Model

### Finding Aggregate Root
```go
type Finding struct {
    ID              uuid.UUID
    CVEID           string              // CVE-2024-XXXXX
    AssetID         uuid.UUID
    ProductID       uuid.UUID
    EngagementID    *uuid.UUID
    TestID          *uuid.UUID
    Status          FindingStatus
    Severity        SeverityLevel       // Inherited from CVE, can be overridden
    SeverityOverride *SeverityLevel     // Manual override
    RiskScore       float64             // Computed: CVSS × asset_criticality
    Title           string
    Description     string
    Component       string              // Affected component
    Version         string              // Affected version
    FoundAt         time.Time
    SLADeadline     *time.Time          // Computed from SLA policy
    ResolvedAt      *time.Time
    AcceptedRisk    *RiskAcceptance
    Comments        []Comment
    Tags            []string
    AssignedTo      *uuid.UUID          // User
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type FindingStatus string
const (
    FindingNew          FindingStatus = "new"
    FindingConfirmed    FindingStatus = "confirmed"
    FindingInProgress   FindingStatus = "in_progress"
    FindingResolved     FindingStatus = "resolved"
    FindingAccepted     FindingStatus = "accepted"         // Risk accepted
    FindingFalsePos     FindingStatus = "false_positive"
    FindingDuplicate    FindingStatus = "duplicate"
)
```

### Product & Engagement
```go
type Product struct {
    ID           uuid.UUID
    Name         string
    Description  string
    Type         ProductType      // WEB | MOBILE | API | INFRASTRUCTURE
    Criticality  Criticality      // CRITICAL | HIGH | MEDIUM | LOW
    TeamID       uuid.UUID
    Tags         []string
    Engagements  []Engagement
}

type Engagement struct {
    ID          uuid.UUID
    ProductID   uuid.UUID
    Name        string
    StartDate   time.Time
    EndDate     *time.Time
    Status      EngagementStatus // ACTIVE | COMPLETED | ARCHIVED
    Tests       []Test
}

type Test struct {
    ID           uuid.UUID
    EngagementID uuid.UUID
    ScanJobID    *uuid.UUID       // Link to scan-service
    Title        string
    Type         TestType         // DEPENDENCY | SBOM | IMAGE | CODE
    ExecutedAt   time.Time
    FindingCount int
}
```

### SLA Policy
```go
type SLAPolicy struct {
    ID              uuid.UUID
    Name            string
    ProductID       *uuid.UUID       // nil = global default
    Critical        int              // Days to remediate CRITICAL
    High            int              // Days to remediate HIGH
    Medium          int              // Days to remediate MEDIUM
    Low             int              // Days to remediate LOW
    IsActive        bool
}
```

---

## API Specification

### HTTP REST Endpoints

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET`  | `/findings` | JWT | Danh sách findings (filter, paginate) |
| `POST` | `/findings` | JWT | Tạo finding thủ công |
| `GET`  | `/findings/{id}` | JWT | Chi tiết finding |
| `PUT`  | `/findings/{id}` | JWT | Cập nhật finding |
| `POST` | `/findings/{id}/resolve` | JWT | Đánh dấu resolved |
| `POST` | `/findings/{id}/accept` | JWT | Accept risk |
| `POST` | `/findings/{id}/false-positive` | JWT | Mark false positive |
| `POST` | `/findings/{id}/comments` | JWT | Thêm comment |
| `GET`  | `/findings/{id}/audit` | JWT | Audit trail |
| `GET`  | `/products` | JWT | Danh sách products |
| `POST` | `/products` | JWT | Tạo product |
| `GET`  | `/products/{id}` | JWT | Chi tiết product |
| `CRUD` | `/products/{id}/engagements` | JWT | Engagement management |
| `CRUD` | `/engagements/{id}/tests` | JWT | Test management |
| `GET`  | `/sla/policies` | JWT | SLA policies |
| `POST` | `/sla/policies` | Admin | Tạo SLA policy |
| `GET`  | `/sla/status` | JWT | SLA compliance dashboard |
| `POST` | `/reports/generate` | JWT | Generate report |
| `GET`  | `/reports/{id}` | JWT | Download report |
| `GET`  | `/dashboard/metrics` | JWT | Dashboard data |

### gRPC Services (internal)

```protobuf
service FindingService {
    rpc CreateFinding(CreateFindingRequest) returns (FindingResponse);
    rpc GetFinding(GetFindingRequest) returns (FindingResponse);
    rpc ListFindings(ListFindingsRequest) returns (ListFindingsResponse);
    rpc UpdateFindingStatus(UpdateFindingStatusRequest) returns (FindingResponse);
    rpc GetDashboardMetrics(MetricsRequest) returns (MetricsResponse);
}

service ProductService {
    rpc GetProduct(GetProductRequest) returns (ProductResponse);
    rpc ListProducts(ListProductsRequest) returns (ListProductsResponse);
}

service ReportService {
    rpc GenerateReport(GenerateReportRequest) returns (ReportResponse);
    rpc GetReport(GetReportRequest) returns (stream ReportChunk);
}
```

---

## Event Subscriptions (NATS)

| Subject | Source | Action |
|---------|--------|--------|
| `scan.job.completed` | scan-service | Auto-create findings từ scan results |
| `data.cve.updated` | data-service | Update finding severity nếu CVE thay đổi |

## Event Publishing (NATS)

| Event | Subject | Consumer |
|-------|---------|----------|
| `FindingCreated` | `finding.created` | notification-service |
| `FindingStatusChanged` | `finding.status_changed` | notification-service |
| `SLABreached` | `finding.sla_breached` | notification-service |
| `SLADueSoon` | `finding.sla_due_soon` | notification-service |

---

## Dependencies

```
github.com/jackc/pgx/v5        # PostgreSQL
go.mongodb.org/mongo-driver    # MongoDB (report storage)
github.com/nats-io/nats.go     # NATS messaging
github.com/go-chi/chi/v5       # HTTP router
google.golang.org/grpc         # gRPC
github.com/robfig/cron/v3      # SLA check cron
github.com/xuri/excelize/v2    # Excel report generation
github.com/osv/shared/pkg
github.com/osv/shared/proto
```

---

## Database Schema (PostgreSQL)

```sql
-- Products
CREATE TABLE products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    type        VARCHAR(30),
    criticality VARCHAR(20),
    team_id     UUID,
    tags        TEXT[],
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Engagements
CREATE TABLE engagements (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID REFERENCES products(id),
    name       VARCHAR(255),
    start_date DATE,
    end_date   DATE,
    status     VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Findings
CREATE TABLE findings (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cve_id            VARCHAR(30),
    asset_id          UUID,
    product_id        UUID REFERENCES products(id),
    engagement_id     UUID REFERENCES engagements(id),
    status            VARCHAR(30) DEFAULT 'new',
    severity          VARCHAR(20),
    severity_override VARCHAR(20),
    risk_score        NUMERIC(5,2),
    title             TEXT,
    component         TEXT,
    version           VARCHAR(100),
    found_at          TIMESTAMPTZ NOT NULL,
    sla_deadline      TIMESTAMPTZ,
    resolved_at       TIMESTAMPTZ,
    assigned_to       UUID,
    tags              TEXT[],
    created_at        TIMESTAMPTZ DEFAULT NOW(),
    updated_at        TIMESTAMPTZ DEFAULT NOW()
);

-- SLA Policies
CREATE TABLE sla_policies (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(255),
    product_id UUID REFERENCES products(id),
    critical   INT DEFAULT 7,
    high       INT DEFAULT 30,
    medium     INT DEFAULT 90,
    low        INT DEFAULT 180,
    is_active  BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Audit Logs
CREATE TABLE audit_logs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity     VARCHAR(50),    -- finding | product | sla_policy
    entity_id  UUID,
    action     VARCHAR(50),    -- created | status_changed | commented | etc.
    actor_id   UUID,
    changes    JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```
