# TASK-GCV-016 — KEV Ransomware Endpoint + Advanced Stats

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-016 |
| **Service** | `data-service` |
| **CR** | CR-GCV-007 |
| **Phase** | 2 — Enrichment |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-GCV-014 |

## Context

Thêm `GET /api/v2/kev/ransomware` endpoint và nâng cấp `GET /api/v2/kev/stats` để trả thêm: `total_ransomware`, `top_vendors`, `by_month` (12 tháng), `avg_days_to_patch`.

## Reference

- Solution: [SOL-GCV-007](../solutions/SOL-GCV-007-kev-enhancement.md) §2.4, §2.5
- CR: [CR-GCV-007](../CR-GCV-007-kev-service-enhancement.md)

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/delivery/http/kev_handler.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/usecase/kev/ (query usecase)
```

**Đọc trước**: Cấu trúc KEV handler và query use cases hiện có.

## Implementation Spec

### Query UseCase — ADD IsRansomware filter

Tìm Request struct trong KEV query use case, thêm:

```go
type Request struct {
    // existing fields...
    IsRansomware bool    // NEW: filter only known ransomware KEV
    Page         int
    Limit        int
}
```

Trong query builder:
```go
if req.IsRansomware {
    query = query.Where("is_known_ransomware = TRUE")
    // hoặc: conditions = append(conditions, "is_known_ransomware = TRUE")
}
```

### Stats Response — EXTEND

Tìm `GetStats` use case/handler, extend response struct:

```go
type KEVStats struct {
    // ─── EXISTING ──────────────────────────────────────────────────────────
    Total          int `json:"total"`
    AddedThisMonth int `json:"added_this_month"`
    AddedThisYear  int `json:"added_this_year"`

    // ─── NEW ───────────────────────────────────────────────────────────────
    TotalRansomware int            `json:"total_ransomware"`
    TopVendors      []VendorCount  `json:"top_vendors"`    // top 10 by count
    ByMonth         []MonthCount   `json:"by_month"`       // last 12 months
    AvgDaysToPatch  float64        `json:"avg_days_to_patch"`
}

type VendorCount struct {
    Vendor string `json:"vendor"`
    Count  int    `json:"count"`
}

type MonthCount struct {
    Month string `json:"month"` // "2026-01"
    Count int    `json:"count"`
}
```

SQL queries cho stats mới:

```sql
-- total_ransomware
SELECT COUNT(*) FROM kev_entries WHERE is_known_ransomware = TRUE;

-- top_vendors (top 10)
SELECT vendor_project AS vendor, COUNT(*) AS count
FROM kev_entries
GROUP BY vendor_project
ORDER BY count DESC
LIMIT 10;

-- by_month (last 12 months)
SELECT to_char(date_added, 'YYYY-MM') AS month, COUNT(*) AS count
FROM kev_entries
WHERE date_added >= NOW() - INTERVAL '12 months'
GROUP BY month
ORDER BY month ASC;

-- avg_days_to_patch
-- (avg days from CVE published to CISA added to KEV)
SELECT COALESCE(AVG(
    EXTRACT(EPOCH FROM (k.date_added::timestamptz - c.published)) / 86400
), 0) AS avg_days
FROM kev_entries k
JOIN cves c ON c.id = k.cve_id
WHERE c.published IS NOT NULL
  AND k.date_added IS NOT NULL;
```

### HTTP Handler — ADD GetRansomware

```go
// GetRansomware handles GET /api/v2/kev/ransomware
// Returns all KEV entries classified as known ransomware.
func (h *KevHandler) GetRansomware(w http.ResponseWriter, r *http.Request) {
    req := &query.Request{
        IsRansomware: true,
        Page:  parseInt(r.URL.Query().Get("page"), 0),
        Limit: parseInt(r.URL.Query().Get("limit"), 50),
    }

    resp, err := h.queryUC.Execute(r.Context(), req)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to query ransomware KEV")
        return
    }
    respondJSON(w, http.StatusOK, resp)
}
```

### Router — ADD Route

**QUAN TRỌNG**: `/api/v2/kev/ransomware` phải đứng TRƯỚC `/api/v2/kev/{cveId}` trong router:

```go
// Trong KEV router setup:
r.Get("/api/v2/kev/ransomware", h.GetRansomware)  // SPECIFIC: phải trước
r.Get("/api/v2/kev/{cveId}", h.GetKEVEntry)        // GENERIC: sau
r.Get("/api/v2/kev", h.ListKEV)
r.Get("/api/v2/kev/stats", h.GetStats)             // SPECIFIC: phải trước /kev
```

## Acceptance Criteria

- [x] `GET /api/v2/kev/ransomware` → chỉ return entries với `is_known_ransomware=true`
- [x] `GET /api/v2/kev/ransomware?limit=10` → paginated correctly
- [x] `GET /api/v2/kev/stats` → response có `total_ransomware`, `top_vendors`, `by_month`, `avg_days_to_patch`
- [x] `top_vendors` là array 10 elements sorted by count DESC
- [x] `by_month` có 12 entries (12 tháng gần nhất), sorted ASC
- [x] `avg_days_to_patch` là số float hợp lý (0-365 ngày)
- [x] `GET /api/v2/kev/ransomware` không conflict với `GET /api/v2/kev/{cveId}` routing
- [x] Response KEV list có `is_known_ransomware`, `short_description`, `required_action` fields
- [x] `go build ./...` pass không lỗi
