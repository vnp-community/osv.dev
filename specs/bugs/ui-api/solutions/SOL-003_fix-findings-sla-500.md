# SOL-003 — Fix Findings 500 & SLA Dashboard 500 (BUG-BE-003 + BUG-BE-004)

| Trường | Giá trị |
|---|---|
| **Bugs** | [BUG-BE-003](../BUG-BE-003_findings-500.md), [BUG-BE-004](../BUG-BE-004_sla-endpoints-broken.md) |
| **Services** | `services/finding-service` (:8085), `services/sla-service` (:8086) |
| **Gateway BFF** | `apps/osv/internal/gateway/bff/dashboard.go` |
| **Priority** | P0 — Blocking |
| **Estimated effort** | 4–6h |
| **Kiến trúc** | [architecture.md §3.5](../../../01-architecture.md), [§3.7](../../../01-architecture.md) |

---

## Root Cause: Findings 500

Lỗi `{"error":"failed to list findings"}` từ `finding-service:8085` khi gọi `GET /api/v1/findings`.

Routes trong `apps/osv/internal/gateway/router.go`:
```go
// Line 122-131 — đã có
mux.Handle("GET /api/v1/findings", protected(proxy.Forward("finding-service:8085")))
mux.Handle("GET /api/v1/findings/stats", protected(proxy.Forward("finding-service:8085")))
```

**Routes đúng** → vấn đề trong `finding-service` handler hoặc database.

Nguyên nhân có thể:
1. SQL query lỗi — JOIN với bảng `engagements` hoặc `tests` (theo hierarchy `Product→Engagement→Test→Finding`)
2. Column name không khớp sau migration
3. `page_size` param không được parse (handler expect `pageSize` camelCase thay vì `page_size`)

---

## Root Cause: Dashboard SLA 500

Từ `apps/osv/internal/gateway/bff/dashboard.go` line 98:
```go
func (bff *DashboardBFF) HandleDashboardSLA(w http.ResponseWriter, r *http.Request) {
    // ...
    err := bff.client.get(ctx, "http://"+bff.findingSvcAddr+"/internal/sla-dashboard", &result)
    if err != nil {
        http.Error(w, `{"error":"SERVICE_UNAVAILABLE",...}`, 503)
        return
    }
    // ...
}
```

BFF gọi `finding-service:8085/internal/sla-dashboard` → **endpoint này không tồn tại** hoặc finding-service trả về response format khác JSON mong đợi → parse lỗi → 500.

Thực tế BFF trả 500 vì message là `"Failed to parse SLA response"` — không phải 503 — nghĩa là request đến được nhưng decode JSON thất bại.

---

## Giải Pháp A — Fix Findings 500

### A1. Kiểm tra và fix SQL query

```bash
# Kiểm tra logs finding-service:
docker logs osv-backend-finding-service-1 --tail 200 | grep -i "error\|finding\|sql"

# Kiểm tra bảng findings có data không:
docker exec osv-backend-postgres-1 psql -U osv -d osv -c "SELECT COUNT(*) FROM findings;"

# Kiểm tra migration đã chạy đủ:
docker exec osv-backend-postgres-1 psql -U osv -d osv -c "\dt"
```

### A2. Fix query param parsing

Theo spec: `GET /api/v1/findings?page=1&page_size=10` (dùng `page_size` không phải `pageSize`).

Trong finding-service handler:
```go
// Kiểm tra handler có parse đúng không:
// TRƯỚC (chỉ nhận camelCase):
pageSize := r.URL.Query().Get("pageSize")

// SAU (nhận cả hai):
pageSize := r.URL.Query().Get("page_size")
if pageSize == "" {
    pageSize = r.URL.Query().Get("pageSize") // backward compat
}
```

### A3. Fix SQL query (nếu cần)

```go
// services/finding-service/internal/infra/postgres/finding_repo.go
// Đảm bảo LIST query dùng đúng column names và không JOIN bảng thiếu:
func (r *PostgresFindingRepository) List(ctx context.Context, filter FindingFilter) ([]Finding, int, error) {
    query := `
        SELECT f.id, f.title, f.severity, f.state, f.cve_id, 
               f.component_name, f.sla_expiry, f.created_at
        FROM findings f
        -- Bỏ JOIN không cần thiết nếu product_id là optional
        WHERE ($1 = '' OR f.status = $1)
          AND ($2 = '' OR f.severity = $2)
        ORDER BY f.created_at DESC
        LIMIT $3 OFFSET $4
    `
    // ...
}
```

---

## Giải Pháp B — Fix Dashboard SLA 500

### B1. Fix `HandleDashboardSLA` trong BFF

File: [`apps/osv/internal/gateway/bff/dashboard.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/bff/dashboard.go)

**Vấn đề**: BFF gọi `finding-service:8085/internal/sla-dashboard` nhưng endpoint này có thể:
- Không tồn tại → connection timeout
- Trả về format khác map[string]interface{} kỳ vọng

**Fix A — Proxy thẳng đến sla-service** (đúng theo arch §3.7):

```go
// apps/osv/internal/gateway/bff/dashboard.go
func (bff *DashboardBFF) HandleDashboardSLA(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    // Gọi sla-service trực tiếp thay vì finding-service
    // sla-service có endpoint /api/v2/sla-dashboard
    var result map[string]interface{}
    err := bff.client.get(ctx, "http://sla-service:8086/api/v2/sla-dashboard", &result)
    if err != nil {
        log.Error().Err(err).Msg("failed to fetch SLA dashboard from sla-service")
        // Trả về dữ liệu empty thay vì 500
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "summary": map[string]interface{}{
                "total_active_findings": 0,
                "compliance_percent":    100.0,
                "breached":              0,
                "at_risk":               0,
                "ok":                    0,
            },
            "compliance_trend":  []interface{}{},
            "breached_findings": []interface{}{},
            "at_risk_findings":  []interface{}{},
            "by_product":        []interface{}{},
            "total_breached":    0,
            "total_at_risk":     0,
            "page":              1,
            "page_size":         20,
        })
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}
```

**Fix B — Thêm sla-service address vào DashboardBFF**:

```go
// apps/osv/internal/gateway/bff/dashboard.go
type DashboardBFF struct {
    client         *InternalClient
    redis          *redis.Client
    findingSvcAddr string
    dataSvcAddr    string
    slaSvcAddr     string // THÊM MỚI
}

func NewDashboardBFF(
    redis *redis.Client,
    findingSvcAddr, dataSvcAddr, slaSvcAddr string, // THÊM slaSvcAddr
) *DashboardBFF {
    return &DashboardBFF{
        client:         NewInternalClient(),
        redis:          redis,
        findingSvcAddr: findingSvcAddr,
        dataSvcAddr:    dataSvcAddr,
        slaSvcAddr:     slaSvcAddr, // THÊM
    }
}
```

Cập nhật `router.go` line 41:
```go
// apps/osv/internal/gateway/router.go — line 41
// TRƯỚC:
dashBFF := bff.NewDashboardBFF(redisClient, "finding-service:8085", "data-service:8082")

// SAU:
dashBFF := bff.NewDashboardBFF(redisClient, "finding-service:8085", "data-service:8082", "sla-service:8086")
```

### B2. Fix internal/sla-dashboard endpoint trong finding-service

Nếu muốn giữ nguyên BFF gọi finding-service, cần thêm internal endpoint:

```go
// services/finding-service/internal/delivery/http/ — thêm route mới
// GET /internal/sla-dashboard
func (h *Handler) InternalSLADashboard(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Đếm findings theo trạng thái SLA
    breached, _ := h.findingRepo.CountByCondition(ctx, "state = 'Active' AND sla_expiry < NOW()")
    atRisk, _ := h.findingRepo.CountByCondition(ctx, "state = 'Active' AND sla_expiry BETWEEN NOW() AND NOW() + INTERVAL '7 days'")
    total, _ := h.findingRepo.CountByCondition(ctx, "state = 'Active'")
    ok := total - breached - atRisk
    
    var compliance float64
    if total > 0 {
        compliance = float64(ok) / float64(total) * 100
    } else {
        compliance = 100.0
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "summary": map[string]interface{}{
            "total_active_findings": total,
            "compliance_percent":    compliance,
            "breached":              breached,
            "at_risk":               atRisk,
            "ok":                    ok,
        },
        "compliance_trend":  []interface{}{},
        "breached_findings": []interface{}{},
        "at_risk_findings":  []interface{}{},
        "by_product":        []interface{}{},
        "total_breached":    breached,
        "total_at_risk":     atRisk,
        "page":              1,
        "page_size":         20,
    })
}
```

---

## Giải Pháp C — Fix SLA Overview 404

`GET /api/v1/sla/overview` đã có route trong `router.go` line 276:
```go
mux.Handle("GET /api/v1/sla/overview", protected(http.HandlerFunc(slaOverviewBFF(proxy))))
```

`slaOverviewBFF` tại line 550 rewrite đến `sla-service:8086/api/v2/sla-dashboard`.

**Fix**: Đảm bảo sla-service route `/api/v2/sla-dashboard` trả đúng JSON:

```bash
# Kiểm tra sla-service:
curl http://localhost:8086/api/v2/sla-dashboard
# Nếu 404 → sla-service chưa đăng ký route này
```

Thêm route trong sla-service:
```go
// services/sla-service/internal/delivery/http/ 
r.Get("/api/v2/sla-dashboard", h.GetSLADashboard)
```

---

## Fix SLA Config Schema (BUG-BE-004 phần 2)

`GET /api/v1/sla/config` trả 200 nhưng thiếu `global` và `product_overrides`.

Theo `technical-design.md §5.3`:
```go
type SLAConfig struct {
    CriticalDays int
    HighDays     int
    MediumDays   int
    LowDays      int
}
```

Cần wrap response đúng spec:
```go
// services/sla-service/internal/delivery/http/sla_handler.go
func (h *SLAHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
    global, _ := h.configRepo.GetGlobal(r.Context())
    overrides, _ := h.configRepo.GetProductOverrides(r.Context())

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "global": map[string]interface{}{
            "critical_days": global.CriticalDays, // 7
            "high_days":     global.HighDays,     // 30
            "medium_days":   global.MediumDays,   // 90
            "low_days":      global.LowDays,      // 180
        },
        "product_overrides": overrides, // [] nếu chưa có
    })
}
```

---

## Xác Nhận Fix

```bash
# Findings
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/findings?page=1&page_size=10"
# Expected: HTTP 200

# SLA Dashboard
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/dashboard/sla
# Expected: HTTP 200 với summary schema

# SLA Overview  
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/sla/overview
# Expected: HTTP 200

# SLA Config
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/sla/config
# Expected: HTTP 200 { "global": {...}, "product_overrides": [] }
```
