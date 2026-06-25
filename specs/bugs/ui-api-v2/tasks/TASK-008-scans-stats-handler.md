# TASK-008 — scan-service: Implement Stats Handlers

**Bug**: [BUG-002](../BUG-002-scans.md)  
**Solution**: [SOL-002](../solutions/SOL-002-scans-stats.md)  
**Priority**: 🟡 P2  
**Effort**: ~30 phút  
**Status**: `[x] DONE`

---

## Mô tả

`GET /api/v1/scans/stats` và `GET /api/v1/scans/stats/weekly` trả `404`. Gateway đã đăng ký routes nhưng scan-service chưa implement handlers.

---

## File cần sửa / tạo

**File mới**: `services/scan-service/internal/delivery/http/stats_handler.go`  
**File sửa**: `services/scan-service/internal/delivery/http/router.go` (hoặc tương đương)

---

## Thay đổi 1 — Tạo StatsHandler

**Tạo file mới** `services/scan-service/internal/delivery/http/stats_handler.go`:

```go
package http

import (
    "net/http"
    "time"

    "github.com/rs/zerolog"
)

// ScanStats — overall stats counts
type ScanStats struct {
    Total     int `json:"total"`
    Running   int `json:"running"`
    Completed int `json:"completed"`
    Failed    int `json:"failed"`
    Pending   int `json:"pending"`
    Cancelled int `json:"cancelled"`
}

// DailyStats — daily breakdown
type DailyStats struct {
    Date      string `json:"date"`       // "2026-06-19"
    Total     int    `json:"total"`
    Completed int    `json:"completed"`
    Failed    int    `json:"failed"`
}

// StatsHandler handles scan statistics endpoints.
type StatsHandler struct {
    repo ScanStatsRepository
    log  zerolog.Logger
}

// ScanStatsRepository is the interface for stats queries.
type ScanStatsRepository interface {
    GetScanStats(ctx context.Context) (*ScanStats, error)
    GetWeeklyStats(ctx context.Context, days int) ([]DailyStats, error)
}

func NewStatsHandler(repo ScanStatsRepository, log zerolog.Logger) *StatsHandler {
    return &StatsHandler{repo: repo, log: log}
}

// HandleStats — GET /api/v1/scans/stats
func (h *StatsHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
    stats, err := h.repo.GetScanStats(r.Context())
    if err != nil {
        h.log.Error().Err(err).Msg("StatsHandler.HandleStats")
        respondError(w, http.StatusInternalServerError, "failed to get scan stats")
        return
    }
    if stats == nil {
        stats = &ScanStats{}
    }
    respondJSON(w, http.StatusOK, stats)
}

// HandleWeeklyStats — GET /api/v1/scans/stats/weekly
func (h *StatsHandler) HandleWeeklyStats(w http.ResponseWriter, r *http.Request) {
    weekly, err := h.repo.GetWeeklyStats(r.Context(), 7)
    if err != nil {
        h.log.Error().Err(err).Msg("StatsHandler.HandleWeeklyStats")
        respondError(w, http.StatusInternalServerError, "failed to get weekly stats")
        return
    }
    if weekly == nil {
        weekly = make([]DailyStats, 0)
    }
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data": weekly,
    })
}
```

---

## Thay đổi 2 — Implement Repository Queries

**Tìm** scan repository trong `services/scan-service/internal/infra/postgres/` và **thêm** methods:

```go
func (r *ScanRepo) GetScanStats(ctx context.Context) (*ScanStats, error) {
    var stats ScanStats
    err := r.pool.QueryRow(ctx, `
        SELECT
            COUNT(*)                                          AS total,
            COUNT(*) FILTER (WHERE status = 'running')       AS running,
            COUNT(*) FILTER (WHERE status = 'completed')     AS completed,
            COUNT(*) FILTER (WHERE status = 'failed')        AS failed,
            COUNT(*) FILTER (WHERE status = 'pending')       AS pending,
            COUNT(*) FILTER (WHERE status = 'cancelled')     AS cancelled
        FROM osv_scan.scans
    `).Scan(&stats.Total, &stats.Running, &stats.Completed,
            &stats.Failed, &stats.Pending, &stats.Cancelled)
    return &stats, err
}

func (r *ScanRepo) GetWeeklyStats(ctx context.Context, days int) ([]DailyStats, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT
            created_at::date::text                           AS date,
            COUNT(*)                                         AS total,
            COUNT(*) FILTER (WHERE status = 'completed')    AS completed,
            COUNT(*) FILTER (WHERE status = 'failed')       AS failed
        FROM osv_scan.scans
        WHERE created_at >= NOW() - make_interval(days => $1)
        GROUP BY created_at::date
        ORDER BY date ASC
    `, days)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

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

---

## Thay đổi 3 — Register routes trong scan-service router

**Tìm** scan-service router và **thêm routes TRƯỚC wildcard `/{id}`**:

```go
// Literal paths BEFORE /{id}
r.Get("/api/v1/scans/stats/weekly", statsHandler.HandleWeeklyStats)  // TRƯỚC
r.Get("/api/v1/scans/stats", statsHandler.HandleStats)               // TRƯỚC
r.Get("/api/v1/scans/{id}", scanHandler.Get)                         // SAU
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/scans/stats` trả `200` với object có keys `total`, `running`, `completed`, v.v.
- [ ] `GET /api/v1/scans/stats/weekly` trả `200` với `{"data": [...]}`
- [ ] `data` trong weekly stats luôn là array (kể cả `[]`)
- [ ] `go build ./...` trong scan-service không có lỗi

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service
go build ./...

curl -s -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/scans/stats" | jq '{total, running, completed}'
# Expected: {"total": N, "running": N, "completed": N}

curl -s -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/scans/stats/weekly" | jq '.data | type'
# Expected: "array"
```
