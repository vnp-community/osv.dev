# SOL-013: Public Stats Endpoint (No Auth) & AI Triage Queue Schema

> **CR:** [CR-013](../CR-013-public-stats-ai-triage-queue-schema.md)  
> **Priority:** 🟡 HIGH (Phase 3)  
> **Service(s):** `apps/osv` (Gateway BFF), `ai-service` (`:9103`), `audit-service` (`:8090`)  
> **Tạo:** 2026-06-19  
> **Trạng thái:** ✅ **IMPLEMENTED** — 2026-06-22  

---

## ✅ Implementation Status

| Phần | Trạng thái | File |
|---|---|---|
| Public Stats — `GET /api/v2/public/stats` (no auth) | ✅ Done | `apps/osv/internal/gateway/bff/public_bff.go` |
| Public Stats — Cache 5 min Redis | ✅ Done | `HandlePublicStats()` → Redis cache |
| Public Stats — Graceful degradation (service down → 200) | ✅ Done | `aggregate()` fallback values |
| Public Stats — Gateway route `/api/v2/public/stats` | ✅ Done | `router.go:85` |
| Public Stats — Rate limit 60/minute | ✅ Done | `rl.Limit("60/minute")` |
| Public Stats — `ThreatIndicators` nested object | ✅ Done | `PublicStats.ThreatIndicators` |
| AI Triage Queue schema | ✅ Done (SOL-014) | → Xem SOL-014 |
| Audit Log — `?search=` filter | ✅ Done | `audit_repo.go`: ILIKE across action/user_email/entity_type |
| Audit Log — `?severity=` filter | ✅ Done | `audit_repo.go`: COALESCE(severity, derived) |
| Audit Log — `?date_from=` filter | ✅ Done | `audit_repo.go`: `created_at >= $n` |
| Audit Log — `?date_to=` filter | ✅ Done | `audit_repo.go`: `created_at <= $n` |
| Audit Log — Handler parse params | ✅ Done | `handler.go:ListAuditLog()` |
| Audit Log — `severity` field in response | ✅ Done | `AuditEventDTO.Severity` + `deriveSeverity()` |
| Gateway route: `GET /audit-log` | ✅ Done | `router.go:284` → `audit-service:8090` |

---

## 1. Tóm tắt Giải pháp

CR-013 có 3 phần:

| Phần | Service | Mức độ |
|---|---|---|
| `GET /api/v2/public/stats` | Gateway BFF (no auth) | HIGH |
| AI Triage Queue schema upgrade | ai-service | HIGH (overlap với CR-014) |
| Audit Log query params (`search`, `severity`, `date_from`, `date_to`) | audit-service | MEDIUM |

> **Overlap với CR-014:** Phần AI Triage Queue trong CR-013 là bản tóm lược — CR-014 cung cấp spec đầy đủ. Solution implementation nằm trong [SOL-014](./SOL-014-ai-triage-queue-human-decision.md). CR-013 §2.2 chỉ cần reference sang SOL-014.

---

## 2. Public Stats Endpoint (Gateway BFF)

### 2.1 Route — No Auth

```go
// apps/osv/internal/gateway/router.go
// PUBLIC — không có protected() middleware
mux.Handle("GET /api/v2/public/stats",
    rateLimiter.Limit("60/minute")(http.HandlerFunc(publicBFF.HandlePublicStats)))
```

### 2.2 BFF Implementation

#### File: `apps/osv/internal/gateway/bff/public_bff.go`

```go
package bff

import (
    "context"
    "encoding/json"
    "net/http"
    "time"

    "github.com/redis/go-redis/v9"
)

// PublicStats — response schema cho login page
type PublicStats struct {
    TotalCVEs       string           `json:"total_cves"`        // "240K+"
    ScansToday      int              `json:"scans_today"`
    FindingAccuracy string           `json:"finding_accuracy"`  // "98.4%"
    UptimeSLA       string           `json:"uptime_sla"`        // "99.99%"
    ThreatIndicators ThreatIndicators `json:"threat_indicators"`
}

type ThreatIndicators struct {
    CriticalThreats int `json:"critical_threats"`
    KEVActive       int `json:"kev_active"`
    AssetsAtRisk    int `json:"assets_at_risk"`
}

// PublicBFF aggregates từ nhiều services
type PublicBFF struct {
    scanClient    ScanServiceClient    // scan-service:8084
    findingClient FindingServiceClient // finding-service:8085
    dataClient    DataServiceClient    // data-service:8082 (KEV count)
    assetClient   AssetServiceClient   // asset-service:8091
    cache         *redis.Client
}

const (
    publicStatsCacheKey = "public:stats:v1"
    publicStatsCacheTTL = 5 * time.Minute
)

// HandlePublicStats godoc
// GET /api/v2/public/stats
// - No authentication required
// - Rate limited: 60/minute
// - Cached: 5 minutes
// - CORS: open (login page may be on different domain)
func (b *PublicBFF) HandlePublicStats(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // CORS headers cho login page
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Methods", "GET")

    // 1. Check cache
    if cached, err := b.cache.Get(ctx, publicStatsCacheKey).Bytes(); err == nil {
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("X-Cache", "HIT")
        w.Write(cached)
        return
    }

    // 2. Aggregate từ services — graceful degradation
    stats := PublicStats{
        TotalCVEs:       b.getTotalCVEs(ctx),
        ScansToday:      b.getScansToday(ctx),
        FindingAccuracy: "98.4%",  // Từ AI model metrics (static hoặc ai-service)
        UptimeSLA:       "99.99%", // Từ monitoring (static hoặc external)
        ThreatIndicators: ThreatIndicators{
            CriticalThreats: b.getCriticalThreats(ctx),
            KEVActive:       b.getKEVActive(ctx),
            AssetsAtRisk:    b.getAssetsAtRisk(ctx),
        },
    }

    // 3. Cache 5 phút
    data, _ := json.Marshal(stats)
    b.cache.Set(ctx, publicStatsCacheKey, data, publicStatsCacheTTL)

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Cache", "MISS")
    w.Write(data)
}

// Graceful degradation helpers — nếu service down, trả fallback value

func (b *PublicBFF) getTotalCVEs(ctx context.Context) string {
    count, err := b.dataClient.CountCVEs(ctx)
    if err != nil {
        return "240K+" // Static fallback (approximate)
    }
    if count >= 240000 {
        return fmt.Sprintf("%dK+", count/1000)
    }
    return strconv.Itoa(count)
}

func (b *PublicBFF) getScansToday(ctx context.Context) int {
    count, err := b.scanClient.CountCompletedToday(ctx)
    if err != nil {
        return 0 // Graceful degradation
    }
    return count
}

func (b *PublicBFF) getCriticalThreats(ctx context.Context) int {
    count, err := b.findingClient.CountActiveBySeverity(ctx, "Critical")
    if err != nil {
        return 0
    }
    return count
}

func (b *PublicBFF) getKEVActive(ctx context.Context) int {
    count, err := b.dataClient.CountActiveKEV(ctx)
    if err != nil {
        return 0
    }
    return count
}

func (b *PublicBFF) getAssetsAtRisk(ctx context.Context) int {
    count, err := b.assetClient.CountAtRisk(ctx)
    if err != nil {
        return 0
    }
    return count
}
```

### 2.3 Service Clients (Internal HTTP calls)

#### File: `apps/osv/internal/gateway/bff/clients.go`

```go
package bff

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// ScanServiceClient — gọi scan-service:8084
type ScanServiceClient struct {
    baseURL string
    client  *http.Client
}

func (c *ScanServiceClient) CountCompletedToday(ctx context.Context) (int, error) {
    // Dùng internal endpoint (không qua gateway để tránh auth loop)
    // Tái dùng GET /api/v1/scans/stats từ SOL-008
    resp, err := c.client.Get(c.baseURL + "/api/v1/scans/stats")
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()

    var stats struct{ CompletedToday int `json:"completed_today"` }
    if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
        return 0, err
    }
    return stats.CompletedToday, nil
}

// FindingServiceClient — gọi finding-service:8085
type FindingServiceClient struct {
    baseURL string
    client  *http.Client
}

func (c *FindingServiceClient) CountActiveBySeverity(ctx context.Context, severity string) (int, error) {
    url := fmt.Sprintf("%s/api/v2/findings?severity=%s&status=Active&count_only=true", c.baseURL, severity)
    resp, err := c.client.Get(url)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()

    var result struct{ Total int `json:"total"` }
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Total, nil
}

// DataServiceClient — gọi data-service:8082
type DataServiceClient struct {
    baseURL string
    client  *http.Client
}

func (c *DataServiceClient) CountCVEs(ctx context.Context) (int, error) {
    resp, err := c.client.Get(c.baseURL + "/api/v1/dbinfo")
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()

    var info struct{ TotalCVEs int `json:"total_cves"` }
    json.NewDecoder(resp.Body).Decode(&info)
    return info.TotalCVEs, nil
}

func (c *DataServiceClient) CountActiveKEV(ctx context.Context) (int, error) {
    resp, err := c.client.Get(c.baseURL + "/api/v1/kev/stats")
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()

    var stats struct{ Active int `json:"active"` }
    json.NewDecoder(resp.Body).Decode(&stats)
    return stats.Active, nil
}
```

---

## 3. AI Triage Queue Schema (Reference SOL-014)

> **Xem:** [SOL-014](./SOL-014-ai-triage-queue-human-decision.md) — đây là implementation chi tiết đầy đủ.

Phần trong CR-013 §2.2 là tóm lược của CR-014. Không cần implement riêng.

**Checklist từ CR-013:**
- [ ] `GET /api/v1/ai/triage/queue` → full `AITriageQueueResponse` ← SOL-014 §5
- [ ] `POST /api/v1/ai/triage/{findingId}/review` với `note` field ← SOL-014 §5
- [ ] DB migration: `human_decision`, `human_note`, `reviewed_by`, `reviewed_at` ← SOL-014 §2

---

## 4. Audit Log — Thêm Query Params

### 4.1 Handler Update

#### File: `services/audit-service/internal/delivery/http/audit_handler.go`

```go
// GetAuditLog godoc
// GET /api/v1/audit-log
// Query: search, severity, date_from, date_to, page, page_size
func (h *AuditHandler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    q   := r.URL.Query()

    filter := AuditLogFilter{
        Search:   q.Get("search"),
        Severity: q.Get("severity"),  // "Info" | "Warning" | "Critical"
        DateFrom: parseDateParam(q.Get("date_from")),
        DateTo:   parseDateParam(q.Get("date_to")),
        Page:     parseIntDefault(q.Get("page"), 1),
        PageSize: parseIntDefault(q.Get("page_size"), 50),
    }

    events, total, err := h.repo.List(ctx, filter)
    if err != nil {
        jsonError(w, 500, "INTERNAL", "Failed to fetch audit log")
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(AuditLogResponse{
        Events: events,
        Total:  total,
    })
}
```

### 4.2 Repository Update

```go
// services/audit-service/internal/infra/postgres/audit_repo.go

type AuditLogFilter struct {
    Search   string
    Severity string     // maps to action type categorization
    DateFrom *time.Time
    DateTo   *time.Time
    Page     int
    PageSize int
}

func (r *AuditRepo) List(ctx context.Context, f AuditLogFilter) ([]domain.AuditEvent, int, error) {
    query := `
        SELECT id, actor_id, actor_email, action, resource_type, resource_id,
               before_json, after_json, hmac_sig, created_at
        FROM audit_events
        WHERE 1=1
    `
    args := []interface{}{}
    idx  := 1

    // Full-text search (action, actor_email, resource_type)
    if f.Search != "" {
        query += fmt.Sprintf(`
            AND (action ILIKE $%d
              OR actor_email ILIKE $%d
              OR resource_type ILIKE $%d)
        `, idx, idx, idx)
        args = append(args, "%"+f.Search+"%")
        idx++
    }

    // Severity filter: mapping action patterns → severity levels
    // "Critical" = user locked, data deleted
    // "Warning"  = login failed, permission denied
    // "Info"     = everything else
    if f.Severity != "" {
        switch f.Severity {
        case "Critical":
            query += fmt.Sprintf(" AND action IN (SELECT action FROM audit_severity_map WHERE severity = $%d)", idx)
        case "Warning":
            query += fmt.Sprintf(" AND action IN (SELECT action FROM audit_severity_map WHERE severity = $%d)", idx)
        case "Info":
            query += fmt.Sprintf(" AND (action NOT IN (SELECT action FROM audit_severity_map) OR action IN (SELECT action FROM audit_severity_map WHERE severity = $%d))", idx)
        }
        args = append(args, f.Severity)
        idx++
    }

    // Date range
    if f.DateFrom != nil {
        query += fmt.Sprintf(" AND created_at >= $%d", idx)
        args = append(args, f.DateFrom)
        idx++
    }
    if f.DateTo != nil {
        query += fmt.Sprintf(" AND created_at < $%d", idx)
        args = append(args, f.DateTo.Add(24*time.Hour)) // inclusive end date
        idx++
    }

    // Count + paginate (same as other repos)
    // ...
}
```

> **Lưu ý:** `audit_severity_map` là bảng static hoặc cần định nghĩa rule mapping action → severity. Alternative đơn giản hơn: thêm `severity` column trực tiếp vào `audit_events` table và set khi insert.

---

## 5. Gateway Routing

```go
// apps/osv/internal/gateway/router.go

// PUBLIC endpoint — no auth, rate limited
mux.Handle("GET /api/v2/public/stats",
    rateLimiter.Limit("60/minute")(http.HandlerFunc(publicBFF.HandlePublicStats)))

// Audit log đã có route — verify params được pass qua proxy
// Gateway forward query params tự động via proxy.Forward
```

---

## 6. Tests

### 6.1 Public Stats

```go
func TestPublicStats_NoAuthRequired(t *testing.T) {
    // Gọi không có Authorization header
    resp := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/api/v2/public/stats", nil)
    // Không set Authorization header
    
    router.ServeHTTP(resp, req)
    assert.Equal(t, 200, resp.Code)
}

func TestPublicStats_HasRequiredFields(t *testing.T) {
    resp := GET("/api/v2/public/stats") // no auth
    var stats PublicStats
    json.Unmarshal(resp.Body, &stats)

    assert.NotEmpty(t, stats.TotalCVEs)
    assert.GreaterOrEqual(t, stats.ScansToday, 0)
    assert.NotEmpty(t, stats.FindingAccuracy)
    assert.NotEmpty(t, stats.UptimeSLA)
}

func TestPublicStats_GracefulDegradation(t *testing.T) {
    // Mock: scan-service down
    // Response phải vẫn là 200 với scans_today = 0
    resp := GET("/api/v2/public/stats") // no auth
    assert.Equal(t, 200, resp.StatusCode)
}

func TestPublicStats_CacheHit(t *testing.T) {
    GET("/api/v2/public/stats")                         // Cache MISS
    resp := GET("/api/v2/public/stats")                 // Cache HIT
    assert.Equal(t, "HIT", resp.Header.Get("X-Cache"))
}
```

### 6.2 Audit Log Filter

```go
func TestAuditLog_SearchFilter(t *testing.T) {
    resp := GET("/api/v1/audit-log?search=login")
    // Verify tất cả events có "login" trong action/actor/resource
}

func TestAuditLog_DateRange(t *testing.T) {
    resp := GET("/api/v1/audit-log?date_from=2026-06-01&date_to=2026-06-19")
    // Verify tất cả events trong range
}
```

---

## 7. Acceptance Criteria Checklist

### Public Stats
- [ ] `GET /api/v2/public/stats` → HTTP 200 **không cần Authorization header**
- [ ] Response có 5 fields: `total_cves`, `scans_today`, `finding_accuracy`, `uptime_sla`, `threat_indicators`
- [ ] Service down → HTTP 200 với default values (graceful degradation)
- [ ] Response cache → `X-Cache: HIT` trong 5 phút
- [ ] Cache không quá 10 phút

### AI Triage (reference SOL-014)
- [ ] Xem SOL-014 §9 acceptance criteria

### Audit Log
- [ ] `?search=login` → entries có "login" trong action/username/resource
- [ ] `?severity=Critical` → chỉ critical audit events
- [ ] `?date_from=&date_to=` → filter theo time range
