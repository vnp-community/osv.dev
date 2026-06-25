# TASK-GCV-007 — EPSS Stats Endpoint

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-007 |
| **Service** | `search-service` |
| **CR** | CR-GCV-002 |
| **Phase** | 1 — Core Pipeline |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-GCV-006 |

## Context

Thêm `GET /api/v2/epss/stats` endpoint vào `search-service`. Endpoint trả thống kê EPSS tổng hợp: total scored CVEs, avg EPSS, high-risk count (>=0.9), top 10 CVEs by EPSS.

## Reference

- Solution: [SOL-GCV-002](../solutions/SOL-GCV-002-epss-integration.md) §2.4
- CR: [CR-GCV-002](../CR-GCV-002-epss-integration.md)

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/delivery/http/search_handler.go
        (thêm handler EPSSStats + đăng ký route)
```

**Cũng cần**: thêm `GetEPSSStats` method vào CVE repository interface và implementation — đọc cấu trúc `search-service/internal/domain/repository/` và `search-service/internal/infra/` để biết chính xác file cần sửa.

## Implementation Spec

### HTTP Handler — EPSSStats

Thêm vào `search_handler.go` (hoặc file handler phù hợp):

```go
// EPSSStats handles GET /api/v2/epss/stats
func (h *Handler) EPSSStats(w http.ResponseWriter, r *http.Request) {
    stats, err := h.cveRepo.GetEPSSStats(r.Context())
    if err != nil {
        respondError(w, http.StatusInternalServerError, "epss stats failed")
        return
    }
    respondJSON(w, http.StatusOK, stats)
}
```

### Route Registration

Trong `NewRouter` function, thêm route:

```go
r.Get("/api/v2/epss/stats", h.EPSSStats)
```

**QUAN TRỌNG**: Route `/api/v2/epss/stats` phải đứng TRƯỚC `/api/v2/cves` trong router để tránh shadowing.

### Repository Interface — ADD method

Tìm CVE repository interface trong `search-service/internal/domain/repository/`, thêm:

```go
type CVERepository interface {
    // ... existing methods ...

    // GetEPSSStats returns aggregate EPSS statistics.
    GetEPSSStats(ctx context.Context) (*EPSSStats, error)
}

// EPSSStats aggregates EPSS scoring data.
type EPSSStats struct {
    TotalScored   int64       `json:"total_scored"`    // CVEs with epss != NULL
    AvgEPSS       float64     `json:"avg_epss"`
    HighRiskCount int64       `json:"high_risk_count"` // epss >= 0.9
    TopCVEs       []EPSSEntry `json:"top_cves"`        // top 10 by epss
    UpdatedAt     time.Time   `json:"updated_at"`
}

type EPSSEntry struct {
    CVEID          string  `json:"cve_id"`
    EPSS           float64 `json:"epss"`
    EPSSPercentile float64 `json:"epss_percentile"`
    Severity       string  `json:"severity"`
}
```

### Repository Implementation

Tìm PostgreSQL implementation của CVE repository, thêm method:

```go
func (r *pgCVERepository) GetEPSSStats(ctx context.Context) (*repository.EPSSStats, error) {
    var stats repository.EPSSStats

    // Total scored
    row := r.db.QueryRowContext(ctx, `
        SELECT COUNT(*), COALESCE(AVG(epss), 0)
        FROM cves
        WHERE epss IS NOT NULL
    `)
    if err := row.Scan(&stats.TotalScored, &stats.AvgEPSS); err != nil {
        return nil, err
    }

    // High risk count
    row = r.db.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM cves WHERE epss >= 0.9
    `)
    if err := row.Scan(&stats.HighRiskCount); err != nil {
        return nil, err
    }

    // Top 10 CVEs by EPSS
    rows, err := r.db.QueryContext(ctx, `
        SELECT id, epss, COALESCE(epss_percentile, 0), COALESCE(severity, 'UNKNOWN')
        FROM cves
        WHERE epss IS NOT NULL
        ORDER BY epss DESC
        LIMIT 10
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    for rows.Next() {
        var e repository.EPSSEntry
        if err := rows.Scan(&e.CVEID, &e.EPSS, &e.EPSSPercentile, &e.Severity); err != nil {
            continue
        }
        stats.TopCVEs = append(stats.TopCVEs, e)
    }

    stats.UpdatedAt = time.Now()
    return &stats, nil
}
```

## Acceptance Criteria

- [x] `GET /api/v2/epss/stats` → 200 với JSON: `total_scored`, `avg_epss`, `high_risk_count`, `top_cves`
- [x] `total_scored` > 0 nếu EPSS đã được sync
- [x] `top_cves` là mảng 10 entries, sorted by `epss` descending
- [x] Endpoint được cache 30 phút bởi gateway (gateway config — không cần implement ở service)
- [x] Response time < 500ms (dùng indexed query)
- [x] `go build ./...` pass không lỗi
