# SOL-GCV-002 — EPSS Integration (Daily Scoring & Filter/Sort)

| Trường | Giá trị |
|--------|---------|
| **CR** | [CR-GCV-002](../CR-GCV-002-epss-integration.md) |
| **Target Service** | `data-service` (EPSS sync) + `search-service` (filter/sort) |
| **apps/osv role** | Không thay đổi — gateway forward |
| **Priority** | 🔴 High |

---

## 1. Hiện trạng

- `data-service/internal/fetcher/epss.go` → **đã có** EPSS fetcher
- `data-service/internal/domain/entity/cve.go` → `EPSS float64`, `EPSSPercentile float64` **đã có**
- `search-service/internal/delivery/http/search_handler.go` → `SearchCVEs` **chưa có** `min_epss`, `sort=epss_desc` params

---

## 2. Giải pháp

### 2.1 data-service — Kiểm tra và hoàn chỉnh EPSS fetcher

**File**: `data-service/internal/fetcher/epss.go` (review + patch nếu thiếu)

EPSS fetcher đã có — verify nó:
1. Download từ `https://epss.cyentia.com/epss_scores-YYYY-MM-DD.csv.gz`
2. Parse CSV: `cve_id,epss_score,epss_percentile`
3. Batch upsert vào column `epss` và `epss_percentile` của bảng `cves`

**Schedule**: `0 0 3 * * *` (daily 3am) — đã có trong scheduler

**Migration** (nếu chưa có columns):
```sql
ALTER TABLE cves ADD COLUMN IF NOT EXISTS epss NUMERIC(8,7) DEFAULT NULL;
ALTER TABLE cves ADD COLUMN IF NOT EXISTS epss_percentile NUMERIC(8,7) DEFAULT NULL;
ALTER TABLE cves ADD COLUMN IF NOT EXISTS epss_updated_at TIMESTAMPTZ DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_cves_epss ON cves(epss) WHERE epss IS NOT NULL;
```

### 2.2 search-service — Thêm EPSS filter + sort

**File**: `search-service/internal/usecase/cvesearch/request.go` (thêm fields)

```go
type Request struct {
    Query    string
    Severity string
    Source   string
    Sort     string  // "newest" | "oldest" | "severity_desc" | "epss_desc" (NEW)
    Page     int
    Limit    int

    // NEW — EPSS filter
    MinEPSS  *float64  // filter CVEs with epss >= min_epss (e.g., 0.5)
    MaxEPSS  *float64  // optional upper bound
}
```

**File**: `search-service/internal/delivery/http/search_handler.go` (parse new params)

```go
func (h *Handler) SearchCVEs(w http.ResponseWriter, r *http.Request) {
    req := &search.Request{
        Query:    r.URL.Query().Get("query"),
        Severity: strings.ToUpper(r.URL.Query().Get("severity")),
        Source:   strings.ToUpper(r.URL.Query().Get("source")),
        Sort:     r.URL.Query().Get("sort"),
        Page:     parseInt(r.URL.Query().Get("page"), 0),
        Limit:    parseInt(r.URL.Query().Get("limit"), 50),
    }

    // NEW: EPSS filter
    if minEPSS := r.URL.Query().Get("min_epss"); minEPSS != "" {
        if v, err := strconv.ParseFloat(minEPSS, 64); err == nil {
            req.MinEPSS = &v
        }
    }

    // ... execute usecase
}
```

**File**: `search-service/internal/usecase/cvesearch/usecase.go` (apply filter)

```go
// Trong repo query builder, thêm EPSS filter:
if req.MinEPSS != nil {
    query = query.Where("epss >= ?", *req.MinEPSS)
}

// Sort by EPSS descending:
switch req.Sort {
case "epss_desc":
    query = query.OrderBy("epss DESC NULLS LAST")
case "severity_desc":
    query = query.OrderBy("cvss3 DESC")
// ... existing sorts
}
```

### 2.3 EPSS in API Response

**File**: `search-service/internal/domain/entity/cve.go` (verify EPSS fields)

```go
type CVE struct {
    // ... existing fields ...
    EPSS           *float64 `db:"epss"             json:"epss,omitempty"`
    EPSSPercentile *float64 `db:"epss_percentile"  json:"epss_percentile,omitempty"`
    EPSSUpdatedAt  *time.Time `db:"epss_updated_at" json:"epss_updated_at,omitempty"`
}
```

### 2.4 EPSS Stats Endpoint

**File**: `search-service/internal/delivery/http/search_handler.go` (new handler)

```go
// GET /api/v2/epss/stats
// Response: { total_scored, avg_epss, high_risk_count (epss >= 0.9), top_cves: [...] }
func (h *Handler) EPSSStats(w http.ResponseWriter, r *http.Request) {
    stats, err := h.epssRepo.GetStats(r.Context())
    // ...
}
```

**Route**: `search-service/internal/delivery/http/search_handler.go`
```go
r.Get("/api/v2/epss/stats", h.EPSSStats)
```

---

## 3. apps/osv Changes

> **Không thay đổi** — gateway forward `/api/v2/cves?min_epss=...` tới `search-service` như cũ.

Gateway caching config (gateway-service):
```yaml
cache:
  epss_stats_ttl: 1800  # 30 minutes
```

---

## 4. Files cần tạo/sửa

### data-service (VERIFY/FIX)
```
internal/fetcher/epss.go           ← Verify hoàn chỉnh (có thể cần fix)
migrations/XXXX_add_epss_cols.sql  ← Nếu chưa có columns
```

### search-service (MODIFY)
```
internal/usecase/cvesearch/request.go    ← Add MinEPSS, MaxEPSS fields
internal/usecase/cvesearch/usecase.go    ← Apply filter + sort logic
internal/delivery/http/search_handler.go ← Parse min_epss param, add /epss/stats
internal/domain/entity/cve.go           ← Verify EPSS fields in response entity
internal/domain/repository/cve_repo.go  ← Add EPSSStats method
```

### gateway-service (MODIFY)
```
config/config.yaml  ← Add epss_stats_ttl cache config
```

---

## 5. API Spec

```
GET /api/v2/cves?min_epss=0.5            → CVEs với EPSS >= 0.5
GET /api/v2/cves?sort=epss_desc          → Sorted by EPSS descending
GET /api/v2/cves?min_epss=0.9&sort=epss_desc → High-risk CVEs
GET /api/v2/epss/stats                   → EPSS statistics summary
```

---

## 6. Acceptance Criteria

- [x] `GET /api/v2/cves?min_epss=0.9` → chỉ trả CVEs có `epss >= 0.9`
- [x] `GET /api/v2/cves?sort=epss_desc` → sorted by EPSS descending, nulls last
- [x] Response mỗi CVE có `epss` và `epss_percentile` fields
- [x] `GET /api/v2/epss/stats` → trả `total_scored`, `avg_epss`, `high_risk_count`
- [x] EPSS sync chạy daily 3am, update epss cho ~200K CVEs
- [x] EPSS cũ không bị xóa khi EPSS mới thiếu CVE ID (NULL-safe upsert)


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Toàn bộ giải pháp đã được triển khai đầy đủ và build verified.
