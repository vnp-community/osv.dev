# TASK-CR008-P1-03 — Scan Stats & Weekly Activity Endpoints

**Phase:** Phase 2 — Blocking UI  
**Nguồn giải pháp:** [`solutions/SOL-008`](../solutions/SOL-008-scan-stats-weekly-activity.md)  
**Ưu tiên:** 🔴 P1 — ScanDashboard crash vì fallback về `Math.random()`  
**Phụ thuộc:** Không có  
**Status:** ✅ **DONE** — 2026-06-19  

---

## Mục tiêu

Thêm 2 endpoints mới vào `scan-service:8084`:
- `GET /api/v1/scans/stats` — KPI cards (active, completed today, findings, scheduled)
- `GET /api/v1/scans/stats/weekly` — Weekly bar chart (7 days, scans + findings per day)

---

## Điều tra trước khi code

```bash
# 1. Xem cấu trúc scan-service
ls services/scan-service/internal/delivery/http/
ls services/scan-service/internal/infra/

# 2. Kiểm tra table scans có columns cần thiết
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "SELECT column_name, data_type FROM information_schema.columns \
      WHERE table_name = 'scans' \
      AND column_name IN ('status','started_at','completed_at','finding_count');"

# 3. Kiểm tra table scheduled_scans
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "\d scheduled_scans" 2>/dev/null || echo "scheduled_scans NOT FOUND"

# 4. Xem router scan-service
grep -rn "Handle\|Get\|Post\|r\.Get" \
  services/scan-service/ --include="*.go" | grep -i "stats\|weekly"
```

---

## Bước 1: DB Check & Fix

```bash
# Kiểm tra finding_count column
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "SELECT column_name FROM information_schema.columns \
      WHERE table_name='scans' AND column_name='finding_count';"
```

Nếu thiếu `finding_count`:
```sql
ALTER TABLE scans ADD COLUMN IF NOT EXISTS finding_count INTEGER NOT NULL DEFAULT 0;

-- Indexes tối ưu
CREATE INDEX IF NOT EXISTS idx_scans_status ON scans(status);
CREATE INDEX IF NOT EXISTS idx_scans_completed_at ON scans(completed_at DESC)
    WHERE status = 'completed';
CREATE INDEX IF NOT EXISTS idx_scans_started_at ON scans(started_at DESC);
```

---

## Bước 2: Repository Methods

**Tìm repo hiện tại:**
```bash
grep -r "ScanRepository\|scanRepo\|CountByStatus" \
  services/scan-service/internal/ --include="*.go" -l
```

**Thêm vào file repo hiện có** (hoặc tạo `stats_repo.go`):

```go
// CountByStatus — số scans theo status
func (r *ScanRepo) CountByStatus(ctx context.Context, status string) (int, error) {
    var count int
    err := r.db.QueryRow(ctx,
        `SELECT COUNT(*) FROM scans WHERE status = $1`, status,
    ).Scan(&count)
    return count, err
}

// CountCompletedToday — scans hoàn thành hôm nay UTC
func (r *ScanRepo) CountCompletedToday(ctx context.Context) (int, error) {
    var count int
    err := r.db.QueryRow(ctx, `
        SELECT COUNT(*) FROM scans
        WHERE status = 'completed'
          AND completed_at >= CURRENT_DATE
          AND completed_at <  CURRENT_DATE + INTERVAL '1 day'
    `).Scan(&count)
    return count, err
}

// CountFindingsToday — tổng findings từ scans completed hôm nay
func (r *ScanRepo) CountFindingsToday(ctx context.Context) (int, error) {
    var count int
    err := r.db.QueryRow(ctx, `
        SELECT COALESCE(SUM(finding_count), 0) FROM scans
        WHERE status = 'completed'
          AND completed_at >= CURRENT_DATE
          AND completed_at <  CURRENT_DATE + INTERVAL '1 day'
    `).Scan(&count)
    return count, err
}

// CountScheduledActive — scheduled scans active
func (r *ScanRepo) CountScheduledActive(ctx context.Context) (int, error) {
    // Thử scheduled_scans table trước
    var count int
    err := r.db.QueryRow(ctx, `
        SELECT COUNT(*) FROM scheduled_scans
        WHERE enabled = true
    `).Scan(&count)
    if err != nil {
        // Fallback: count pending scans
        r.db.QueryRow(ctx,
            `SELECT COUNT(*) FROM scans WHERE status = 'pending'`,
        ).Scan(&count)
    }
    return count, nil
}

// WeeklyActivityRow — 1 ngày trong weekly chart
type WeeklyActivityRow struct {
    Day      string
    Scans    int
    Findings int
}

// GetWeeklyActivity — 7 ngày, đảm bảo luôn có 7 items
func (r *ScanRepo) GetWeeklyActivity(ctx context.Context) ([]WeeklyActivityRow, error) {
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
        rows.Scan(&row.Day, &row.Scans, &row.Findings)
        dbData[row.Day] = row
    }

    // Đảm bảo luôn 7 items kể cả ngày không có data
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

## Bước 3: Handler

**Tìm handler file:**
```bash
ls services/scan-service/internal/delivery/http/
# Tìm file handler scans
grep -rn "func.*Handler.*Get\|ListScans" \
  services/scan-service/internal/delivery/http/ --include="*.go"
```

**Thêm vào handler file hiện có** hoặc tạo `stats_handler.go`:

```go
// ScanStats response type
type ScanStats struct {
    ActiveScans    int `json:"active_scans"`
    CompletedToday int `json:"completed_today"`
    TotalFindings  int `json:"total_findings"`
    ScheduledScans int `json:"scheduled_scans"`
}

type WeeklyActivity struct {
    Day      string `json:"day"`
    Scans    int    `json:"scans"`
    Findings int    `json:"findings"`
}

// GET /api/v1/scans/stats
func (h *ScanHandler) GetStats(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Check Redis cache
    if h.cache != nil {
        if cached, err := h.cache.Get(ctx, "scan:stats").Bytes(); err == nil {
            w.Header().Set("Content-Type", "application/json")
            w.Header().Set("X-Cache", "HIT")
            w.Write(cached)
            return
        }
    }

    active, _    := h.repo.CountByStatus(ctx, "running")
    completed, _ := h.repo.CountCompletedToday(ctx)
    findings, _  := h.repo.CountFindingsToday(ctx)
    scheduled, _ := h.repo.CountScheduledActive(ctx)

    stats := ScanStats{
        ActiveScans:    active,
        CompletedToday: completed,
        TotalFindings:  findings,
        ScheduledScans: scheduled,
    }

    data, _ := json.Marshal(stats)
    if h.cache != nil {
        h.cache.Set(ctx, "scan:stats", data, 30*time.Second)
    }

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Cache", "MISS")
    w.Write(data)
}

// GET /api/v1/scans/stats/weekly
func (h *ScanHandler) GetWeeklyActivity(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    if h.cache != nil {
        if cached, err := h.cache.Get(ctx, "scan:stats:weekly").Bytes(); err == nil {
            w.Header().Set("Content-Type", "application/json")
            w.Header().Set("X-Cache", "HIT")
            w.Write(cached)
            return
        }
    }

    rows, err := h.repo.GetWeeklyActivity(ctx)
    if err != nil {
        respondError(w, 500, "failed to fetch weekly activity")
        return
    }

    result := make([]WeeklyActivity, len(rows))
    for i, row := range rows {
        result[i] = WeeklyActivity{Day: row.Day, Scans: row.Scans, Findings: row.Findings}
    }

    data, _ := json.Marshal(result)
    if h.cache != nil {
        h.cache.Set(ctx, "scan:stats:weekly", data, 5*time.Minute)
    }

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Cache", "MISS")
    w.Write(data)
}
```

---

## Bước 4: Register Routes trong scan-service

```bash
# Tìm router file
grep -rn "Handle\|r\.Get\|mux\." \
  services/scan-service/ --include="*.go" | grep -v "_test" | head -20
```

Thêm routes (THỨ TỰ QUAN TRỌNG — `/stats/weekly` phải TRƯỚC `/stats`):

```go
// THÊM vào router — thứ tự: literal paths trước wildcard
mux.Handle("GET /api/v1/scans/stats/weekly", protected(h.GetWeeklyActivity))
mux.Handle("GET /api/v1/scans/stats",         protected(h.GetStats))
// Routes hiện có vẫn giữ nguyên
```

---

## Bước 5: Register Routes trong Gateway

```bash
# Kiểm tra gateway đã có routes chưa
grep -n "scans/stats" apps/osv/internal/gateway/router.go
```

Nếu chưa có:
```go
// apps/osv/internal/gateway/router.go
// THÊM vào — /stats/weekly TRƯỚC /stats
mux.Handle("GET /api/v1/scans/stats/weekly",
    protected(proxy.Forward("scan-service:8084")))
mux.Handle("GET /api/v1/scans/stats",
    protected(proxy.Forward("scan-service:8084")))
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/scans/stats` → HTTP 200, `{ active_scans, completed_today, total_findings, scheduled_scans }`
- [ ] Không có random data — tất cả từ DB
- [ ] `GET /api/v1/scans/stats/weekly` → HTTP 200, array đúng **7 items**
- [ ] `day` values: `"Mon"`, `"Tue"`, `"Wed"`, `"Thu"`, `"Fri"`, `"Sat"`, `"Sun"`
- [ ] Ngày không có data: `scans=0, findings=0` (không bị bỏ qua)
- [ ] Cả 2 endpoints trả `401` nếu không có token

## Verification

```bash
TOKEN="<your-token>"

# KPI Stats
curl -s https://c12.openledger.vn/api/v1/scans/stats \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: { "active_scans": N, "completed_today": N, ... }

# Weekly
curl -s https://c12.openledger.vn/api/v1/scans/stats/weekly \
  -H "Authorization: Bearer $TOKEN" | jq 'length, .[0]'
# Expected: 7, { "day": "Mon/Tue/...", "scans": N, "findings": N }

# Route ordering check — /stats/weekly không bị 404
curl -s https://c12.openledger.vn/api/v1/scans/stats/weekly \
  -H "Authorization: Bearer $TOKEN" -o /dev/null -w "%{http_code}"
# Expected: 200 (không phải 404)
```
