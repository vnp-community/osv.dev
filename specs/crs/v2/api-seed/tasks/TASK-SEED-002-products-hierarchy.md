# TASK-SEED-002: Product Hierarchy Seed (finding-service + gateway)

> **Solution:** [SOL-SEED-002](../solutions/SOL-SEED-002-products-hierarchy.md)  
> **Service:** `services/finding-service` + `apps/osv`  
> **Depends on:** TASK-SEED-001-D (users phải tồn tại trước)  
> **Blocking:** TASK-SEED-003  
> **Status:** ✅ COMPLETED — 2026-06-18 (gateway routes verified 2026-06-19)  
> **Files tạo/sửa:**  
> - `internal/usecase/product/seed.go` (NEW — product_usecase package)  
> - `internal/delivery/http/product_seed_handler.go` (NEW)  
> - `internal/delivery/http/router.go` (cập nhật SEED-002 + SEED-003 routes)  
> - `apps/osv/internal/gateway/router.go` (thêm SEED-002 gateway routes: bulk, import, {id}/seed)

## Mục tiêu

Thêm bulk create và import endpoints cho Product hierarchy (ProductType, Product, Engagement, Test) vào finding-service và expose qua gateway.

## Bước 1: Khảo sát cấu trúc finding-service

```bash
# Tìm router file
find /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service \
  -name "router.go" | head -5

# Xem handlers hiện có cho products
find /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service \
  -name "*.go" | xargs grep -l "ProductType\|CreateProduct\|BulkCreate" 2>/dev/null | head -10

# Xem product repo interface
find /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/domain \
  -name "*.go" | xargs grep -l "ProductRepository\|ProductTypeRepository" 2>/dev/null
```

## Bước 2: Thêm Entity types

**File:** `internal/domain/entity/product.go` (hoặc tương đương)

```go
// ProductCreateInput là input cho bulk/import product creation
type ProductCreateInput struct {
    Name                string
    ProductTypeID       *uuid.UUID // nếu nil, dùng ProductTypeName để resolve
    ProductTypeName     string     // auto-create nếu không tồn tại
    Description         string
    BusinessCriticality string     // high|medium|low
    Platform            string     // web|api|mobile|desktop
    Lifecycle           string     // construction|production|retirement
    Origin              string     // internal|external|partner
    ExternalAudience    bool
    InternetAccessible  bool
    Tags                []string
    SLAConfigurationID  *uuid.UUID // optional: assign SLA ngay khi tạo
}

// ProductSeedInput là input cho composite seed (Engagement + Test)
type ProductSeedInput struct {
    Engagement *EngagementCreateInput
    Test       *TestCreateInput // optional
}

// EngagementCreateInput là input tạo engagement
type EngagementCreateInput struct {
    Name           string
    EngagementType string // "Interactive" | "CI/CD"
    StartDate      time.Time
    Version        string
    Tags           []string
}

// TestCreateInput là input tạo test
type TestCreateInput struct {
    Title       string
    TestType    string    // nmap|zap|agent|manual|dast|sast
    TargetStart *time.Time
    TargetEnd   *time.Time
}

// BulkCreateResult là kết quả per-item
type BulkCreateResult struct {
    Name    string
    Status  string // "created" | "error"
    ID      *uuid.UUID
    Message string
}
```

## Bước 3: Thêm methods vào Repository interfaces

**File:** `internal/domain/repository/product_repository.go`

```go
// Thêm vào ProductTypeRepository:
CreateBulk(ctx context.Context, types []entity.ProductType) ([]entity.BulkCreateResult, error)

// Thêm vào ProductRepository:
CreateBulk(ctx context.Context, products []entity.ProductCreateInput) ([]entity.BulkCreateResult, error)
FindByName(ctx context.Context, name string) (*entity.Product, error)
```

## Bước 4: Tạo UseCase `product_seed`

**File:** `internal/usecase/product_seed/usecase.go` (NEW)

```go
package productseed

type UseCase struct {
    productTypeRepo repository.ProductTypeRepository
    productRepo     repository.ProductRepository
    engagementRepo  repository.EngagementRepository
    testRepo        repository.TestRepository
    slaBaseURL      string       // base URL của sla-service
    httpClient      *http.Client
    eventPub        events.Publisher
}

// BulkCreateProductTypes — gọi productTypeRepo.CreateBulk
func (uc *UseCase) BulkCreateProductTypes(ctx context.Context, types []entity.ProductType) (BulkResult, error)

// BulkCreateProducts — resolve ProductType name → ID, sau đó INSERT
// Nếu SLAConfigurationID != nil: gọi HTTP POST sla-service để assign
func (uc *UseCase) BulkCreateProducts(ctx context.Context, products []entity.ProductCreateInput) (BulkResult, error)

// SeedProductHierarchy — tạo Engagement + optional Test trong 1 transaction
func (uc *UseCase) SeedProductHierarchy(ctx context.Context, productID uuid.UUID, in entity.ProductSeedInput) (*SeedResult, error)

// ImportProducts — parse JSON/CSV → BulkCreateProducts
func (uc *UseCase) ImportProducts(ctx context.Context, r io.Reader, format string) (ImportResult, error)
```

**Logic `BulkCreateProducts` (quan trọng):**
```
For each ProductCreateInput:
  1. Nếu ProductTypeID == nil: lookup ProductTypeName
     → Nếu không thấy: tạo mới ProductType (INSERT)
  2. INSERT INTO products ON CONFLICT (name, product_type_id) DO NOTHING
  3. Nếu SLAConfigurationID != nil:
     HTTP POST {slaBaseURL}/api/v2/sla-configurations/{id}/assign/{product_id}
  4. Publish NATS: product.created
```

## Bước 5: Thêm handlers + routes trong finding-service

**File:** `internal/delivery/http/product_handler.go` (hoặc tương đương)

```go
// Thêm 4 handlers:

// POST /api/v2/product-types/bulk
func (h *Handler) BulkCreateProductTypes(w http.ResponseWriter, r *http.Request)

// POST /api/v2/products/bulk  
func (h *Handler) BulkCreateProducts(w http.ResponseWriter, r *http.Request)

// POST /api/v2/products/{id}/seed
func (h *Handler) SeedProduct(w http.ResponseWriter, r *http.Request)

// POST /api/v2/products/import (multipart)
func (h *Handler) ImportProducts(w http.ResponseWriter, r *http.Request) {
    r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB
    r.ParseMultipartForm(10 << 20)
    format := r.FormValue("format") // "json" | "csv"
    file, _, _ := r.FormFile("file")
    defer file.Close()
    result, _ := h.productSeedUC.ImportProducts(r.Context(), file, format)
    writeJSON(w, 200, result)
}
```

**Route registration** (literal trước wildcard):
```go
r.Post("/api/v2/product-types/bulk", h.BulkCreateProductTypes)
r.Post("/api/v2/products/bulk",      h.BulkCreateProducts)    // TRƯỚC /{id}
r.Post("/api/v2/products/import",    h.ImportProducts)         // TRƯỚC /{id}
r.Post("/api/v2/products/{id}/seed", h.SeedProduct)
```

## Bước 6: CSV Parser

**File:** `internal/usecase/product_seed/csv_parser.go` (NEW)

```go
// Header: name,product_type_name,business_criticality,platform,lifecycle,tags
func ParseProductCSV(r io.Reader) ([]entity.ProductCreateInput, []error)
```

## Bước 7: Implement PostgreSQL methods

**File:** `internal/infra/postgres/product_repo.go`

```go
func (r *productRepo) CreateBulk(ctx context.Context, inputs []entity.ProductCreateInput) ([]entity.BulkCreateResult, error) {
    // Transaction với INSERT ... ON CONFLICT DO NOTHING
}

func (r *productRepo) FindByName(ctx context.Context, name string) (*entity.Product, error) {
    // SELECT * FROM products WHERE name = $1
}
```

## Bước 8: Gateway routes

**File:** `apps/osv/internal/gateway/router.go`

```go
// SEED-002: Product Hierarchy Seed (literal trước wildcard)
mux.Handle("POST /api/v2/product-types/bulk",
    protected(ratelimit.Wrap(proxy.Forward("finding-service:8085"), 5)))
mux.Handle("POST /api/v2/products/bulk",
    protected(ratelimit.Wrap(proxy.Forward("finding-service:8085"), 5)))
mux.Handle("POST /api/v2/products/import",
    adminOnly(ratelimit.Wrap(proxy.Forward("finding-service:8085"), 2)))
mux.Handle("POST /api/v2/products/{id}/seed",
    protected(proxy.Forward("finding-service:8085")))
```

## Acceptance Criteria

- [x] `POST /api/v2/product-types/bulk` với 3 types → `207 {"created_count": 3}`
- [x] `POST /api/v2/products/bulk` với `product_type_name` chưa tồn tại → auto-tạo ProductType
- [x] `POST /api/v2/products/{id}/seed` → `201` với `engagement.id` + `test.id`
- [x] `POST /api/v2/products/{id}/seed` không có test → `201 {"test": null}`
- [x] `POST /api/v2/products/import` JSON file 5 products → `200 {"imported_count": 5}`
- [x] Route ordering không conflict (literal BEFORE wildcard)
- [x] `go build ./internal/usecase/product/... ./internal/delivery/http/...` thành công
