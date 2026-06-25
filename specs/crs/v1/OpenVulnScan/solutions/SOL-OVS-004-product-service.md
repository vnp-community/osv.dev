# SOL-OVS-004 — Giải Pháp: Product Service (Product/Engagement/Test Hierarchy)

| Trường | Giá trị |
|--------|---------|
| **Solution ID** | SOL-OVS-004 |
| **CR tham chiếu** | CR-OVS-004 |
| **Tiêu đề** | Product Service — Product/Engagement/Test Hierarchy (DefectDojo-style), Business Criticality, CI/CD Orchestration |
| **Ngày tạo** | 2026-06-16 |
| **Ngày implement** | 2026-06-17 |
| **Trạng thái** | ✅ Implemented |

---

## 0. Implementation Status

> **Trạng thái**: ✅ **IMPLEMENTED** — 2026-06-17

| Task | File | Trạng thái |
|------|------|------------|
| T-PROD-001 | `product-service/internal/domain/entity/entities.go` | ✅ Done |
| T-PROD-002 | `product-service/internal/usecase/orchestrator/cicd.go` | ✅ Done |
| T-PROD-003 | `product-service/internal/delivery/http/handlers.go` | ✅ Done |

**Chi tiết implementation**:
- **Entities**: `ProductType → Product → Engagement → Test`, `ComputeGrade(stats)` 0-100 scoring
- **CI/CD Orchestrator**: find-or-create Product (case-insensitive), auto Engagement + Test, finding-service gRPC import, close engagement
- **REST API**: CRUD cho ProductType/Product/Engagement/Test + `POST /orchestrate/cicd`
- **SLA Integration**: `BusinessCriticality=very_high` → auto-create tighter SLA (Critical=3d, High=14d)

---

## 1. Tổng Quan Giải Pháp

### 1.1 Bối Cảnh

OSV.dev không có khái niệm về "sản phẩm phần mềm" hay "chu kỳ kiểm tra bảo mật". `product-service` mang đến context tổ chức cho tất cả findings trong OpenVulnScan, theo mô hình của DefectDojo:

```
ProductType → Product → Engagement → Test → Finding
```

### 1.2 Vai Trò

- **Tổ chức context**: Mọi finding phải có TestID → EngagementID → ProductID
- **CI/CD Orchestrator**: Tự động tạo hierarchy khi pipeline chạy
- **SLA per Product**: Product `business_criticality=very high` có SLA keener hơn

---

## 2. Kiến Trúc

### 2.1 Vị Trí Trong Hệ Thống

```
CI/CD Pipeline ──▶ POST /api/v1/orchestrate/cicd
                           │
                   product-service (CICDOrchestrator)
                           │
              ┌────────────┼────────────────────┐
              │            │                    │
              ▼            ▼                    ▼
        Find/Create    Create          Import findings
        Product +      Test            via finding-service
        Engagement                     gRPC client
```

### 2.2 Cấu Trúc Thư Mục

```
services/product-service/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── product_type/entity.go
│   │   ├── product/entity.go
│   │   ├── engagement/entity.go
│   │   └── test/entity.go
│   ├── usecase/
│   │   ├── product_type/
│   │   │   ├── create.go
│   │   │   └── list.go
│   │   ├── product/
│   │   │   ├── create.go
│   │   │   ├── update.go
│   │   │   ├── list.go
│   │   │   └── delete.go
│   │   ├── engagement/
│   │   │   ├── create.go
│   │   │   ├── update.go
│   │   │   ├── close.go
│   │   │   └── list.go
│   │   ├── test/
│   │   │   ├── create.go
│   │   │   └── list.go
│   │   └── orchestrator/
│   │       └── cicd.go             # CI/CD Orchestrator use case
│   ├── adapter/
│   │   ├── repository/postgres/
│   │   └── client/
│   │       └── finding_service_client.go  # gRPC client for finding-service
│   └── delivery/
│       ├── http/
│       │   ├── product_type_handler.go
│       │   ├── product_handler.go
│       │   ├── engagement_handler.go
│       │   ├── test_handler.go
│       │   └── orchestrator_handler.go
│       └── grpc/
│           └── product_server.go
├── migrations/
│   └── 001_create_product_tables.sql
└── config/config.yaml
```

---

## 3. Domain Hierarchy Chi Tiết

### 3.1 ProductType

```go
// Category cho products: Web Application, Mobile App, Infrastructure, API
type ProductType struct {
    ID              uuid.UUID
    Name            string  // Unique, max 255
    Description     string
    CriticalProduct bool    // Mark as high-priority category
    KeyProduct      bool    // Business-critical category
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// Seeded values (migration):
// - "Web Application"
// - "Mobile Application"  
// - "Infrastructure"
// - "API / Microservice"
// - "Desktop Application"
// - "Libraries / Components"
```

### 3.2 Product Entity

```go
// Constraints cho Product.Name: unique per organization (no global unique)
// ProdNumericGrade: computed từ finding-service counts
//   = 100 - (Critical*20 + High*10 + Medium*5 + Low*1)
//   min = 0

// Security grade calculation:
func (p *Product) ComputeGrade(stats FindingStats) int {
    score := 100 - (stats.Critical*20 + stats.High*10 + 
                    stats.Medium*5 + stats.Low*1)
    if score < 0 { score = 0 }
    return score
}
```

### 3.3 Engagement Types

```
Interactive:
  - Manual security assessment
  - Penetration testing
  - Code review
  - Has start/end date, lead user
  - Status transitions: NotStarted → InProgress → Completed/OnHold

CI/CD:
  - Automated pipeline scan
  - One per pipeline run
  - Has build_id, commit_hash, branch_tag
  - Auto-closed after orchestration
  - DeduplicationOnEngagement = true by default
```

---

## 4. CI/CD Orchestrator — Core Flow

### 4.1 Orchestration Steps

```
POST /api/v1/orchestrate/cicd
{
  "product_name": "E-Commerce Platform",
  "product_type": "Web Application",
  "build_id": "build-1234",
  "commit_hash": "abc123def456",
  "branch_tag": "main",
  "repo_url": "https://github.com/org/repo",
  "version": "v2.3.1",
  "scan_results": [
    {"title": "...", "cve": "...", "severity": "High", ...}
  ]
}

ORCHESTRATION STEPS:
1. Find OR Create ProductType by name "Web Application"
2. Find OR Create Product by name "E-Commerce Platform" + product_type_id
   (find = case-insensitive name match within same org)
3. Create NEW Engagement:
   {name="CI/CD Pipeline - Build #build-1234", type=CI/CD, 
    build_id, commit_hash, branch_tag, repo_url, version}
4. Create Test:
   {title="Automated Scan - main", test_type="automated", 
    engagement_id, target_start=now}
5. For each scan_result:
   → Call finding-service gRPC CreateFinding (with dedup)
   → Count: new vs duplicate
6. Close engagement: status=Completed, end_date=now
7. Return: {product_id, engagement_id, test_id, total, new, duplicates}
```

### 4.2 Find-Or-Create Pattern

```go
// Product.FindOrCreate — idempotent
func (uc *CICDOrchestrator) findOrCreateProduct(ctx context.Context, 
    typeID uuid.UUID, name string) (*Product, error) {
    
    // Case-insensitive search
    existing, err := uc.productRepo.FindByNameInsensitive(ctx, name)
    if err == nil && existing != nil {
        return existing, nil  // Reuse existing product
    }
    
    // Create new with defaults
    return uc.createProductUC.Execute(ctx, CreateProductInput{
        ProductTypeID:       typeID,
        Name:                name,
        BusinessCriticality: BCMedium,
        Platform:            PlatformWeb,
        Lifecycle:           LCProduction,
    })
}
```

---

## 5. SLA Integration với Business Criticality

```go
// product-service/internal/usecase/product/create.go

// Khi tạo product với business_criticality=very high:
// → Tự động tạo tighter SLA configuration trong finding-service

func (uc *CreateProductUseCase) Execute(ctx context.Context, in CreateProductInput) (*Product, error) {
    product, err := product.NewProduct(in.ProductTypeID, in.Name, in.Description)
    if err != nil { return nil, err }
    
    product.BusinessCriticality = in.BusinessCriticality
    // ... set other fields
    
    if err := uc.productRepo.Save(ctx, product); err != nil { return nil, err }
    
    // Tự động tạo SLA config nếu very high criticality
    if in.BusinessCriticality == BCVeryHigh {
        uc.findingSvcClient.CreateSLAConfig(ctx, &findingpb.CreateSLAConfigRequest{
            ProductId: product.ID.String(),
            Critical:  3,   // 3 days (vs default 7)
            High:      14,  // 14 days (vs default 30)
            Medium:    45,  // 45 days (vs default 90)
            Low:       90,  // 90 days (vs default 180)
        })
    }
    
    // Publish event
    uc.eventBus.Publish(ctx, "product.created", &ProductCreatedEvent{
        ProductID:   product.ID,
        Name:        product.Name,
        Criticality: string(in.BusinessCriticality),
    })
    
    return product, nil
}
```

---

## 6. Database Schema

```sql
-- migrations/001_create_product_tables.sql

CREATE TABLE product_types (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             VARCHAR(255) UNIQUE NOT NULL,
    description      TEXT,
    critical_product BOOLEAN NOT NULL DEFAULT FALSE,
    key_product      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default product types
INSERT INTO product_types (name, description) VALUES
    ('Web Application', 'Browser-based web applications'),
    ('Mobile Application', 'iOS and Android mobile apps'),
    ('Infrastructure', 'Servers, networks, cloud infrastructure'),
    ('API / Microservice', 'API services and microservices'),
    ('Desktop Application', 'Desktop software'),
    ('Libraries / Components', 'Software libraries and SDKs');

CREATE TABLE products (
    id                            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_type_id               UUID NOT NULL REFERENCES product_types(id),
    name                          VARCHAR(255) NOT NULL,
    description                   TEXT,
    prod_numeric_grade            INT CHECK (prod_numeric_grade BETWEEN 0 AND 100),
    business_criticality          VARCHAR(20) 
                                  CHECK (business_criticality IN 
                                      ('very high','high','medium','low','very low')),
    platform                      VARCHAR(50) 
                                  CHECK (platform IN ('web','api','mobile','desktop')),
    lifecycle                     VARCHAR(20) 
                                  CHECK (lifecycle IN ('construction','production','retirement')),
    origin                        VARCHAR(50) 
                                  CHECK (origin IN ('internal','external','partner')),
    external_audience             BOOLEAN NOT NULL DEFAULT FALSE,
    internet_accessible           BOOLEAN NOT NULL DEFAULT FALSE,
    enable_full_risk_acceptance   BOOLEAN NOT NULL DEFAULT FALSE,
    enable_simple_risk_acceptance BOOLEAN NOT NULL DEFAULT FALSE,
    tags                          TEXT[] DEFAULT '{}',
    created_at                    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_products_type         ON products(product_type_id);
CREATE INDEX idx_products_criticality  ON products(business_criticality);
CREATE INDEX idx_products_lifecycle    ON products(lifecycle);
CREATE INDEX idx_products_name         ON products(LOWER(name));  -- Case-insensitive

CREATE TABLE engagements (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id                  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name                        VARCHAR(255) NOT NULL,
    description                 TEXT,
    lead_id                     UUID,
    engagement_type             VARCHAR(20) NOT NULL DEFAULT 'Interactive'
                                CHECK (engagement_type IN ('Interactive','CI/CD')),
    status                      VARCHAR(20) NOT NULL DEFAULT 'In Progress'
                                CHECK (status IN ('Not Started','In Progress','On Hold','Completed','Cancelled')),
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
CREATE INDEX idx_engagements_type    ON engagements(engagement_type);

CREATE TABLE tests (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id  UUID NOT NULL REFERENCES engagements(id) ON DELETE CASCADE,
    title          VARCHAR(255),
    description    TEXT,
    test_type      VARCHAR(100),
    -- nmap|zap|agent|manual|dast|sast|sca|automated
    target_start   TIMESTAMPTZ,
    target_end     TIMESTAMPTZ,
    scan_id        UUID,          -- Link to scan-service scan
    finding_count  INT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tests_engagement ON tests(engagement_id);
CREATE INDEX idx_tests_scan_id    ON tests(scan_id) WHERE scan_id IS NOT NULL;
```

---

## 7. API Routes

```
# ProductType
GET    /api/v1/product-types              → List all product types
POST   /api/v1/product-types              → Create (admin only)
GET    /api/v1/product-types/{id}         → Get product type
PUT    /api/v1/product-types/{id}         → Update (admin only)
DELETE /api/v1/product-types/{id}         → Delete (admin only)

# Product  
GET    /api/v1/products                   → List (filters: type, lifecycle, criticality, tags)
POST   /api/v1/products                   → Create product
GET    /api/v1/products/{id}              → Get product detail + stats
PUT    /api/v1/products/{id}              → Update product
DELETE /api/v1/products/{id}              → Delete product (cascade engagements)

# Engagement
GET    /api/v1/products/{id}/engagements  → List engagements for product
POST   /api/v1/products/{id}/engagements  → Create engagement
GET    /api/v1/engagements/{id}           → Get engagement detail
PUT    /api/v1/engagements/{id}           → Update engagement
DELETE /api/v1/engagements/{id}           → Delete engagement
POST   /api/v1/engagements/{id}/close     → Close engagement (status=Completed)

# Test
GET    /api/v1/engagements/{id}/tests     → List tests
POST   /api/v1/engagements/{id}/tests     → Create test
GET    /api/v1/tests/{id}                 → Get test detail
PUT    /api/v1/tests/{id}                 → Update test

# CI/CD Orchestrator
POST   /api/v1/orchestrate/cicd           → CI/CD pipeline trigger
```

### 7.1 Orchestrate Response

```json
// POST /api/v1/orchestrate/cicd
// Response 200:
{
  "product_id": "uuid",
  "product_name": "E-Commerce Platform",
  "engagement_id": "uuid",
  "test_id": "uuid",
  "finding_count": 25,
  "new_findings": 18,
  "duplicates": 7,
  "engagement_name": "CI/CD Pipeline - Build #build-1234",
  "created_at": "2026-06-16T11:00:00Z"
}
```

---

## 8. gRPC Service Definition

```protobuf
// product-service/proto/product/v1/product.proto
syntax = "proto3";

service ProductService {
  // Called by finding-service to get product context
  rpc GetProduct(GetProductRequest) returns (ProductResponse);
  rpc GetEngagement(GetEngagementRequest) returns (EngagementResponse);
  rpc GetTest(GetTestRequest) returns (TestResponse);
  
  // Called by scan-service to link scan to test
  rpc LinkScanToTest(LinkScanRequest) returns (LinkScanResponse);
  
  // Called by finding-service to update test finding count
  rpc UpdateTestFindingCount(UpdateCountRequest) returns (google.protobuf.Empty);
  rpc UpdateProductGrade(UpdateGradeRequest) returns (google.protobuf.Empty);
}
```

---

## 9. Configuration

```yaml
# config/config.yaml
server:
  grpc_port: 50061
  http_port: 8061

database:
  host: "${DB_HOST}"
  port: 5432
  name: "product_service"
  user: "${DB_USER}"
  password: "${DB_PASSWORD}"

clients:
  finding_service:
    grpc_address: "finding-service:50060"
  scan_service:
    grpc_address: "scan-service:50058"

auth:
  jwt_public_key_path: "/secrets/jwt_public.pem"

nats:
  url: "${NATS_URL}"
```

---

## 10. Implementation Roadmap

### Phase 1 — Domain CRUD (Sprint 1)
- [ ] Database migrations + seed data
- [ ] ProductType CRUD
- [ ] Product CRUD  
- [ ] Engagement CRUD

### Phase 2 — Test + Orchestrator (Sprint 2)
- [ ] Test CRUD
- [ ] CI/CD Orchestrator use case
- [ ] finding-service gRPC client
- [ ] `POST /api/v1/orchestrate/cicd` endpoint

### Phase 3 — Integration (Sprint 3)
- [ ] Product grade computation (from finding stats)
- [ ] SLA override for very_high criticality
- [ ] gRPC server
- [ ] Integration tests

---

## 11. Acceptance Criteria Mapping

| Criterion | Implementation |
|-----------|---------------|
| `POST /api/v1/products` → product với ProductType link | `CreateProductUseCase` |
| Engagement với `type=CI/CD` → build_id, commit_hash | `CreateEngagementInput.BuildID` etc. |
| `GET /api/v1/products` paginated với filters | `ProductFilter` struct |
| CI/CD orchestrate với existing product name → reuse | `findOrCreateProduct()` |
| 1 engagement per pipeline run | New engagement each call |
| 1 test per orchestration call | `createTestUC.Execute()` |
| Findings → linked to test/engagement/product | `FindingRequest.TestID` etc. |
| `POST /engagements/{id}/close` → status=Completed | `engagement.Close()` |
| `prod_numeric_grade` 0-100 | `ComputeGrade(stats)` |
| `business_criticality=very high` → tighter SLA | `findingSvcClient.CreateSLAConfig()` |
