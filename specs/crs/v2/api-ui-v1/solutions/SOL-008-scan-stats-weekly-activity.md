# SOL-008: Scan Stats & Weekly Activity Endpoints

> **CR:** [CR-008](../CR-008-scan-stats-weekly-activity.md)  
> **Priority:** 🔴 HIGH (Phase 2)  
> **Service(s):** `scan-service` (`:8084`)  
> **Tạo:** 2026-06-19  
> **Cập nhật:** 2026-06-22  
> **Trạng thái:** ❌ PENDING — Chưa implement (xác nhận bởi API test 2026-06-22)

---

## 1. Tóm tắt Giải pháp

Thêm 2 endpoints mới vào `scan-service` + gateway routing:

| Endpoint | Handler | Cache |
|---|---|---|
| `GET /api/v1/scans/stats` | `GetStats` | Redis TTL 30s |
| `GET /api/v1/scans/stats/weekly` | `GetWeeklyActivity` | Redis TTL 5m |
| `GET /api/v1/scans` (cập nhật) | Thêm `stats` block | Tái dùng `GetStats` |

**Kiến trúc:** Thêm vào layer `delivery/http` của scan-service. Repository queries thuần SQL không random.

---

## 2. Repository Layer

### File: `services/scan-service/internal/infra/postgres/scan_stats_repo.go`

```go
package postgres

import (
    "context"
    "time"
)

// ScanStatsRepository — queries cho stats endpoints
type ScanStatsRepository struct {
    db *pgxpool.Pool
}

// CountByStatus đếm scans theo status (running, pending, completed, ...)
func (r *ScanStatsRepository) CountByStatus(ctx context.Context, status string) (int, error) {
    var count int
    err := r.db.QueryRow(ctx,
        `SELECT COUNT(*) FROM scans WHERE status = $1`,
        status,
    ).Scan(&count)
    return count, err
}

// CountCompletedToday đếm scans hoàn thành trong ngày hôm nay (UTC)
func (r *ScanStatsRepository) CountCompletedToday(ctx context.Context) (int, error) {
    var count int
    err := r.db.QueryRow(ctx, `
        SELECT COUNT(*) FROM scans
        WHERE status = 'completed'
          AND completed_at >= CURRENT_DATE
          AND completed_at <  CURRENT_DATE + INTERVAL '1 day'
    `).Scan(&count)
    return count, err
}

// CountFindingsToday đếm tổng findings từ các scans hoàn thành hôm nay
func (r *ScanStatsRepository) CountFindingsToday(ctx context.Context) (int, error) {
    var count int
    err := r.db.QueryRow(ctx, `
        SELECT COALESCE(SUM(finding_count), 0)
        FROM scans
        WHERE status = 'completed'
          AND completed_at >= CURRENT_DATE
          AND completed_at <  CURRENT_DATE + INTERVAL '1 day'
    `).Scan(&count)
    return count, err
}

// CountScheduledActive đếm scheduled scans đang active (chưa expired)
func (r *ScanStatsRepository) CountScheduledActive(ctx context.Context) (int, error) {
    var count int
    err := r.db.QueryRow(ctx, `
        SELECT COUNT(*) FROM scheduled_scans
        WHERE enabled = true
          AND (next_run_at IS NULL OR next_run_at >= NOW())
    `).Scan(&count)
    return count, err
}

// WeeklyActivityRow đại diện một ngày trong 7 ngày qua
type WeeklyActivityRow struct {
    Day      string // "Mon", "Tue", ..., "Sun"
    Scans    int
    Findings int
}

// GetWeeklyActivity trả về 7 ngày gần nhất (Mon-Sun ordering theo tuần hiện tại)
func (r *ScanStatsRepository) GetWeeklyActivity(ctx context.Context) ([]WeeklyActivityRow, error) {
    rows, err := r.db.Query(ctx, `
        SELECT
            to_char(date_trunc('day', started_at AT TIME ZONE 'UTC'), 'Dy') AS day,
            COUNT(*) FILTER (WHERE status = 'completed')                     AS scans,
            COALESCE(SUM(finding_count), 0)                                  AS findings
        FROM scans
        WHERE started_at >= NOW() - INTERVAL '7 days'
        GROUP BY date_trunc('day', started_at AT TIME ZONE 'UTC')
        ORDER BY date_trunc('day', started_at AT TIME ZONE 'UTC')
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    dbData := map[string]WeeklyActivityRow{}
    for rows.Next() {
        var row WeeklyActivityRow
        if err := rows.Scan(&row.Day, &row.Scans, &row.Findings); err != nil {
            return nil, err
        }
        dbData[row.Day] = row
    }

    // Đảm bảo luôn có đủ 7 ngày, kể cả ngày không có data
    result := make([]WeeklyActivityRow, 7)
    now := time.Now().UTC()
    for i := 0; i < 7; i++ {
        day := now.AddDate(0, 0, -(6 - i))
        dayStr := day.Format("Mon")
        if row, ok := dbData[dayStr]; ok {
            result[i] = row
        } else {
            result[i] = WeeklyActivityRow{Day: dayStr, Scans: 0, Findings: 0}
        }
    }
    return result, nil
}
```

---

## 3. Handler Layer

### File: `services/scan-service/internal/delivery/http/stats_handler.go`

```go
package http

import (
    "context"
    "encoding/json"
    "net/http"
    "time"

    "github.com/redis/go-redis/v9"
)

// ScanStats — response schema cho GET /api/v1/scans/stats
type ScanStats struct {
    ActiveScans    int `json:"active_scans"`
    CompletedToday int `json:"completed_today"`
    TotalFindings  int `json:"total_findings"`
    ScheduledScans int `json:"scheduled_scans"`
}

// WeeklyActivity — response schema cho 1 phần tử trong /api/v1/scans/stats/weekly
type WeeklyActivity struct {
    Day      string `json:"day"`      // "Mon", "Tue", ..., "Sun"
    Scans    int    `json:"scans"`
    Findings int    `json:"findings"`
}

type StatsHandler struct {
    repo  ScanStatsRepository
    cache *redis.Client
}

// GetStats godoc
// GET /api/v1/scans/stats
// Requires: Bearer token
// Cache: Redis TTL 30s
func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    cacheKey := "scan:stats"

    // 1. Check cache
    if cached, err := h.cache.Get(ctx, cacheKey).Bytes(); err == nil {
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("X-Cache", "HIT")
        w.Write(cached)
        return
    }

    // 2. Query DB
    stats, err := h.computeStats(ctx)
    if err != nil {
        jsonError(w, 500, "INTERNAL", "Failed to compute stats")
        return
    }

    // 3. Cache 30 giây
    data, _ := json.Marshal(stats)
    h.cache.Set(ctx, cacheKey, data, 30*time.Second)

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Cache", "MISS")
    w.Write(data)
}

// GetWeeklyActivity godoc
// GET /api/v1/scans/stats/weekly
// Requires: Bearer token
// Cache: Redis TTL 5m
func (h *StatsHandler) GetWeeklyActivity(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    cacheKey := "scan:stats:weekly"

    // 1. Check cache
    if cached, err := h.cache.Get(ctx, cacheKey).Bytes(); err == nil {
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("X-Cache", "HIT")
        w.Write(cached)
        return
    }

    // 2. Query DB — 7 ngày, tất cả từ DB không random
    rows, err := h.repo.GetWeeklyActivity(ctx)
    if err != nil {
        jsonError(w, 500, "INTERNAL", "Failed to fetch weekly activity")
        return
    }

    // 3. Map sang response type
    result := make([]WeeklyActivity, len(rows))
    for i, row := range rows {
        result[i] = WeeklyActivity{
            Day:      row.Day,
            Scans:    row.Scans,
            Findings: row.Findings,
        }
    }

    // 4. Cache 5 phút
    data, _ := json.Marshal(result)
    h.cache.Set(ctx, cacheKey, data, 5*time.Minute)

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Cache", "MISS")
    w.Write(data)
}

// computeStats tổng hợp KPI từ DB
func (h *StatsHandler) computeStats(ctx context.Context) (*ScanStats, error) {
    active, err := h.repo.CountByStatus(ctx, "running")
    if err != nil {
        return nil, err
    }
    completed, err := h.repo.CountCompletedToday(ctx)
    if err != nil {
        return nil, err
    }
    findings, err := h.repo.CountFindingsToday(ctx)
    if err != nil {
        return nil, err
    }
    scheduled, err := h.repo.CountScheduledActive(ctx)
    if err != nil {
        return nil, err
    }

    return &ScanStats{
        ActiveScans:    active,
        CompletedToday: completed,
        TotalFindings:  findings,
        ScheduledScans: scheduled,
    }, nil
}
```

---

## 4. Cập nhật ScansListResponse

### File: `services/scan-service/internal/delivery/http/scan_handler.go`

```go
// ScansListResponse — cập nhật thêm stats block
type ScansListResponse struct {
    Scans    []ScanDTO `json:"scans"`
    Total    int       `json:"total"`
    Page     int       `json:"page"`
    PageSize int       `json:"page_size"`
    Stats    *ScanStats `json:"stats,omitempty"` // ← MỚI — backward compatible
}

// ListScans — cập nhật để include stats
func (h *ScanHandler) ListScans(w http.ResponseWriter, r *http.Request) {
    // ... existing list logic ...

    // Thêm stats vào response (tái dùng computeStats)
    stats, _ := h.statsHandler.computeStats(r.Context())

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(ScansListResponse{
        Scans:    scans,
        Total:    total,
        Page:     page,
        PageSize: pageSize,
        Stats:    stats, // Graceful nil nếu stats query fail
    })
}
```

---

## 5. Gateway Routing

### File: `apps/osv/internal/gateway/router.go`

```go
// ⚠️ CRITICAL: /scans/stats/weekly phải đăng ký TRƯỚC /scans/stats
// để tránh "weekly" bị parse là path variable của /scans/stats/{id}

// Scan Stats endpoints (KHÔNG có {id} pattern conflict)
mux.Handle("GET /api/v1/scans/stats/weekly",
    protected(proxy.Forward("scan-service:8084")))

mux.Handle("GET /api/v1/scans/stats",
    protected(proxy.Forward("scan-service:8084")))

// Note: Go 1.22 ServeMux dùng longest-match routing nên ordering ít
// quan trọng hơn, nhưng vẫn nên đăng ký theo thứ tự specific → generic
```

> **Lưu ý port:** CR-008 ghi `scan-service:8083` nhưng theo architecture spec port `8083` là `search-service`. Port đúng của `scan-service` là **`:8084`**. Đây là lỗi typo trong CR, cần dùng `:8084`.

---

## 6. DB Schema Requirements

Đảm bảo bảng `scans` có các columns được dùng trong queries:

```sql
-- Kiểm tra columns cần thiết
SELECT column_name, data_type
FROM information_schema.columns
WHERE table_name = 'scans'
  AND column_name IN ('status', 'started_at', 'completed_at', 'finding_count');

-- Nếu finding_count chưa có:
ALTER TABLE scans ADD COLUMN IF NOT EXISTS finding_count INTEGER NOT NULL DEFAULT 0;

-- Index tối ưu cho stats queries
CREATE INDEX IF NOT EXISTS idx_scans_status ON scans(status);
CREATE INDEX IF NOT EXISTS idx_scans_completed_at ON scans(completed_at DESC)
    WHERE status = 'completed';
CREATE INDEX IF NOT EXISTS idx_scans_started_at ON scans(started_at DESC);
```

---

## 7. Cache Invalidation

Khi một scan hoàn thành (status → `completed`), invalidate cache:

```go
// services/scan-service/internal/usecase/scan_usecase.go
func (uc *ScanUseCase) CompleteScan(ctx context.Context, scanID uuid.UUID, result ScanResult) error {
    // ... update scan status ...

    // Invalidate stats cache
    uc.cache.Del(ctx, "scan:stats", "scan:stats:weekly")

    // Publish NATS event
    uc.nats.Publish("scan.scan.completed", ...)
    return nil
}
```

---

## 8. Tests

```go
// services/scan-service/internal/delivery/http/stats_handler_test.go

func TestGetStats_ReturnsRealData(t *testing.T) {
    // Seed: 2 running scans, 3 completed today
    // Assert: active_scans=2, completed_today=3
    // Assert: NO random values
}

func TestGetWeeklyActivity_Always7Items(t *testing.T) {
    resp := GET("/api/v1/scans/stats/weekly")
    var result []WeeklyActivity
    json.Unmarshal(resp.Body, &result)

    assert.Len(t, result, 7)
    assert.Equal(t, "Mon", result[0].Day) // tuần hiện tại
    assert.Equal(t, "Sun", result[6].Day)
}

func TestGetWeeklyActivity_ZeroForDaysWithNoData(t *testing.T) {
    // Clear all scans data
    // All 7 items phải có scans=0, findings=0 (không bị skip)
}

func TestStats_RequiresAuth(t *testing.T) {
    resp := GET("/api/v1/scans/stats") // no token
    assert.Equal(t, 401, resp.StatusCode)
}
```

---

## 9. Acceptance Criteria Checklist

> **Kết quả API Test (2026-06-22):**
> - `GET /api/v1/scans/stats` → ❌ **404** (chưa implement)
> - `GET /api/v1/scans/stats/weekly` → ❌ **404** (chưa implement)
> - `GET /api/v1/scans` → ✅ 200 nhưng ⚠️ thiếu `stats` field

- [ ] `GET /api/v1/scans/stats` → HTTP 200, `ScanStats` object
- [ ] Response time < 200ms (cache hit)
- [ ] Không có random data — tất cả từ DB
- [ ] `GET /api/v1/scans/stats/weekly` → HTTP 200, array đúng 7 items
- [ ] `day` values: `"Mon"`, `"Tue"`, `"Wed"`, `"Thu"`, `"Fri"`, `"Sat"`, `"Sun"`
- [ ] Route `/stats/weekly` không conflict với `/stats`
- [ ] `GET /api/v1/scans` response có `stats` field (backward compatible)
- [ ] Cả 2 endpoints require Bearer token — trả `401` nếu không có
- [ ] `active_scans` = số scans đang `status = 'running'` thực tế
