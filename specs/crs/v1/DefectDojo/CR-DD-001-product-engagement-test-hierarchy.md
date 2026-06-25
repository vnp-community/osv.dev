# ✅ COMPLETED — CR-DD-001 — Product/Engagement/Test Hierarchy Service

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DD-001 |
| **Tiêu đề** | Product Management — Product/Engagement/Test Hierarchy & Members |
| **Nguồn tham chiếu** | `django-DefectDojo/specs/services/03-product-management-service.md`, `SRS.md §3.1 FR-DM-01, FR-DM-04` |
| **Target Service** | **MỚI**: `product-service` (`github.com/osv/product-service`) |
| **Ưu tiên** | 🔴 High |
| **Loại** | New Service |
| **Ngày tạo** | 2026-06-13 |

---

## 1. Tổng quan

OSV không có khái niệm **Product → Engagement → Test** hierarchy — cốt lõi của DefectDojo để tổ chức security scans theo sản phẩm, sprint/engagement, và test execution. Đây là nền tảng cho toàn bộ DefectDojo workflow.

### Business Case

```
Công ty có 3 sản phẩm:
  ProductType: "Web Applications"
    Product: "Portal" (SLA: Critical=7d, High=30d)
      Engagement: "Q2 2026 Security Review" (Interactive)
        Test: "OWASP ZAP DAST Scan" → Findings
        Test: "Bandit SAST Scan"     → Findings
      Engagement: "CI/CD Pipeline" (CI/CD)
        Test: "Trivy Container Scan" [auto, per commit]
    Product: "Mobile App"
      ...
```

---

## 2. Gap Analysis

### OSV hiện tại

| Feature | OSV Status |
|---------|-----------|
| Product hierarchy | ❌ Không có |
| Engagement model | ❌ Không có |
| Test (scan run) tracking | ❌ Không có |
| Product members + RBAC | ❌ Không có |
| Product-level SLA assignment | ❌ Không có |
| Risk Acceptance | ❌ Không có (CR-DD-005) |
| Tool Configuration | ❌ Không có |

### DefectDojo Python → Go translation

```python
# Django models (dojo/models.py) → Go domain entities
Product_Type → ProductType struct
Product      → Product struct (+ SLAConfiguration FK)
Engagement   → Engagement struct (Interactive | CI/CD)
Test         → Test struct (linked to Test_Type/ScanType)
```

---

## 3. Service Structure

```
product-service/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── producttype/
│   │   │   ├── entity.go         # ProductType
│   │   │   └── repository.go
│   │   ├── product/
│   │   │   ├── entity.go         # Product + business rules
│   │   │   ├── value_objects.go  # Platform, Lifecycle, Origin, Criticality
│   │   │   └── repository.go
│   │   ├── engagement/
│   │   │   ├── entity.go         # Engagement + state machine
│   │   │   └── repository.go
│   │   ├── test/
│   │   │   ├── entity.go         # Test + TestType
│   │   │   └── repository.go
│   │   ├── member/
│   │   │   ├── entity.go         # ProductMember, ProductTypeMember
│   │   │   └── repository.go
│   │   └── tool/
│   │       ├── entity.go         # ToolConfiguration (scanner credentials)
│   │       └── repository.go
│   ├── usecase/
│   │   ├── producttype/          # CRUD
│   │   ├── product/              # CRUD + grade computation
│   │   ├── engagement/           # CRUD + auto-close
│   │   ├── test/                 # CRUD
│   │   ├── member/               # Add/Remove/Update members
│   │   └── tool/                 # CRUD tool configurations
│   ├── delivery/
│   │   ├── http/                 # REST handlers
│   │   └── grpc/                 # gRPC server (consumed by Scan/Finding services)
│   └── infra/
│       ├── postgres/             # Repositories
│       └── scheduler/            # Auto-close engagements cron
├── proto/product/v1/product.proto
└── migrations/
```

---

## 4. Domain Model

### 4.1 Entities

```go
// domain/producttype/entity.go
type ProductType struct {
    ID              string
    Name            string   // unique
    Description     string
    CriticalProduct bool
    KeyProduct      bool
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// domain/product/entity.go
// Mirrors Python: dojo/models.py::Product
type Product struct {
    ID                  string
    Name                string    // unique (globally)
    Description         string
    ProductTypeID       string

    // Security classification
    BusinessCriticality Criticality // very high|high|medium|low|very low|none
    Platform            Platform    // web|mobile|desktop|api|iot
    Lifecycle           Lifecycle   // construction|production|retirement
    Origin              Origin      // internal|contractor|outsourced|open source|purchased

    // Configuration
    SLAConfigurationID  *string
    EnableSimpleRiskAcceptance bool
    EnableFullRiskAcceptance   bool
    EnableProductTagInheritance bool

    Tags []string

    CreatedAt time.Time
    UpdatedAt time.Time
}

type Criticality string
const (
    CriticalityVeryHigh Criticality = "very high"
    CriticalityHigh     Criticality = "high"
    CriticalityMedium   Criticality = "medium"
    CriticalityLow      Criticality = "low"
    CriticalityVeryLow  Criticality = "very low"
    CriticalityNone     Criticality = "none"
)

type Platform string
const (
    PlatformWeb     Platform = "web"
    PlatformMobile  Platform = "mobile"
    PlatformDesktop Platform = "desktop"
    PlatformAPI     Platform = "api"
    PlatformIoT     Platform = "iot"
)

// domain/engagement/entity.go
// Mirrors Python: dojo/models.py::Engagement
type Engagement struct {
    ID          string
    Name        string
    Description string
    ProductID   string

    Type   EngagementType   // Interactive | CI/CD
    Status EngagementStatus

    TargetStart time.Time
    TargetEnd   time.Time

    // CI/CD metadata
    BuildID    string
    CommitHash string
    BranchTag  string
    SourceCodeManagementURI string

    // Behavior
    DeduplicationOnEngagement bool

    // Tool links (FK to ToolConfiguration IDs)
    BuildServerID           *string
    OrchestrationEngineID   *string
    SCMServerID             *string

    Tags []string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type EngagementType string
const (
    EngagementTypeInteractive EngagementType = "Interactive"
    EngagementTypeCICD        EngagementType = "CI/CD"
)

type EngagementStatus string
const (
    EngagementStatusNotStarted         EngagementStatus = "Not Started"
    EngagementStatusInProgress         EngagementStatus = "In Progress"
    EngagementStatusCompleted          EngagementStatus = "Completed"
    EngagementStatusBlocked            EngagementStatus = "Blocked"
    EngagementStatusCancelled          EngagementStatus = "Cancelled"
    EngagementStatusOnHold             EngagementStatus = "On Hold"
    EngagementStatusWaitingForResource EngagementStatus = "Waiting for Resource"
)

// domain/test/entity.go
// Mirrors Python: dojo/models.py::Test
type Test struct {
    ID           string
    Title        string
    EngagementID string
    TestTypeID   string
    ScanType     string  // Trivy Scan, Bandit, etc. — matches parser

    // Progress
    TargetStart     time.Time
    TargetEnd       time.Time
    PercentComplete int

    // CI/CD metadata
    BuildID    string
    CommitHash string
    BranchTag  string
    Version    string

    Tags []string
    CreatedAt time.Time
    UpdatedAt time.Time
}

// domain/member/entity.go
// Mirrors Python: Product_Member + Product_Type_Member models
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

// domain/tool/entity.go
// Mirrors Python: Tool_Configuration
type ToolConfiguration struct {
    ID          string
    Name        string
    Description string
    ToolType    string  // "GitHub", "GitLab", "Jira", "Slack", etc.
    URL         string
    AuthType    string  // api_key | http_basic | ssh | bearer
    Username    string
    PasswordEnc string  // AES-256 encrypted
    APIKeyEnc   string  // AES-256 encrypted

    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### 4.2 Auto-close Engagements

```go
// usecase/engagement/auto_close_expired.go
// Mirrors Python: dojo/tasks.py::close_expired_risk_acceptances()

// Chạy daily 07:00 — tìm engagements đã qua target_end mà vẫn "In Progress"
func (uc *AutoCloseExpiredUseCase) Execute(ctx context.Context) error {
    expired, err := uc.engagementRepo.FindExpiredActive(ctx, time.Now())
    for _, eng := range expired {
        eng.Status = EngagementStatusCompleted
        uc.engagementRepo.Save(ctx, eng)
        uc.eventPub.Publish(ctx, &events.EngagementClosed{
            EngagementID: eng.ID,
            ProductID:    eng.ProductID,
            Reason:       "auto_close_expired",
        })
    }
    return nil
}
```

---

## 5. gRPC Contract

```protobuf
// proto/product/v1/product.proto
syntax = "proto3";
package product.v1;
option go_package = "github.com/osv/product-service/proto/product/v1";

service ProductService {
    // ProductType CRUD
    rpc GetProductType(GetProductTypeRequest) returns (GetProductTypeResponse);
    rpc ListProductTypes(ListProductTypesRequest) returns (ListProductTypesResponse);

    // Product CRUD
    rpc GetProduct(GetProductRequest) returns (GetProductResponse);
    rpc ListProductsForUser(ListProductsForUserRequest) returns (ListProductsForUserResponse);

    // Engagement
    rpc GetEngagement(GetEngagementRequest) returns (GetEngagementResponse);
    rpc GetOrCreateEngagement(GetOrCreateEngagementRequest) returns (GetOrCreateEngagementResponse);
    rpc UpdateEngagementTimestamps(UpdateEngagementTimestampsRequest) returns (UpdateEngagementTimestampsResponse);

    // Test
    rpc GetTest(GetTestRequest) returns (GetTestResponse);
    rpc GetOrCreateTest(GetOrCreateTestRequest) returns (GetOrCreateTestResponse);
    rpc UpdateTestMetadata(UpdateTestMetadataRequest) returns (UpdateTestMetadataResponse);

    // Members
    rpc GetProductMembers(GetProductMembersRequest) returns (GetProductMembersResponse);
    rpc CheckProductPermission(CheckProductPermissionRequest) returns (CheckProductPermissionResponse);

    // SLA
    rpc GetSLAConfiguration(GetSLAConfigurationRequest) returns (GetSLAConfigurationResponse);
}

message GetOrCreateEngagementRequest {
    string product_id    = 1;
    string name          = 2;
    optional string engagement_type = 3;  // Interactive | CI/CD
    optional string build_id        = 4;
    optional string branch_tag      = 5;
    optional string commit_hash     = 6;
}

message CheckProductPermissionRequest {
    string user_id     = 1;
    string product_id  = 2;
    string permission  = 3;  // "product:view" | "finding:add" | "scan:import"
}

message CheckProductPermissionResponse {
    bool allowed = 1;
    string reason = 2;
}
```

---

## 6. REST API

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET/POST` | `/api/v2/product-types` | JWT | CRUD ProductType |
| `GET/POST` | `/api/v2/products` | JWT | CRUD Product |
| `POST` | `/api/v2/products/{id}/members` | JWT/Owner | Add member |
| `DELETE` | `/api/v2/products/{id}/members/{uid}` | JWT/Owner | Remove member |
| `GET/POST` | `/api/v2/engagements` | JWT | CRUD Engagement |
| `POST` | `/api/v2/engagements/{id}/close` | JWT | Close engagement |
| `POST` | `/api/v2/engagements/{id}/reopen` | JWT | Reopen engagement |
| `GET/POST` | `/api/v2/tests` | JWT | CRUD Test |
| `GET/POST` | `/api/v2/sla-configurations` | JWT | CRUD SLA Config |
| `GET/POST` | `/api/v2/tool-configurations` | JWT/Admin | CRUD Tool Config |

---

## 7. NATS Events Published

```
product.type.created    {product_type_id, name, created_by}
product.created         {product_id, product_type_id, name, sla_config_id, created_by}
product.updated         {product_id, changes}
product.member.added    {product_id, user_id, role}
engagement.created      {engagement_id, product_id, type, name}
engagement.closed       {engagement_id, product_id, reason}
test.created            {test_id, engagement_id, product_id, scan_type}
```

---

## 8. Database Schema

```sql
-- product_types
CREATE TABLE product_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    critical_product BOOLEAN DEFAULT FALSE,
    key_product BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- products
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    product_type_id UUID NOT NULL REFERENCES product_types(id),
    sla_configuration_id UUID,   -- FK → sla_configurations (in sla-service DB, ref by ID)
    platform VARCHAR(50),
    lifecycle VARCHAR(50),
    origin VARCHAR(50),
    business_criticality VARCHAR(50),
    enable_simple_risk_acceptance BOOLEAN DEFAULT FALSE,
    enable_full_risk_acceptance BOOLEAN DEFAULT FALSE,
    enable_product_tag_inheritance BOOLEAN DEFAULT FALSE,
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- engagements
CREATE TABLE engagements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    engagement_type VARCHAR(50) DEFAULT 'Interactive',
    status VARCHAR(50) DEFAULT 'In Progress',
    target_start DATE NOT NULL,
    target_end DATE NOT NULL,
    build_id VARCHAR(255),
    commit_hash VARCHAR(255),
    branch_tag VARCHAR(255),
    source_code_management_uri TEXT,
    deduplication_on_engagement BOOLEAN DEFAULT FALSE,
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_eng_product_active ON engagements(product_id, status);

-- tests
CREATE TABLE tests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255),
    engagement_id UUID NOT NULL REFERENCES engagements(id) ON DELETE CASCADE,
    test_type_id UUID,
    scan_type VARCHAR(255),
    target_start TIMESTAMPTZ,
    target_end TIMESTAMPTZ,
    percent_complete INTEGER DEFAULT 0,
    build_id VARCHAR(255),
    commit_hash VARCHAR(255),
    branch_tag VARCHAR(255),
    version VARCHAR(100),
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- product_members
CREATE TABLE product_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    role_id VARCHAR(50) NOT NULL,  -- Owner|Maintainer|Writer|API_Importer|Reader
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (product_id, user_id)
);

-- tool_configurations
CREATE TABLE tool_configurations (
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

---

## 9. Port Conventions

| Service | gRPC | HTTP |
|---------|:----:|:----:|
| product-service | 9003 | 8083 |

---

## 10. Acceptance Criteria

- [x] `POST /api/v2/products` tạo Product với SLA Config assignment
- [x] `POST /api/v2/engagements` tạo Engagement linked to Product
- [x] `POST /api/v2/tests` tạo Test linked to Engagement
- [x] `POST /api/v2/products/{id}/members` thêm member với role Owner/Writer
- [x] gRPC `GetOrCreateEngagement` hoạt động — nếu engagement tồn tại thì không tạo mới
- [x] gRPC `GetOrCreateTest` hoạt động — dùng bởi scan-orchestrator
- [x] `CheckProductPermission` trả về đúng với RBAC (Reader chỉ xem, không import)
- [x] Auto-close expired engagements chạy daily 07:00
- [x] NATS events được publish sau Create/Update/Close
- [x] Xóa Product sẽ cascade xóa Engagement → Test (không xóa Findings, chỉ unlink)

## Implementation Status: ✅ DONE

> Implemented in `finding-service` (not a separate `product-service` — consolidated per solution design)
> `finding-service/internal/domain/{product,product_type,engagement,test,member,tool}/` — all entities implemented
> `finding-service/internal/usecase/engagement/auto_close_expired.go` — daily 07:00 cron
> `finding-service/internal/delivery/grpc/server/finding_server.go` — GetOrCreateEngagement, GetOrCreateTest, CheckProductPermission
> `finding-service/migrations/{001_products,002_product_types,003_engagements,004_tests,005_product_members,006_tool_configurations}.sql`
> **Note**: Target service consolidated from `product-service` → `finding-service` per architectural decision (avoid service proliferation)
