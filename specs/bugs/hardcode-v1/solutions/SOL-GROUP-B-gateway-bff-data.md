# SOL-GROUP-B — Gateway BFF & Data Service Hardcoded Stubs

> **Fixes**: BUG-002, BUG-011  
> **Services**: `gateway-service` (BFF handlers), `data-service` (KEV & EPSS stats)  
> **Priority**: 🟡 Medium — fix trước release để tránh UI hiển thị data sai

---

## BUG-002 — Gateway: Hardcoded ProductTypes và AdminRoles Responses

### Root Cause

`UIAPIHandler.ProductTypes()` và `UIAPIHandler.AdminRoles()` trả về JSON tĩnh hardcode
thay vì proxy về service upstream. Khi thêm role/product type mới, phải sửa code và
redeploy gateway.

### Files Cần Sửa

- `services/gateway-service/internal/bff/handlers/handler_ui_api.go`

### Solution

**Bước 1**: Thêm service URL fields vào `UIAPIHandler`:

```go
// services/gateway-service/internal/bff/handlers/handler_ui_api.go

type UIAPIHandler struct {
    // ... existing fields ...
    productServiceURL  string  // [ADD]
    identityServiceURL string  // [ADD] (có thể đã có)
    proxy              *proxy.ReverseProxy
}

// UIAPIHandlerConfig — config struct cho UIAPIHandler
type UIAPIHandlerConfig struct {
    // ... existing fields ...
    ProductServiceURL  string
    IdentityServiceURL string
}

func NewUIAPIHandler(ctx context.Context, cfg UIAPIHandlerConfig) *UIAPIHandler {
    return &UIAPIHandler{
        // ...
        productServiceURL:  cfg.ProductServiceURL,
        identityServiceURL: cfg.IdentityServiceURL,
    }
}
```

**Bước 2**: Sửa `ProductTypes` — proxy đến `product-service`:

```go
// [FIX] Thay vì trả về hardcoded JSON:
//
// func (h *UIAPIHandler) ProductTypes(w http.ResponseWriter, r *http.Request) {
//     respondJSON(w, http.StatusOK, map[string]interface{}{
//         "types": []map[string]string{
//             {"value": "web_app", "label": "Web Application"},
//             ...
//         },
//     })
// }

// ProductTypes proxies GET /api/v1/products/types → product-service.
// product-service là source of truth cho các loại sản phẩm được hỗ trợ.
func (h *UIAPIHandler) ProductTypes(w http.ResponseWriter, r *http.Request) {
    target := h.productServiceURL + "/api/v1/products/types"
    h.proxy.ForwardTo(target)(w, r)
}
```

**Bước 3**: Sửa `AdminRoles` — proxy đến `identity-service`:

```go
// [FIX] Thay vì trả về hardcoded JSON:
//
// func (h *UIAPIHandler) AdminRoles(w http.ResponseWriter, r *http.Request) {
//     respondJSON(w, http.StatusOK, map[string]interface{}{
//         "roles": []map[string]interface{}{
//             {"value": "admin", "label": "Administrator", ...},
//             ...
//         },
//     })
// }

// AdminRoles proxies GET /api/v1/admin/roles → identity-service.
// identity-service/admin_handler.go:L262-282 là source of truth cho role definitions.
func (h *UIAPIHandler) AdminRoles(w http.ResponseWriter, r *http.Request) {
    target := h.identityServiceURL + "/api/v1/admin/roles"
    h.proxy.ForwardTo(target)(w, r)
}
```

### Thêm Endpoint vào `product-service`

`product-service` cần expose endpoint `GET /api/v1/products/types`:

```go
// services/product-service/internal/delivery/http/handlers.go

// ProductTypeValue là enum các loại product được hỗ trợ.
// Source of truth — được đọc từ config hoặc database.
type ProductTypeValue struct {
    Value string `json:"value"`
    Label string `json:"label"`
}

// GetProductTypes trả về danh sách product types được hỗ trợ.
// Trong tương lai có thể load từ database nếu cần extensibility.
func (h *ProductHandler) GetProductTypes(w http.ResponseWriter, r *http.Request) {
    // Hiện tại: load từ config hoặc constants — không hardcode trong gateway
    types := h.cfg.ProductTypes  // từ config file hoặc env

    // Fallback về defaults nếu config rỗng
    if len(types) == 0 {
        types = []ProductTypeValue{
            {Value: "web_app",        Label: "Web Application"},
            {Value: "api",            Label: "API"},
            {Value: "infrastructure", Label: "Infrastructure"},
            {Value: "mobile",         Label: "Mobile"},
        }
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "types": types,
    })
}
```

### Thêm Endpoint vào `identity-service`

`identity-service` đã có logic tại `admin_handler.go:L262-282`.
Chỉ cần expose endpoint `GET /api/v1/admin/roles`:

```go
// services/identity-service/adapter/handler/http/admin_handler.go

// GetRoles trả về danh sách role definitions — source of truth duy nhất.
// Route: GET /api/v1/admin/roles
func (h *AdminHandler) GetRoles(w http.ResponseWriter, r *http.Request) {
    // Role definitions từ domain constants — không hardcode trong gateway hay frontend
    roles := []map[string]interface{}{
        {"value": "admin",    "label": "Administrator",   "description": "Full system access"},
        {"value": "user",     "label": "Security Analyst","description": "Can scan and manage findings"},
        {"value": "readonly", "label": "Read-Only",       "description": "View-only access"},
        {"value": "agent",    "label": "Agent",           "description": "Automated scanning agent"},
    }
    respondJSON(w, http.StatusOK, map[string]interface{}{"roles": roles})
}

// Đăng ký route trong router setup:
// router.GET("/api/v1/admin/roles", adminHandler.GetRoles)
```

### Diagram: Before vs After

```
BEFORE (BUG-002):
  Client → Gateway (hardcoded JSON) ✗ — không thể extend

AFTER (FIX):
  Client → Gateway → product-service → GET /api/v1/products/types ✓
  Client → Gateway → identity-service → GET /api/v1/admin/roles   ✓
```

---

## BUG-011 — Data Service: Hardcoded Empty Stats (KEV & EPSS)

### Root Cause

Hai endpoint trả về `[]` thay vì query database vì repository methods chưa được implement.
Client nhận HTTP 200 với empty data — không biết tính năng chưa hoàn thiện.

### Files Cần Sửa

- `services/data-service/internal/delivery/http/kev_handler.go`
- `services/data-service/internal/delivery/http/epss_handler.go`
- `services/data-service/internal/infra/persistence/postgres/kev_repo.go`
- Thêm migration SQL (nếu cần thêm columns)

### Solution

#### Option A (Immediate Fix): Omit unimplemented fields + trả về metadata

Áp dụng ngay mà không cần implement full repo methods:

```go
// kev_handler.go — GetKEVStats

func (h *KEVHandler) GetKEVStats(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    stats, err := h.repo.GetStats(ctx)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }

    // [FIX] Bỏ by_vendor và recent_additions cho đến khi implement.
    // Thêm "_partial": true để client biết response chưa đầy đủ.
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "total":       stats.Total,
        "by_severity": stats.BySeverity,
        // "by_vendor":        REMOVED — will be added when GetStatsByVendor is implemented
        // "recent_additions": REMOVED — will be added when GetRecentAdditions is implemented
        "_meta": map[string]interface{}{
            "partial":              true,
            "unimplemented_fields": []string{"by_vendor", "recent_additions"},
        },
    })
}
```

```go
// epss_handler.go — GetEPSSByCVE

func (h *EPSSHandler) GetEPSSByCVE(w http.ResponseWriter, r *http.Request) {
    // ...
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "cve_id":     cveID,
        "score":      score.Score,
        "percentile": score.Percentile,
        // "history": REMOVED — will be added when GetHistory is implemented
        "_meta": map[string]interface{}{
            "partial":              true,
            "unimplemented_fields": []string{"history"},
        },
    })
}
```

#### Option B (Full Fix): Implement repository methods

**Step 1**: Định nghĩa interface methods trong repository:

```go
// services/data-service/internal/domain/repository/kev_repository.go

type KEVRepository interface {
    // ... existing methods ...
    
    // [ADD] GetStatsByVendor trả về top N vendors có nhiều KEV entries nhất.
    GetStatsByVendor(ctx context.Context, limit int) ([]VendorStat, error)
    
    // [ADD] GetRecentAdditions trả về N entries được thêm gần đây nhất.
    GetRecentAdditions(ctx context.Context, n int) ([]KEVEntry, error)
}

type EPSSRepository interface {
    // ... existing methods ...
    
    // [ADD] GetHistory trả về lịch sử EPSS score của 1 CVE trong N ngày gần đây.
    // Yêu cầu bảng epss_history hoặc tương đương trong DB.
    GetHistory(ctx context.Context, cveID string, days int) ([]EPSSPoint, error)
}

// Domain types
type VendorStat struct {
    Vendor string `json:"vendor"`
    Count  int    `json:"count"`
}

type EPSSPoint struct {
    Date  time.Time `json:"date"`
    Score float64   `json:"score"`
}
```

**Step 2**: Implement trong PostgreSQL repository:

```go
// services/data-service/internal/infra/persistence/postgres/kev_repo.go

// GetStatsByVendor query top N vendors với KEV entries nhiều nhất.
func (r *PostgresKEVRepository) GetStatsByVendor(ctx context.Context, limit int) ([]domain.VendorStat, error) {
    if limit <= 0 {
        limit = 10
    }
    
    // Giả sử kev_entries join với cves để lấy vendor
    const q = `
        SELECT c.vendor, COUNT(*) AS count
        FROM kev_entries ke
        JOIN cves c ON c.cve_id = ke.cve_id
        WHERE c.vendor IS NOT NULL AND c.vendor != ''
        GROUP BY c.vendor
        ORDER BY count DESC
        LIMIT $1
    `
    rows, err := r.db.Query(ctx, q, limit)
    if err != nil {
        return nil, fmt.Errorf("GetStatsByVendor: %w", err)
    }
    defer rows.Close()
    
    var results []domain.VendorStat
    for rows.Next() {
        var s domain.VendorStat
        if err := rows.Scan(&s.Vendor, &s.Count); err != nil {
            return nil, err
        }
        results = append(results, s)
    }
    return results, rows.Err()
}

// GetRecentAdditions trả về N KEV entries được thêm gần đây nhất.
func (r *PostgresKEVRepository) GetRecentAdditions(ctx context.Context, n int) ([]domain.KEVEntry, error) {
    if n <= 0 {
        n = 10
    }
    
    const q = `
        SELECT ke.cve_id, ke.date_added, ke.short_description, ke.required_action
        FROM kev_entries ke
        ORDER BY ke.date_added DESC
        LIMIT $1
    `
    // ... scan rows ...
}
```

**Step 3**: Cập nhật handler dùng methods mới:

```go
// kev_handler.go — GetKEVStats (full implementation)

func (h *KEVHandler) GetKEVStats(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    stats, err := h.repo.GetStats(ctx)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    byVendor, err := h.repo.GetStatsByVendor(ctx, 10)
    if err != nil {
        log.Warn().Err(err).Msg("GetStatsByVendor failed, using empty list")
        byVendor = []domain.VendorStat{}
    }
    
    recentAdditions, err := h.repo.GetRecentAdditions(ctx, 10)
    if err != nil {
        log.Warn().Err(err).Msg("GetRecentAdditions failed, using empty list")
        recentAdditions = []domain.KEVEntry{}
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "total":            stats.Total,
        "by_severity":      stats.BySeverity,
        "by_vendor":        byVendor,        // [FIX] real data now
        "recent_additions": recentAdditions, // [FIX] real data now
    })
}
```

#### EPSS History — Migration SQL (nếu chưa có bảng)

```sql
-- Migration: thêm bảng epss_history để lưu lịch sử EPSS score
CREATE TABLE IF NOT EXISTS epss_history (
    id         SERIAL PRIMARY KEY,
    cve_id     VARCHAR(20) NOT NULL REFERENCES cves(cve_id) ON DELETE CASCADE,
    score      NUMERIC(6,5) NOT NULL,
    percentile NUMERIC(6,5),
    recorded_at DATE NOT NULL DEFAULT CURRENT_DATE,
    UNIQUE (cve_id, recorded_at)  -- 1 record/ngày/CVE
);

CREATE INDEX idx_epss_history_cve_id ON epss_history(cve_id);
CREATE INDEX idx_epss_history_recorded_at ON epss_history(recorded_at);
```

EPSSSyncer sẽ insert vào `epss_history` mỗi lần chạy daily sync.

---

## Tóm Tắt Thay Đổi

| Bug | File | Thay Đổi Chính |
|-----|------|----------------|
| BUG-002 | `handler_ui_api.go` | Proxy `ProductTypes` → product-service; `AdminRoles` → identity-service |
| BUG-002 | `product-service/handlers.go` | Thêm `GET /api/v1/products/types` endpoint |
| BUG-002 | `identity-service/admin_handler.go` | Thêm `GET /api/v1/admin/roles` endpoint |
| BUG-011 | `kev_handler.go` | Option A: bỏ fields chưa implement; Option B: implement đầy đủ |
| BUG-011 | `epss_handler.go` | Tương tự kev_handler |
| BUG-011 | `kev_repo.go` | Implement `GetStatsByVendor`, `GetRecentAdditions` |
| BUG-011 | SQL migration | Thêm bảng `epss_history` |

## Recommendation

- **Ngắn hạn**: Apply Option A cho BUG-011 (omit fields) để fix ngay UI confusion.
- **Dài hạn**: Apply Option B (full implementation) + migration trong sprint tiếp theo.
- **BUG-002**: Implement product-service và identity-service endpoints trước khi gateway proxy.
