> **✅ COMPLETED** — Implemented via Bridge Pattern. `go build && go vet` passed.

# T10 — Product Service Wiring (Asset & Tag Management)

## Thông tin
| | |
|---|---|
| **Phase** | 4 — Assets |
| **Ước tính** | 3–4 giờ |
| **Depends on** | T08 |
| **Blocks** | T12 (report cần product/engagement) |

---

## Packages cần import

| Import path | Thành phần |
|-------------|------------|
| `product-service/internal/delivery/http/` | HTTP handler — mount nguyên |
| `product-service/internal/usecase/product/` | Product CRUD |
| `product-service/internal/usecase/engagement/` | Engagement CRUD |
| `product-service/internal/usecase/test/` | Test CRUD |
| `product-service/internal/infra/` | Repositories |
| `scan-service/internal/usecase/asset/upsert_asset/` | Asset upsert (sau scan) |

---

## Các bước thực hiện

### 10.1 Đọc product-service API

```bash
cat osv.dev/services/product-service/internal/delivery/http/*.go
cat osv.dev/services/product-service/internal/usecase/product/*.go
cat osv.dev/services/product-service/internal/usecase/engagement/*.go
cat osv.dev/services/product-service/internal/infra/*.go | head -50
```

### 10.2 Khởi tạo product repositories

```go
import (
    productrepo "github.com/osv/product-service/internal/infra"
)

productRepo    := productrepo.NewProductRepository(a.db)
engagementRepo := productrepo.NewEngagementRepository(a.db)
testRepo       := productrepo.NewTestRepository(a.db)
```

### 10.3 Khởi tạo product usecases

```go
import (
    productuc    "github.com/osv/product-service/internal/usecase/product"
    engagementuc "github.com/osv/product-service/internal/usecase/engagement"
)

productUC    := productuc.New(productRepo, a.nc)
engagementUC := engagementuc.New(engagementRepo, productRepo, a.nc)
```

### 10.4 Mount product HTTP handler

```go
import (
    producthttp "github.com/osv/product-service/internal/delivery/http"
)

productHandler := producthttp.New(productUC, engagementUC, a.log)

// Trong router protected group:
r.Mount("/api/v1", producthttp.NewRouter(productHandler, a.log))
```

**Routes kỳ vọng:**
```
GET    /api/v1/products
GET    /api/v1/products/{id}
POST   /api/v1/products
PUT    /api/v1/products/{id}
DELETE /api/v1/products/{id}
POST   /api/v1/products/{id}/tags
DELETE /api/v1/products/{id}/tags/{tag}
GET    /api/v1/tags
POST   /api/v1/engagements
GET    /api/v1/engagements/{id}
```

### 10.5 Auto-upsert asset sau scan hoàn thành

```go
import (
    assetupsert "github.com/osv/scan-service/internal/usecase/asset/upsert_asset"
)

assetUpsertUC := assetupsert.New(productRepo, a.log)

// Trong scan.completed NATS subscriber:
// Sau khi tạo findings, upsert assets:
scan, _ := a.ScanRepo.FindByID(ctx, scanID)
for _, target := range scan.Targets {
    assetUpsertUC.Execute(ctx, assetupsert.Input{
        Hostname:        target.Hostname,
        IP:              target.IP,
        VulnerabilityCount: target.FindingCount,
    })
}
```

### 10.6 Cập nhật App struct

```go
type App struct {
    // ... existing fields

    // Product service
    ProductRepo     product.Repository
    ProductUC       *productuc.UseCase
    EngagementUC    *engagementuc.UseCase
    ProductHandler  *producthttp.Handler
    AssetUpsertUC   *assetupsert.UseCase
}
```

---

## Output

- [x] Product repositories khởi tạo ✓ (productBridge với pgxpool.Pool — direct SQL)
- [x] Product, Engagement usecases khởi tạo ✓ (GetOrCreateProduct, GetOrCreateEngagement, GetOrCreateTest)
- [x] Product HTTP handler mounted ✓ (/api/v1/products, engagements, tests)
- [x] Asset upsert sau mỗi scan hoàn thành ✓ (NATS scan.completed → upsertAssetFromScan)

## Acceptance Criteria

```bash
TOKEN=<token>

# Tạo product/asset
curl -X POST http://localhost:8080/api/v1/products \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"My Web Server","type":"web","url":"https://example.com"}'
# → {"id":"...","name":"My Web Server"}

# List products (assets)
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/products
# → {"products":[...],"total":N}

# Sau scan, asset tự được tạo/cập nhật
# Chạy scan vào 127.0.0.1 rồi check products
# → 127.0.0.1 xuất hiện trong products với vuln_count

# Tag
curl -X POST http://localhost:8080/api/v1/products/{id}/tags \
  -d '{"tag":"production"}'
# → 200 OK
```
