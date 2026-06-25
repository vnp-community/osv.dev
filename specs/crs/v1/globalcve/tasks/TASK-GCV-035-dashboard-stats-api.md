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

Thêm `GET /api/v2/stats/dashboard` tổng hợp các metrics cho frontend dashboard: total CVEs, by severity, trending, top vendors, KEV count, EPSS distribution. Được cache 5 phút bởi gateway.

## Reference

- Solution: [SOL-GCV-010](../solutions/SOL-GCV-010-export-frontend-support.md) §2.4

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/delivery/http/stats_handler.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/domain/repository/
        (ADD GetDashboardStats to CVERepository interface)
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/infra/postgres/
        (IMPLEMENT GetDashboardStats)
```

## Implementation Spec

### DashboardStats struct

```go
type DashboardStats struct {
    TotalCVEs      int64              `json:"total_cves"`
    BySeverity     map[string]int64   `json:"by_severity"`
    NewThisWeek    int64              `json:"new_this_week"`
    NewThisMonth   int64              `json:"new_this_month"`
    TopVendors     []VendorCVECount   `json:"top_vendors"`
    EPSSDistrib    EPSSDistribution   `json:"epss_distribution"`
    TotalKEV       int64              `json:"total_kev"`
    TotalExploit   int64              `json:"total_exploit"`
    LastUpdated    time.Time          `json:"last_updated"`
}

type VendorCVECount struct {
    Vendor string `json:"vendor"`
    Count  int64  `json:"count"`
}

type EPSSDistribution struct {
    VeryLow  int64 `json:"very_low"`  // 0–0.1
    Low      int64 `json:"low"`       // 0.1–0.5
    High     int64 `json:"high"`      // 0.5–0.9
    Critical int64 `json:"critical"`  // 0.9+
}
```

### PostgreSQL implementation — GetDashboardStats

```go
func (r *pgCVERepository) GetDashboardStats(ctx context.Context) (*repository.DashboardStats, error) {
    stats := &repository.DashboardStats{
        BySeverity:  make(map[string]int64),
        LastUpdated: time.Now().UTC(),
    }

    // 1. Total CVEs
    r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cves").Scan(&stats.TotalCVEs)

    // 2. By severity
    rows, _ := r.db.QueryContext(ctx,
        "SELECT COALESCE(severity,'UNKNOWN'), COUNT(*) FROM cves GROUP BY severity")
    for rows.Next() {
        var sev string; var count int64
        rows.Scan(&sev, &count)
        stats.BySeverity[sev] = count
    }
    rows.Close()

    // 3. Trending
    r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cves WHERE published >= NOW() - INTERVAL '7 days'").
        Scan(&stats.NewThisWeek)
    r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cves WHERE published >= NOW() - INTERVAL '30 days'").
        Scan(&stats.NewThisMonth)

    // 4. Top vendors (explode vendors array)
    vendorRows, _ := r.db.QueryContext(ctx, `
        SELECT vendor, COUNT(*) as count
        FROM cves, UNNEST(vendors) as vendor
        WHERE vendor IS NOT NULL AND vendor != ''
        GROUP BY vendor ORDER BY count DESC LIMIT 10
    `)
    for vendorRows.Next() {
        var v repository.VendorCVECount
        vendorRows.Scan(&v.Vendor, &v.Count)
        stats.TopVendors = append(stats.TopVendors, v)
    }
    vendorRows.Close()

    // 5. EPSS distribution
    r.db.QueryRowContext(ctx, `
        SELECT
            COUNT(*) FILTER (WHERE epss < 0.1)                  AS very_low,
            COUNT(*) FILTER (WHERE epss >= 0.1 AND epss < 0.5)  AS low,
            COUNT(*) FILTER (WHERE epss >= 0.5 AND epss < 0.9)  AS high,
            COUNT(*) FILTER (WHERE epss >= 0.9)                  AS critical
        FROM cves WHERE epss IS NOT NULL
    `).Scan(&stats.EPSSDistrib.VeryLow, &stats.EPSSDistrib.Low,
        &stats.EPSSDistrib.High, &stats.EPSSDistrib.Critical)

    // 6. KEV + Exploit counts
    r.db.QueryRowContext(ctx, `
        SELECT
            COUNT(*) FILTER (WHERE is_kev = TRUE)     AS total_kev,
            COUNT(*) FILTER (WHERE is_exploit = TRUE) AS total_exploit
        FROM cves
    `).Scan(&stats.TotalKEV, &stats.TotalExploit)

    return stats, nil
}
```

### stats_handler.go

```go
package http

type StatsHandler struct {
    cveRepo repository.CVERepository
}

func NewStatsHandler(repo repository.CVERepository) *StatsHandler {
    return &StatsHandler{cveRepo: repo}
}

// GET /api/v2/stats/dashboard
func (h *StatsHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
    stats, err := h.cveRepo.GetDashboardStats(r.Context())
    if err != nil {
        respondError(w, 500, "failed to compute dashboard stats")
        return
    }
    respondJSON(w, 200, stats)
}
```

### Route registration

```go
statsHandler := http.NewStatsHandler(cveRepo)
r.Get("/api/v2/stats/dashboard", statsHandler.Dashboard)
```

## Acceptance Criteria

- [x] `GET /api/v2/stats/dashboard` → 200 với tất cả fields: `total_cves`, `by_severity`, `new_this_week`, `new_this_month`, `top_vendors`, `epss_distribution`, `total_kev`, `total_exploit`, `last_updated`
- [x] `by_severity` map có keys: CRITICAL, HIGH, MEDIUM, LOW, UNKNOWN
- [x] `top_vendors` là array 10 elements, sorted by count DESC
- [x] `epss_distribution` có 4 ranges: `very_low` (0-0.1), `low` (0.1-0.5), `high` (0.5-0.9), `critical` (0.9+)
- [x] `total_kev` = số CVE có `is_kev = TRUE`
- [x] Response time hợp lý (< 2s khi chưa cache, < 100ms từ gateway cache sau lần đầu)
- [x] `last_updated` là timestamp hiện tại (UTC, RFC3339)
- [x] `go build ./...` pass


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Verified directly from codebase.
