# TASK-GCV-034 — Source Attribution in CVE Response

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-034 |
| **Service** | `search-service` |
| **CR** | CR-GCV-010 |
| **Phase** | 4 — Export & UI Support |
| **Priority** | 🟢 Low |
| **Prerequisites** | TASK-GCV-005 |

## Context

Thêm `source` field vào CVE response từ search-service để UI có thể hiển thị nguồn data (NVD, CIRCL, JVN...). Khi TASK-GCV-005 đã thêm `source` column vào DB, task này đảm bảo `search-service` entity đọc và trả field đó.

## Reference

- Solution: [SOL-GCV-010](../solutions/SOL-GCV-010-export-frontend-support.md) §2.3

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/domain/entity/cve.go
        (ADD Source string field nếu chưa có)
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/infra/postgres/cve_pg.go
        (ADD source column to SELECT queries)
```

## Implementation Spec

### entity/cve.go — ADD Source field

Trong `search-service` CVE entity, thêm (nếu chưa có):

```go
type CVE struct {
    // ... existing fields ...
    Source     string  `db:"source"     json:"source,omitempty"`
    IsExploit  bool    `db:"is_exploit" json:"is_exploit,omitempty"`
}
```

### cve_pg.go — ADD source to SELECT

Tìm tất cả SELECT queries trong PostgreSQL CVE repository của `search-service`, thêm `source, is_exploit` vào SELECT columns:

```go
// Example existing query:
// SELECT id, description, severity, cvss3, epss, published, updated_at FROM cves WHERE ...

// Updated to include source:
// SELECT id, description, severity, cvss3, epss, published, updated_at,
//        COALESCE(source, 'NVD') AS source,
//        COALESCE(is_exploit, FALSE) AS is_exploit
// FROM cves WHERE ...
```

**Lưu ý**: Dùng `COALESCE(source, 'NVD')` để đảm bảo backward compatibility với existing CVEs chưa có source.

### Source constants (hiển thị dạng thân thiện)

```go
// Trong entity package:
var SourceDisplayNames = map[string]string{
    "NVD":       "National Vulnerability Database",
    "CIRCL":     "CIRCL (Luxembourg)",
    "JVN":       "Japan Vulnerability Notes",
    "EXPLOITDB": "Exploit Database",
    "CVE.ORG":   "CVE.org (MITRE)",
    "CNNVD":     "China National Vulnerability Database",
    "EPSS":      "EPSS Model",
}
```

## Acceptance Criteria

- [x] `GET /api/v2/cves/{id}` response có `"source": "NVD"` (hoặc CIRCL, JVN, etc.)
- [x] `GET /api/v2/cves` list response — mỗi CVE có `source` field
- [x] Existing CVEs không có `source` trong DB → default `"NVD"` (COALESCE)
- [x] `GET /api/v2/cves/export?format=csv` có `Source` column với giá trị đúng
- [x] `go build ./...` pass


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Verified directly from codebase.

---

# TASK-GCV-035 — Dashboard Stats API

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-035 |
| **Service** | `search-service` |
| **CR** | CR-GCV-010 |
| **Phase** | 4 — Export & UI Support |
| **Priority** | 🟢 Low |
| **Prerequisites** | — |

## Context

Thêm `GET /api/v2/stats/dashboard` endpoint tổng hợp tất cả stats cần thiết cho frontend dashboard: total CVEs, by severity, trending (7 ngày), top vendors, KEV stats, EPSS distribution.

## Reference

- Solution: [SOL-GCV-010](../solutions/SOL-GCV-010-export-frontend-support.md) §2.4

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/delivery/http/stats_handler.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/domain/repository/
        (ADD GetDashboardStats to CVERepository)
```

## Implementation Spec

### DashboardStats response struct

```go
type DashboardStats struct {
    TotalCVEs       int64              `json:"total_cves"`
    BySeverity      map[string]int64   `json:"by_severity"`
    NewThisWeek     int64              `json:"new_this_week"`
    NewThisMonth    int64              `json:"new_this_month"`
    TopVendors      []VendorCVECount   `json:"top_vendors"`
    EPSSDistrib     map[string]int64   `json:"epss_distribution"` // "0-0.1","0.1-0.5","0.5-0.9","0.9+"
    TotalKEV        int64              `json:"total_kev"`
    TotalExploit    int64              `json:"total_exploit"`
    LastUpdated     time.Time          `json:"last_updated"`
}

type VendorCVECount struct {
    Vendor string `json:"vendor"`
    Count  int64  `json:"count"`
}
```

### SQL queries

```sql
-- total
SELECT COUNT(*) FROM cves;

-- by_severity
SELECT COALESCE(severity,'UNKNOWN'), COUNT(*) FROM cves GROUP BY severity;

-- new_this_week
SELECT COUNT(*) FROM cves WHERE published >= NOW() - INTERVAL '7 days';

-- new_this_month
SELECT COUNT(*) FROM cves WHERE published >= NOW() - INTERVAL '30 days';

-- top_vendors (top 10)
SELECT vendor, COUNT(*) as count
FROM cves, UNNEST(vendors) as vendor
GROUP BY vendor ORDER BY count DESC LIMIT 10;

-- epss_distribution
SELECT
    COUNT(*) FILTER (WHERE epss < 0.1)          AS low,
    COUNT(*) FILTER (WHERE epss >= 0.1 AND epss < 0.5) AS medium,
    COUNT(*) FILTER (WHERE epss >= 0.5 AND epss < 0.9) AS high,
    COUNT(*) FILTER (WHERE epss >= 0.9)          AS critical
FROM cves WHERE epss IS NOT NULL;

-- total_kev, total_exploit
SELECT
    COUNT(*) FILTER (WHERE is_kev = TRUE)     AS total_kev,
    COUNT(*) FILTER (WHERE is_exploit = TRUE) AS total_exploit
FROM cves;
```

### stats_handler.go

```go
// GET /api/v2/stats/dashboard
func (h *StatsHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
    stats, err := h.cveRepo.GetDashboardStats(r.Context())
    if err != nil {
        respondError(w, 500, "failed to get dashboard stats")
        return
    }
    respondJSON(w, 200, stats)
}
```

### Route registration

```go
r.Get("/api/v2/stats/dashboard", h.Dashboard)
```

## Acceptance Criteria

- [x] `GET /api/v2/stats/dashboard` → 200 với `total_cves`, `by_severity`, `new_this_week`, `top_vendors`
- [x] `by_severity` có keys: `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `UNKNOWN`
- [x] `epss_distribution` có keys: `low`, `medium`, `high`, `critical` (theo ranges)
- [x] `top_vendors` là 10 vendors có nhiều CVEs nhất
- [x] Response time < 1s (cached bởi gateway 5 phút)
- [x] Response có `last_updated` timestamp
- [x] `go build ./...` pass
