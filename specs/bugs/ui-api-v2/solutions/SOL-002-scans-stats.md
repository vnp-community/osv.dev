# SOL-002 — Scan Dashboard: Thêm Stats Endpoints (P2)

**Bug**: [BUG-002](../BUG-002-scans.md)  
**Service**: `scan-service` (`services/scan-service`)  
**Endpoints**: `GET /api/v1/scans/stats`, `GET /api/v1/scans/stats/weekly`  
**HTTP Error**: `404 Not Found`

**Status**: `✅ Implemented` — via [TASK-008](../../tasks/TASK-008-*.md)

---

## Root Cause

Routes đã đăng ký trong [`router.go:178`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go#L178):

```go
mux.Handle("GET /api/v1/scans/stats/weekly", protected(proxy.Forward("scan-service:8084")))
mux.Handle("GET /api/v1/scans/stats", protected(proxy.Forward("scan-service:8084")))
```

**Vấn đề**: Route đã được đăng ký ở gateway, nhưng **scan-service chưa implement** handler cho các endpoint này.

---

## Giải pháp — Implement trong scan-service

### Bước 1: Tìm router của scan-service

```bash
find services/scan-service/internal/delivery/http -name "router.go" -o -name "*handler*"
```

### Bước 2: Thêm Stats Handler

```go
// services/scan-service/internal/delivery/http/stats_handler.go (mới)

package http

import (
    "net/http"
    "time"
)

type StatsHandler struct {
    repo ScanRepository
    log  zerolog.Logger
}

// HandleStats — GET /api/v1/scans/stats
func (h *StatsHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    stats, err := h.repo.GetScanStats(ctx)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to get scan stats")
        return
    }
    
    // Defensive: đảm bảo không nil
    if stats == nil {
        stats = &ScanStats{}
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "total":      stats.Total,
        "running":    stats.Running,
        "completed":  stats.Completed,
        "failed":     stats.Failed,
        "pending":    stats.Pending,
        "cancelled":  stats.Cancelled,
    })
}

// HandleWeeklyStats — GET /api/v1/scans/stats/weekly
func (h *StatsHandler) HandleWeeklyStats(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Lấy 7 ngày gần nhất
    weekly, err := h.repo.GetWeeklyStats(ctx, 7)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to get weekly stats")
        return
    }
    
    // Defensive: đảm bảo không nil
    if weekly == nil {
        weekly = make([]DailyStats, 0)
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data": weekly,  // []DailyStats
    })
}

// ── Response types ──────────────────────────────────────────────────────────

type ScanStats struct {
    Total     int `json:"total"`
    Running   int `json:"running"`
    Completed int `json:"completed"`
    Failed    int `json:"failed"`
    Pending   int `json:"pending"`
    Cancelled int `json:"cancelled"`
}

type DailyStats struct {
    Date      string `json:"date"`       // "2026-06-19"
    Total     int    `json:"total"`
    Completed int    `json:"completed"`
    Failed    int    `json:"failed"`
}
```

### Bước 3: Implement Repository Queries

```go
// services/scan-service/internal/infra/postgres/scan_repo.go

func (r *ScanRepo) GetScanStats(ctx context.Context) (*ScanStats, error) {
    var stats ScanStats
    err := r.pool.QueryRow(ctx, `
        SELECT
            COUNT(*)                                              AS total,
            COUNT(*) FILTER (WHERE status = 'running')           AS running,
            COUNT(*) FILTER (WHERE status = 'completed')         AS completed,
            COUNT(*) FILTER (WHERE status = 'failed')            AS failed,
            COUNT(*) FILTER (WHERE status = 'pending')           AS pending,
            COUNT(*) FILTER (WHERE status = 'cancelled')         AS cancelled
        FROM osv_scan.scans
    `).Scan(
        &stats.Total, &stats.Running, &stats.Completed,
        &stats.Failed, &stats.Pending, &stats.Cancelled,
    )
    return &stats, err
}

func (r *ScanRepo) GetWeeklyStats(ctx context.Context, days int) ([]DailyStats, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT
            TO_CHAR(created_at::date, 'YYYY-MM-DD')              AS date,
            COUNT(*)                                              AS total,
            COUNT(*) FILTER (WHERE status = 'completed')         AS completed,
            COUNT(*) FILTER (WHERE status = 'failed')            AS failed
        FROM osv_scan.scans
        WHERE created_at >= NOW() - ($1 || ' days')::interval
        GROUP BY created_at::date
        ORDER BY date ASC
    `, days)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    // FIX: make([]T, 0) — never nil
    result := make([]DailyStats, 0)
    for rows.Next() {
        var d DailyStats
        if err := rows.Scan(&d.Date, &d.Total, &d.Completed, &d.Failed); err != nil {
            return nil, err
        }
        result = append(result, d)
    }
    return result, rows.Err()
}
```

### Bước 4: Register routes trong scan-service router

```go
// services/scan-service/internal/delivery/http/router.go

// Literal paths BEFORE wildcard {id}
r.Get("/api/v1/scans/stats/weekly", statsHandler.HandleWeeklyStats)  // TRƯỚC
r.Get("/api/v1/scans/stats", statsHandler.HandleStats)               // TRƯỚC
r.Get("/api/v1/scans/{id}", scanHandler.Get)                         // SAU
```

---

## Response Schema

```json
// GET /api/v1/scans/stats
{
  "total": 42,
  "running": 2,
  "completed": 35,
  "failed": 3,
  "pending": 1,
  "cancelled": 1
}

// GET /api/v1/scans/stats/weekly
{
  "data": [
    { "date": "2026-06-14", "total": 5, "completed": 4, "failed": 1 },
    { "date": "2026-06-15", "total": 8, "completed": 8, "failed": 0 }
  ]
}
```

---

## Verification

```bash
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/scans/stats" | jq '.total'
# Expected: number (0 hoặc cao hơn)

curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/scans/stats/weekly" | jq '.data | type'
# Expected: "array"
```
