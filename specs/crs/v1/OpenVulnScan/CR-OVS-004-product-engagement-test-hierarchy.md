# CR-OVS-004 — Product Service: Product/Engagement/Test Hierarchy & CI/CD Integration

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OVS-004 |
| **Tiêu đề** | Product Service — Product/Engagement/Test Hierarchy (DefectDojo-style), Business Criticality, CI/CD Orchestration |
| **Nguồn tham chiếu** | `OpenVulnScan/specs/services/07-product-service.md`, `OpenVulnScan/docs/PRD.md §F-006` |
| **Target Service** | **MỚI**: `product-service` (port gRPC: 50061) |
| **Ưu tiên** | 🟡 Medium |
| **Loại** | New Service |
| **Ngày tạo** | 2026-06-14 |
| **Ngày implement** | 2026-06-17 |
| **Trạng thái** | ✅ Implemented |

---

## 0. Implementation Status

> **Trạng thái**: ✅ **IMPLEMENTED** — 2026-06-17

| Task | File | Trạng thái |
|------|------|------------|
| ProductType/Product/Engagement/Test entities | `product-service/internal/domain/entity/entities.go` | ✅ Done |
| CI/CD orchestrator use case | `product-service/internal/usecase/orchestrator/cicd.go` | ✅ Done |
| Product HTTP handlers | `product-service/internal/delivery/http/handlers.go` | ✅ Done |

**Chi tiết implementation**:
- **Entities**: `ProductType → Product → Engagement → Test` hiệrarchy (DefectDojo-style)
- **ComputeGrade**: `Critical×9.5 + High×7.5 + Medium×4.5 + Low×1.5` cấp 0–100
- **CI/CD Orchestrator**: find-or-create Product (case-insensitive), auto Engagement + Test, gRPC import findings, close engagement
- **BusinessCriticality**: `very_high/high/medium/low` → auto-create tĩght SLA (Critical=3d, High=14d khi `very_high`)
- **REST API**: CRUD cho ProductType/Product/Engagement/Test + `POST /orchestrate/cicd`

---

## 1. Tổng quan

OSV không có khái niệm về "sản phẩm" hay "engagement". OpenVulnScan (theo mô hình DefectDojo) tổ chức findings theo **Product Hierarchy**:

```
ProductType (Category: "Web Application", "Mobile App", "Infrastructure")
  └── Product (Software: "E-Commerce Platform", "API Gateway")
        └── Engagement (Testing event: "Q2 2026 Security Audit", "CI pipeline #1234")
              └── Test (Specific scan: "Nmap Full Scan", "ZAP Active Scan")
                    └── Finding (Vulnerability finding → managed by finding-service)
```

---

## 2. Gap Analysis

| Feature | OSV | OpenVulnScan |
|---------|-----|-------------|
| Product management | ❌ | ✅ |
| Product categorization | ❌ | ✅ ProductType |
| Business criticality | ❌ | ✅ very high/high/medium/low |
| Lifecycle tracking | ❌ | ✅ construction/production/retirement |
| Engagement management | ❌ | ✅ Interactive + CI/CD |
| CI/CD pipeline context | ❌ | ✅ build_id, commit_hash, branch |
| Test/Scan context tracking | ❌ | ✅ |
| Orchestrator (CI/CD) | ❌ | ✅ auto product+engagement creation |
| SLA per product | ❌ | ✅ |
| Finding dedup per engagement | ❌ | ✅ |

---

## 3. Domain Model

### 3.1 ProductType Entity

```go
// product-service/internal/domain/product_type/entity.go

type ProductType struct {
    ID              uuid.UUID
    Name            string  // "Web Application", "Mobile App", "Infrastructure", "API"
    Description     string
    CriticalProduct bool    // flag for high-priority product types
    KeyProduct      bool    // flag for business-critical products
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

### 3.2 Product Entity

```go
// product-service/internal/domain/product/entity.go

type BusinessCriticality string
const (
    BCVeryHigh BusinessCriticality = "very high"
    BCHigh     BusinessCriticality = "high"
    BCMedium   BusinessCriticality = "medium"
    BCLow      BusinessCriticality = "low"
    BCVeryLow  BusinessCriticality = "very low"
)

type Lifecycle string
const (
    LCConstruction Lifecycle = "construction"  // In development
    LCProduction   Lifecycle = "production"    // Live product
    LCRetirement   Lifecycle = "retirement"    // EOL
)

type Platform string
const (
    PlatformWeb     Platform = "web"
    PlatformAPI     Platform = "api"
    PlatformMobile  Platform = "mobile"
    PlatformDesktop Platform = "desktop"
)

type Product struct {
    ID                         uuid.UUID
    ProductTypeID              uuid.UUID
    Name                       string
    Description                string
    ProdNumericGrade           int               // 1-100, overall security score
    BusinessCriticality        BusinessCriticality
    Platform                   Platform
    Lifecycle                  Lifecycle
    Origin                     string            // internal|external|partner
    ExternalAudience           bool              // Exposed to public?
    InternetAccessible         bool              // Internet-facing?
    EnableFullRiskAcceptance   bool              // Allow accepting all risks?
    EnableSimpleRiskAcceptance bool
    Tags                       []string
    CreatedAt                  time.Time
    UpdatedAt                  time.Time
}

func NewProduct(productTypeID uuid.UUID, name, description string) (*Product, error) {
    if strings.TrimSpace(name) == "" {
        return nil, ErrProductNameRequired
    }
    return &Product{
        ID:                  uuid.New(),
        ProductTypeID:       productTypeID,
        Name:                name,
        Description:         description,
        BusinessCriticality: BCMedium,
        Platform:            PlatformWeb,
        Lifecycle:           LCProduction,
        Tags:                []string{},
        CreatedAt:           time.Now().UTC(),
        UpdatedAt:           time.Now().UTC(),
    }, nil
}
```

### 3.3 Engagement Entity

```go
// product-service/internal/domain/engagement/entity.go

type EngagementType string
const (
    TypeInteractive EngagementType = "Interactive"  // Manual security assessment
    TypeCICD        EngagementType = "CI/CD"        // Automated pipeline scan
)

type EngagementStatus string
const (
    StatusNotStarted EngagementStatus = "Not Started"
    StatusInProgress EngagementStatus = "In Progress"
    StatusOnHold     EngagementStatus = "On Hold"
    StatusCompleted  EngagementStatus = "Completed"
    StatusCancelled  EngagementStatus = "Cancelled"
)

type Engagement struct {
    ID                        uuid.UUID
    ProductID                 uuid.UUID
    Name                      string
    Description               string
    LeadID                    *uuid.UUID  // Security lead user
    EngagementType            EngagementType
    Status                    EngagementStatus
    StartDate                 time.Time
    EndDate                   *time.Time
    Version                   string      // Application version under test (e.g., "v2.3.1")
    BuildID                   string      // CI build ID (e.g., "build-1234")
    CommitHash                string      // Git commit SHA
    BranchTag                 string      // Git branch (e.g., "main", "release/v2.3")
    SourceCodeManagementURI   string      // Repo URL
    DeduplicationOnEngagement bool        // Scope dedup to this engagement only
    Tags                      []string
    CreatedAt                 time.Time
    UpdatedAt                 time.Time
}

func (e *Engagement) Close() error {
    if e.Status == StatusCompleted { return ErrAlreadyClosed }
    e.Status = StatusCompleted
    now := time.Now().UTC()
    e.EndDate = &now
    e.UpdatedAt = now
    return nil
}

func (e *Engagement) IsCICD() bool { return e.EngagementType == TypeCICD }
```

### 3.4 Test Entity

```go
// product-service/internal/domain/test/entity.go
// "Test" = specific scan/assessment context

type Test struct {
    ID             uuid.UUID
    EngagementID   uuid.UUID
    Title          string        // e.g., "Nmap Full Scan - 192.168.1.0/24"
    Description    string
    TestType       string        // "nmap"|"zap"|"agent"|"manual"|"dast"|"sast"
    TargetStart    *time.Time
    TargetEnd      *time.Time
    ScanID         *uuid.UUID    // Link to scan-service scan
    FindingCount   int           // populated from finding-service
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

---

## 4. Use Cases

### 4.1 CRUD Operations

```go
// product-service/internal/usecase/product/create.go

type CreateProductInput struct {
    ProductTypeID       uuid.UUID
    Name                string
    Description         string
    BusinessCriticality BusinessCriticality
    Platform            Platform
    Lifecycle           Lifecycle
    ExternalAudience    bool
    InternetAccessible  bool
    Tags                []string
}

// product-service/internal/usecase/engagement/create.go

type CreateEngagementInput struct {
    ProductID              uuid.UUID
    Name                   string
    Description            string
    EngagementType         EngagementType
    LeadID                 *uuid.UUID
    StartDate              time.Time
    EndDate                *time.Time
    Version                string
    BuildID                string
    CommitHash             string
    BranchTag              string
    SourceCodeManagementURI string
    DeduplicationOnEngagement bool
    Tags                   []string
}
```

### 4.2 CI/CD Orchestrator

```go
// product-service/internal/usecase/orchestrator/cicd.go
// Handles automated CI/CD pipeline integration flow

type CICDTriggerInput struct {
    ProductName   string      // Find or create product with this name
    ProductType   string      // Product type name
    BuildID       string
    CommitHash    string
    BranchTag     string
    RepoURL       string
    Version       string
    ScanResults   []ScanFinding
}

type CICDTriggerOutput struct {
    ProductID    uuid.UUID
    EngagementID uuid.UUID
    TestID       uuid.UUID
    FindingCount int
    NewFindings  int
    Duplicates   int
}

func (uc *CICDOrchestrator) Execute(ctx context.Context, in CICDTriggerInput) (*CICDTriggerOutput, error) {
    // 1. Find or create ProductType
    productType, _ := uc.productTypeRepo.FindByName(ctx, in.ProductType)
    if productType == nil {
        productType, _ = uc.productTypeRepo.Create(ctx, &ProductType{Name: in.ProductType})
    }

    // 2. Find or create Product (by name)
    product, _ := uc.productRepo.FindByName(ctx, in.ProductName)
    if product == nil {
        product, _ = uc.createProduct.Execute(ctx, CreateProductInput{
            ProductTypeID: productType.ID,
            Name:          in.ProductName,
        })
    }

    // 3. Create new Engagement (one per CI/CD run)
    engagement, _ := uc.createEngagement.Execute(ctx, CreateEngagementInput{
        ProductID:      product.ID,
        Name:           fmt.Sprintf("CI/CD Pipeline - Build #%s", in.BuildID),
        EngagementType: TypeCICD,
        BuildID:        in.BuildID,
        CommitHash:     in.CommitHash,
        BranchTag:      in.BranchTag,
        SourceCodeManagementURI: in.RepoURL,
        Version:        in.Version,
    })

    // 4. Create Test
    test, _ := uc.createTest.Execute(ctx, CreateTestInput{
        EngagementID: engagement.ID,
        Title:        fmt.Sprintf("Automated Scan - %s", in.BranchTag),
        TestType:     "automated",
        TargetStart:  time.Now().Pointer(),
    })

    // 5. Import findings via finding-service gRPC
    var newFindings, duplicates int
    for _, sf := range in.ScanResults {
        result, _ := uc.findingSvcClient.CreateFinding(ctx, &FindingRequest{
            Title:        sf.Title,
            CVE:          sf.CVEID,
            Severity:     sf.Severity,
            ComponentName: sf.Package,
            ComponentVersion: sf.Version,
            TestID:       test.ID,
            EngagementID: engagement.ID,
            ProductID:    product.ID,
        })
        if result.Duplicate { duplicates++ } else { newFindings++ }
    }

    // 6. Close engagement
    engagement.Close()
    uc.engagementRepo.Update(ctx, engagement)

    return &CICDTriggerOutput{
        ProductID:    product.ID,
        EngagementID: engagement.ID,
        TestID:       test.ID,
        FindingCount: len(in.ScanResults),
        NewFindings:  newFindings,
        Duplicates:   duplicates,
    }, nil
}
```

---

## 5. Database Schema

```sql
-- product-service migrations/

CREATE TABLE product_types (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             VARCHAR(255) UNIQUE NOT NULL,
    description      TEXT,
    critical_product BOOLEAN NOT NULL DEFAULT FALSE,
    key_product      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE products (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_type_id             UUID NOT NULL REFERENCES product_types(id),
    name                        VARCHAR(255) NOT NULL,
    description                 TEXT,
    prod_numeric_grade          INT CHECK (prod_numeric_grade BETWEEN 0 AND 100),
    business_criticality        VARCHAR(20) CHECK (business_criticality IN
                                    ('very high','high','medium','low','very low')),
    platform                    VARCHAR(50) CHECK (platform IN ('web','api','mobile','desktop')),
    lifecycle                   VARCHAR(20) CHECK (lifecycle IN
                                    ('construction','production','retirement')),
    origin                      VARCHAR(50) CHECK (origin IN ('internal','external','partner')),
    external_audience           BOOLEAN NOT NULL DEFAULT FALSE,
    internet_accessible         BOOLEAN NOT NULL DEFAULT FALSE,
    enable_full_risk_acceptance  BOOLEAN NOT NULL DEFAULT FALSE,
    enable_simple_risk_acceptance BOOLEAN NOT NULL DEFAULT FALSE,
    tags                        TEXT[] DEFAULT '{}',
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_products_type ON products(product_type_id);

CREATE TABLE engagements (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id                  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name                        VARCHAR(255) NOT NULL,
    description                 TEXT,
    lead_id                     UUID,
    engagement_type             VARCHAR(20) NOT NULL DEFAULT 'Interactive',
    status                      VARCHAR(20) NOT NULL DEFAULT 'In Progress',
    start_date                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_date                    TIMESTAMPTZ,
    version                     VARCHAR(100),
    build_id                    VARCHAR(100),
    commit_hash                 VARCHAR(64),
    branch_tag                  VARCHAR(255),
    source_code_management_uri  TEXT,
    deduplication_on_engagement BOOLEAN NOT NULL DEFAULT FALSE,
    tags                        TEXT[] DEFAULT '{}',
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_engagements_product ON engagements(product_id);
CREATE INDEX idx_engagements_status  ON engagements(status);
CREATE INDEX idx_engagements_build   ON engagements(build_id) WHERE build_id IS NOT NULL;

CREATE TABLE tests (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id  UUID NOT NULL REFERENCES engagements(id) ON DELETE CASCADE,
    title          VARCHAR(255),
    description    TEXT,
    test_type      VARCHAR(100),  -- nmap|zap|agent|manual|dast|sast|automated
    target_start   TIMESTAMPTZ,
    target_end     TIMESTAMPTZ,
    scan_id        UUID,          -- Link to scan-service
    finding_count  INT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_tests_engagement ON tests(engagement_id);
```

---

## 6. API Routes

```
# ProductType
GET    /api/v1/product-types         → List product types
POST   /api/v1/product-types         → Create product type (admin)
GET    /api/v1/product-types/{id}    → Get product type

# Product
GET    /api/v1/products              → List products (with filters)
POST   /api/v1/products              → Create product
GET    /api/v1/products/{id}         → Get product details
PUT    /api/v1/products/{id}         → Update product
DELETE /api/v1/products/{id}         → Delete product

# Engagement
GET    /api/v1/products/{id}/engagements     → List engagements for product
POST   /api/v1/products/{id}/engagements     → Create engagement
GET    /api/v1/engagements/{id}              → Get engagement details
PUT    /api/v1/engagements/{id}              → Update engagement
POST   /api/v1/engagements/{id}/close        → Close engagement

# Test
GET    /api/v1/engagements/{id}/tests        → List tests
POST   /api/v1/engagements/{id}/tests        → Create test
GET    /api/v1/tests/{id}                    → Get test details

# CI/CD Orchestrator
POST   /api/v1/orchestrate/cicd             → CI/CD pipeline trigger endpoint
```

---

## 7. Acceptance Criteria

- [ ] `POST /api/v1/products` → product được tạo với ProductType link
- [ ] `POST /api/v1/products/{id}/engagements` với `type=CI/CD` → engagement có build_id, commit_hash
- [ ] `GET /api/v1/products` → list paginated với filters: type, lifecycle, business_criticality
- [ ] `POST /api/v1/orchestrate/cicd` với existing product name → reuse existing product
- [ ] CI/CD orchestrator: creates 1 engagement per pipeline run
- [ ] CI/CD orchestrator: creates 1 test per orchestration call
- [ ] CI/CD orchestrator: imports findings → links to test/engagement/product
- [ ] `POST /api/v1/engagements/{id}/close` → status=Completed, end_date=now
- [ ] Product `prod_numeric_grade` (0-100) được tính toán dựa trên finding counts
- [ ] `business_criticality=very high` → default SLA = Critical:7d, High:14d (tighter)
