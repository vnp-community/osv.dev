# SOL-SEED-002: Giải pháp thực thi — Products Hierarchy Seed

> **CR:** [SEED-002-products-hierarchy-seed.md](../SEED-002-products-hierarchy-seed.md)  
> **Cập nhật:** 2026-06-18  
> **Domain:** `services/finding-service` (canonical) + `apps/osv` (gateway)  
> **Priority:** 🔴 CRITICAL

---

## 1. Phân tích kiến trúc hiện tại

Theo `01-architecture.md §3.5`, `finding-service` là service **canonical** quản lý toàn bộ hierarchy:
```
ProductType → Product → Engagement → Test → Finding
```

Schema DB: `osv_finding` — bảng `product_types`, `products`, `engagements`, `tests`.

`product-service` (`:8089`) là service riêng cho CI/CD orchestrator (`POST /orchestrate/cicd`), nhưng **finding-service là nơi chứa CRUD APIs** cho Product hierarchy.

**Deduplication**: Finding dùng SHA-256 hash (title + component + version + cve_id). Bulk import cần áp dụng same algorithm.

---

## 2. Các thay đổi cần thực hiện

### 2.1 Domain Layer — `services/finding-service/internal/domain/`

**File**: `internal/domain/repository/product_repository.go` — Thêm methods:

```go
type ProductTypeRepository interface {
    // ... existing ...
    CreateBulk(ctx context.Context, types []entity.ProductType) ([]entity.BulkCreateResult, error)
}

type ProductRepository interface {
    // ... existing ...
    CreateBulk(ctx context.Context, products []entity.ProductCreateInput) ([]entity.BulkCreateResult, error)
    FindByName(ctx context.Context, name string) (*entity.Product, error)
}

type EngagementRepository interface {
    // ... existing ...
    // Bulk not needed — dùng CreateEngagement trong ProductSeedUseCase
}
```

**File**: `internal/domain/entity/product.go` — Thêm types:

```go
type ProductCreateInput struct {
    Name                   string
    ProductTypeID          uuid.UUID
    ProductTypeName        string     // resolve to ID if provided
    Description            string
    BusinessCriticality    string
    Platform               string
    Lifecycle              string
    Origin                 string
    ExternalAudience       bool
    InternetAccessible     bool
    Tags                   []string
    SLAConfigurationID     *uuid.UUID // optional: assign SLA immediately
}

type BulkCreateResult struct {
    Name    string
    Status  string  // "created" | "error"
    ID      *uuid.UUID
    Message string
}

// ProductSeedInput là input cho composite seed endpoint
type ProductSeedInput struct {
    Engagement *EngagementCreateInput
    Test       *TestCreateInput
}
```

---

### 2.2 Use Case Layer — `services/finding-service/internal/usecase/`

#### Tạo usecase `product_seed`

**File**: `internal/usecase/product_seed/usecase.go`

```go
package productseed

type UseCase struct {
    productTypeRepo repository.ProductTypeRepository
    productRepo     repository.ProductRepository
    engagementRepo  repository.EngagementRepository
    testRepo        repository.TestRepository
    slaClient       sla.Client   // HTTP client → sla-service:8086
    eventPub        events.Publisher
}

// BulkCreateProductTypes tạo nhiều ProductTypes với partial failure
func (uc *UseCase) BulkCreateProductTypes(ctx context.Context, types []entity.ProductType) (BulkResult, error)

// BulkCreateProducts tạo nhiều Products, resolve ProductType name → ID nếu cần
func (uc *UseCase) BulkCreateProducts(ctx context.Context, products []entity.ProductCreateInput) (BulkResult, error)

// SeedProductHierarchy tạo Engagement + Test trong 1 transaction
// Publish NATS: product.engagement.created
func (uc *UseCase) SeedProductHierarchy(ctx context.Context, productID uuid.UUID, in entity.ProductSeedInput) (*SeedResult, error)

// ImportProducts import từ file đã parse
func (uc *UseCase) ImportProducts(ctx context.Context, items []entity.ProductCreateInput) (ImportResult, error)
```

**Logic `BulkCreateProducts`:**
```
For each ProductCreateInput:
  1. Nếu ProductTypeID không có, lookup bằng ProductTypeName
     → Nếu không tìm thấy: tạo mới ProductType (auto-create)
  2. INSERT product (ON CONFLICT (name) DO UPDATE)
  3. Nếu SLAConfigurationID != nil:
     → HTTP POST to sla-service: /api/v2/sla-configurations/{id}/assign/{product_id}
  4. Append to results
5. Publish NATS: product.batch_created{count, actor_id}
```

**Logic `SeedProductHierarchy`:**
```
1. BEGIN TRANSACTION
2. Create Engagement (status=Not Started)
3. If test input provided: Create Test linked to Engagement
4. COMMIT
5. Publish NATS: product.engagement.created{product_id, engagement_id}
```

---

### 2.3 Adapter Layer — HTTP Handlers

**File**: `internal/delivery/http/product_handler.go`

```go
// BulkCreateProductTypes handles POST /api/v2/product-types/bulk
func (h *Handler) BulkCreateProductTypes(w http.ResponseWriter, r *http.Request)

// BulkCreateProducts handles POST /api/v2/products/bulk
func (h *Handler) BulkCreateProducts(w http.ResponseWriter, r *http.Request)

// SeedProduct handles POST /api/v2/products/{id}/seed
// Composite: creates Engagement + optional Test
func (h *Handler) SeedProduct(w http.ResponseWriter, r *http.Request)

// ImportProducts handles POST /api/v2/products/import
// Multipart: file (JSON|CSV) + metadata
func (h *Handler) ImportProducts(w http.ResponseWriter, r *http.Request) {
    // Parse multipart
    file, format := parseMultipart(r)
    
    // Parse file based on format
    var items []entity.ProductCreateInput
    if format == "json" {
        items = parseJSONProducts(file)
    } else {
        items = parseCSVProducts(file)
    }
    
    // Call usecase
    result := h.productSeedUC.ImportProducts(ctx, items)
    writeJSON(w, 200, result)
}
```

**File**: `internal/delivery/http/router.go` — Thêm routes:

```go
// SEED-002: Product Hierarchy Seed endpoints
// IMPORTANT: literal paths BEFORE wildcards
r.Post("/api/v2/product-types/bulk",  h.BulkCreateProductTypes)
r.Post("/api/v2/products/bulk",       h.BulkCreateProducts)
r.Post("/api/v2/products/import",     h.ImportProducts)
r.Post("/api/v2/products/{id}/seed",  h.SeedProduct)
```

---

### 2.4 File Import — CSV Parser

**File**: `internal/usecase/product_seed/csv_parser.go`

CSV format:
```
name,product_type_name,business_criticality,platform,lifecycle,tags
"Customer Portal","Web Application","high","web","production","b2c,production"
```

Logic:
1. Đọc header row → map column name → field
2. Validate required fields: `name`, `product_type_name`
3. Parse tags: comma-separated trong double quotes
4. Return `[]entity.ProductCreateInput`

---

### 2.5 SLA Client Integration

**File**: `internal/infra/sla_client/client.go` (NEW)

```go
// SLAClient gọi sla-service để assign SLA cho product
type SLAClient struct {
    baseURL    string
    httpClient *http.Client
}

func (c *SLAClient) AssignSLA(ctx context.Context, slaConfigID, productID uuid.UUID) error {
    url := fmt.Sprintf("%s/api/v2/sla-configurations/%s/assign/%s", c.baseURL, slaConfigID, productID)
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
    // Internal service-to-service call (no auth header needed — internal network)
    resp, err := c.httpClient.Do(req)
    // handle response
}
```

---

### 2.6 Gateway Layer — `apps/osv/internal/gateway/router.go`

```go
// ═══════════════════════════════════════════════
// PRODUCT HIERARCHY SEED (SEED-002)
// ═══════════════════════════════════════════════
// Rate limits: bulk = 5/min, import = 2/min
mux.Handle("POST /api/v2/product-types/bulk",
    protected(ratelimit.Wrap(proxy.Forward("finding-service:8085"), 5)))

mux.Handle("POST /api/v2/products/bulk",
    protected(ratelimit.Wrap(proxy.Forward("finding-service:8085"), 5)))

mux.Handle("POST /api/v2/products/import",
    adminOnly(ratelimit.Wrap(proxy.Forward("finding-service:8085"), 2)))

// IMPORTANT: /products/bulk, /products/import phải TRƯỚC /products/{id}
mux.Handle("POST /api/v2/products/{id}/seed",
    protected(proxy.Forward("finding-service:8085")))
```

> ⚠️ **Route ordering**: Trong Go `net/http` ServeMux, literal segments được ưu tiên hơn wildcards tự động. Tuy nhiên, với `chi` router trong finding-service, cần đăng ký literal routes trước.

---

### 2.7 Database — Không cần migration mới

Các bảng `product_types`, `products`, `engagements`, `tests` đã tồn tại trong schema `osv_finding`. Chỉ cần:
- Đảm bảo `INSERT ... ON CONFLICT (name, product_type_id) DO NOTHING` cho idempotency
- Index `products(name)` đã có hoặc cần thêm cho `FindByName`

---

## 3. NATS Events mới

| Subject | Publisher | Consumers | Payload |
|---------|-----------|----------|---------|
| `product.batch_created` | finding-service | audit-service | `{count, product_ids[], actor_id}` |
| `product.engagement.created` | finding-service | audit-service | `{product_id, engagement_id}` |

---

## 4. File thay đổi tổng hợp

| File | Thay đổi |
|------|---------|
| `internal/domain/repository/product_repository.go` | Thêm `CreateBulk`, `FindByName` |
| `internal/domain/entity/product.go` | Thêm `ProductCreateInput`, `BulkCreateResult`, `ProductSeedInput` |
| `internal/usecase/product_seed/usecase.go` | **[NEW]** |
| `internal/usecase/product_seed/csv_parser.go` | **[NEW]** |
| `internal/delivery/http/product_handler.go` | Thêm 4 handlers |
| `internal/delivery/http/router.go` | Thêm 4 routes (literal trước wildcard) |
| `internal/infra/postgres/product_repo.go` | Implement `CreateBulk`, `FindByName` |
| `internal/infra/sla_client/client.go` | **[NEW]** HTTP client cho sla-service |
| `apps/osv/internal/gateway/router.go` | Thêm 4 gateway routes |

---

## 5. Acceptance Criteria

1. `POST /api/v2/product-types/bulk` với 3 types → `207 {"created_count": 3}`.
2. `POST /api/v2/products/bulk` với `product_type_name` không tồn tại → tự động tạo ProductType.
3. `POST /api/v2/products/{id}/seed` với engagement + test → `201` với cả 2 IDs.
4. `POST /api/v2/products/{id}/seed` chỉ với engagement (không test) → `201 {"test": null}`.
5. `POST /api/v2/products/import` với JSON file 10 products → `200 {"imported_count": 10}`.
6. `POST /api/v2/products/import` với CSV file → parse đúng, tạo thành công.
7. `POST /api/v2/products` với `sla_configuration_id` → product được assign SLA ngay lập tức.
8. `POST /api/v2/products/bulk` đứng trước `POST /api/v2/products/{id}/seed` trong gateway — không bị routing conflict.
