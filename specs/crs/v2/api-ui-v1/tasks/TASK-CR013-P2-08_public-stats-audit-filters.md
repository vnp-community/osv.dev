# TASK-CR013-P2-08 — Public Stats BFF (No Auth) + Audit Log Filters

**Phase:** Phase 3 — Degraded UX  
**Nguồn giải pháp:** [`solutions/SOL-013`](../solutions/SOL-013-public-stats-audit-log-params.md)  
**Ưu tiên:** 🟡 P2 — Login page thiếu live stats, audit log không search được  
**Phụ thuộc:** TASK-CR008-P1-03 phải xong (dùng scan stats internal endpoint)  
**Status:** ✅ **DONE** — 2026-06-19  

---

## Mục tiêu

1. `GET /api/v2/public/stats` — **NO Auth**, cached 5 phút, graceful degradation
2. Audit log filters: `?search=`, `?severity=`, `?date_from=`, `?date_to=`

---

## Điều tra trước khi code

```bash
# 1. Kiểm tra /api/v2/public/stats đã tồn tại chưa
curl -s https://c12.openledger.vn/api/v2/public/stats | jq .

# 2. Tìm public routes trong gateway
grep -n "public\|v2\|no-auth\|noauth" apps/osv/internal/gateway/router.go

# 3. Tìm BFF folder
ls apps/osv/internal/gateway/bff/

# 4. Kiểm tra audit log handler
grep -rn "audit-log\|AuditLog\|GetAuditLog" \
  services/audit-service/ --include="*.go" -l

# 5. Xem audit log filter hiện tại
grep -rn "search\|date_from\|date_to\|DateFrom" \
  services/audit-service/ --include="*.go" -n
```

---

## Phần A: Public Stats BFF

### Bước A1: Tạo BFF Handler

**Tìm hoặc tạo:**
```bash
ls apps/osv/internal/gateway/bff/
# Kiểm tra có public_bff.go không
cat apps/osv/internal/gateway/bff/public_bff.go 2>/dev/null || echo "NOT FOUND"
```

**File:** `apps/osv/internal/gateway/bff/public_bff.go`

```go
package bff

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "time"

    "github.com/redis/go-redis/v9"
)

type PublicBFF struct {
    scanBaseURL    string // scan-service:8084
    findingBaseURL string // finding-service:8085
    dataBaseURL    string // data-service:8082
    cache          *redis.Client
    httpClient     *http.Client
}

type PublicStats struct {
    TotalCVEs        string           `json:"total_cves"`
    ScansToday       int              `json:"scans_today"`
    FindingAccuracy  string           `json:"finding_accuracy"`
    UptimeSLA        string           `json:"uptime_sla"`
    ThreatIndicators ThreatIndicators `json:"threat_indicators"`
}

type ThreatIndicators struct {
    CriticalThreats int `json:"critical_threats"`
    KEVActive       int `json:"kev_active"`
    AssetsAtRisk    int `json:"assets_at_risk"`
}

const (
    publicStatsCacheKey = "public:stats:v1"
    publicStatsCacheTTL = 5 * time.Minute
)

// HandlePublicStats — NO authentication, CORS open
func (b *PublicBFF) HandlePublicStats(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // CORS cho login page (có thể khác domain)
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
    if r.Method == "OPTIONS" {
        w.WriteHeader(http.StatusNoContent)
        return
    }

    // Redis cache
    if b.cache != nil {
        if cached, err := b.cache.Get(ctx, publicStatsCacheKey).Bytes(); err == nil {
            w.Header().Set("Content-Type", "application/json")
            w.Header().Set("X-Cache", "HIT")
            w.Write(cached)
            return
        }
    }

    // Aggregate — graceful degradation: nếu service down → fallback values
    stats := PublicStats{
        TotalCVEs:       b.getTotalCVEs(ctx),
        ScansToday:      b.getScansToday(ctx),
        FindingAccuracy: "98.4%",  // Static hoặc từ AI metrics
        UptimeSLA:       "99.99%", // Static hoặc từ monitoring
        ThreatIndicators: ThreatIndicators{
            CriticalThreats: b.getCriticalThreats(ctx),
            KEVActive:       b.getKEVActive(ctx),
            AssetsAtRisk:    0, // Optional — nếu asset-service chưa có endpoint
        },
    }

    data, _ := json.Marshal(stats)
    if b.cache != nil {
        b.cache.Set(ctx, publicStatsCacheKey, data, publicStatsCacheTTL)
    }

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Cache", "MISS")
    w.Write(data)
}

// Internal service calls — graceful degradation

func (b *PublicBFF) getTotalCVEs(ctx context.Context) string {
    resp, err := b.httpClient.Get(b.dataBaseURL + "/api/v1/dbinfo")
    if err != nil {
        return "240K+" // Fallback nếu data-service down
    }
    defer resp.Body.Close()
    var info struct{ TotalCVEs int `json:"total_cves"` }
    if err := json.NewDecoder(resp.Body).Decode(&info); err != nil || info.TotalCVEs == 0 {
        return "240K+"
    }
    if info.TotalCVEs >= 1000 {
        return fmt.Sprintf("%dK+", info.TotalCVEs/1000)
    }
    return strconv.Itoa(info.TotalCVEs)
}

func (b *PublicBFF) getScansToday(ctx context.Context) int {
    // Dùng internal stats endpoint từ SOL-008
    resp, err := b.httpClient.Get(b.scanBaseURL + "/api/v1/scans/stats")
    if err != nil {
        return 0
    }
    defer resp.Body.Close()
    var stats struct{ CompletedToday int `json:"completed_today"` }
    if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
        return 0
    }
    return stats.CompletedToday
}

func (b *PublicBFF) getCriticalThreats(ctx context.Context) int {
    // Lấy từ finding-service — count active Critical findings
    resp, err := b.httpClient.Get(
        b.findingBaseURL + "/api/v1/findings/stats?severity=Critical&status=Active")
    if err != nil {
        return 0
    }
    defer resp.Body.Close()
    var result struct{ Total int `json:"total"` }
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Total
}

func (b *PublicBFF) getKEVActive(ctx context.Context) int {
    resp, err := b.httpClient.Get(b.dataBaseURL + "/api/v1/kev/stats")
    if err != nil {
        return 0
    }
    defer resp.Body.Close()
    var stats struct{ Active int `json:"active"` }
    json.NewDecoder(resp.Body).Decode(&stats)
    return stats.Active
}
```

### Bước A2: Register Route — NO auth middleware

**Trong gateway router:**
```bash
grep -n "public\|v2" apps/osv/internal/gateway/router.go
```

```go
// apps/osv/internal/gateway/router.go
// PUBLIC — không có protected(), không có adminOnly()
// Chỉ rate limit để prevent abuse

// Tạo PublicBFF instance
publicBFF := &bff.PublicBFF{
    scanBaseURL:    "http://scan-service:8084",
    findingBaseURL: "http://finding-service:8085",
    dataBaseURL:    "http://data-service:8082",
    cache:          redisClient, // tái dùng redis instance
    httpClient:     &http.Client{Timeout: 3 * time.Second},
}

// Đăng ký route PUBLIC
mux.Handle("GET /api/v2/public/stats",
    rateLimiter.Limit("60/minute")(http.HandlerFunc(publicBFF.HandlePublicStats)))

// Verify: OPTIONS preflight
mux.Handle("OPTIONS /api/v2/public/stats",
    http.HandlerFunc(publicBFF.HandlePublicStats))
```

---

## Phần B: Audit Log Filters

### Bước B1: Update Audit Log Handler

**Tìm handler:**
```bash
grep -rn "GetAuditLog\|ListAudit\|audit-log\|AuditLog" \
  services/audit-service/ --include="*.go" -n
```

**Thêm filter params:**

```go
// GET /api/v1/audit-log
func (h *AuditHandler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    q   := r.URL.Query()

    search   := q.Get("search")    // search trong action, actor_email, resource_type
    severity := q.Get("severity")  // "Critical"|"Warning"|"Info"
    dateFrom := q.Get("date_from") // "2026-01-01"
    dateTo   := q.Get("date_to")   // "2026-06-30"
    page     := parseIntDefault(q.Get("page"), 1)
    pageSize := parseIntDefault(q.Get("page_size"), 50)

    baseQ := `
        SELECT id, actor_id, actor_email, action, resource_type,
               resource_id, created_at
        FROM audit_events
        WHERE 1=1
    `
    args := []interface{}{}
    idx  := 1

    // Search filter (full-text trên 3 columns)
    if search != "" {
        baseQ += fmt.Sprintf(`
            AND (action ILIKE $%d
              OR actor_email ILIKE $%d
              OR resource_type ILIKE $%d)
        `, idx, idx, idx)
        args = append(args, "%"+search+"%")
        idx++
    }

    // Date range
    if dateFrom != "" {
        if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
            baseQ += fmt.Sprintf(" AND created_at >= $%d", idx)
            args = append(args, t)
            idx++
        }
    }
    if dateTo != "" {
        if t, err := time.Parse("2006-01-02", dateTo); err == nil {
            // Inclusive: đến hết ngày dateTo
            baseQ += fmt.Sprintf(" AND created_at < $%d", idx)
            args = append(args, t.Add(24*time.Hour))
            idx++
        }
    }

    // Severity filter — map sang action patterns
    // Approach đơn giản: severity column trong audit_events
    // Nếu chưa có column, skip hoặc thêm migration
    if severity != "" {
        // Kiểm tra xem audit_events có severity column không
        // Option A: có column severity
        baseQ += fmt.Sprintf(" AND severity = $%d", idx)
        args = append(args, severity)
        idx++
        // Option B: không có column → filter bằng action prefix
        // Ví dụ: "Critical" → action IN ('user.locked', 'data.deleted', ...)
    }

    // Count
    var total int
    // ... count query ...

    // Paginate
    baseQ += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
    args = append(args, pageSize, (page-1)*pageSize)

    // Execute và scan rows
    // ...

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "events": events,
        "total":  total,
    })
}
```

> **Lưu ý về severity:** Nếu `audit_events` chưa có `severity` column:
> ```sql
> ALTER TABLE audit_events
>     ADD COLUMN IF NOT EXISTS severity VARCHAR(20) NOT NULL DEFAULT 'Info'
>         CHECK (severity IN ('Info', 'Warning', 'Critical'));
> ```
> Sau đó populate logic trong audit-service khi insert event.

---

## Acceptance Criteria

### Public Stats
- [ ] `GET /api/v2/public/stats` → HTTP 200 **không cần Authorization header**
- [ ] Response có: `total_cves`, `scans_today`, `finding_accuracy`, `uptime_sla`, `threat_indicators`
- [ ] `threat_indicators` có: `critical_threats`, `kev_active`, `assets_at_risk`
- [ ] Nếu scan-service down → HTTP 200 với `scans_today: 0` (không crash)
- [ ] 2nd request trong 5 phút → `X-Cache: HIT`
- [ ] Rate limit: 60 requests/phút

### Audit Log Filters
- [ ] `?search=login` → chỉ events có "login" trong action/actor_email/resource_type
- [ ] `?date_from=2026-06-01&date_to=2026-06-30` → chỉ events trong range
- [ ] `?severity=Critical` → chỉ Critical events (nếu column tồn tại)
- [ ] Filters có thể combine: `?search=admin&date_from=2026-06-01`
- [ ] Không có filter → trả tất cả (backward compatible)

## Verification

```bash
# PUBLIC — không cần token
curl -s https://c12.openledger.vn/api/v2/public/stats | jq .
# Expected: HTTP 200 với 5 fields

# Verify no auth needed
curl -s https://c12.openledger.vn/api/v2/public/stats \
  -o /dev/null -w "%{http_code}"
# Expected: 200 (không phải 401)

# Cache test
curl -s https://c12.openledger.vn/api/v2/public/stats -v 2>&1 | grep "X-Cache"
# 1st request: X-Cache: MISS
curl -s https://c12.openledger.vn/api/v2/public/stats -v 2>&1 | grep "X-Cache"
# 2nd request: X-Cache: HIT

# CORS headers
curl -s https://c12.openledger.vn/api/v2/public/stats -v 2>&1 | \
  grep "Access-Control"
# Expected: Access-Control-Allow-Origin: *

# Audit log search
curl -s "https://c12.openledger.vn/api/v1/audit-log?search=login" \
  -H "Authorization: Bearer $TOKEN" | jq '.total'

# Audit log date range
curl -s "https://c12.openledger.vn/api/v1/audit-log?date_from=2026-06-01&date_to=2026-06-19" \
  -H "Authorization: Bearer $TOKEN" | jq '.total'
```
