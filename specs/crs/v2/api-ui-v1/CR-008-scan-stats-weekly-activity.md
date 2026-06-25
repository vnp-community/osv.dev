# Change Request 008: Scan Stats & Weekly Activity Endpoints

**Tạo:** 2026-06-19  
**Status:** New — 2 endpoints hoàn toàn thiếu trong backend.  
**Nguồn:** openapi.yaml schemas `ScanStats`, `WeeklyActivity`  
**Target directory:** `specs/crs/v2/api-ui-v1/`

---

## 1. Bối cảnh

Frontend `ScanDashboard.tsx` (sau khi fix hardcode) gọi 2 endpoints mới để lấy:
1. KPI cards (active scans, completed today, total findings, scheduled)  
2. Weekly activity bar chart (Mon–Sun, scans vs findings)

Hiện tại cả 2 endpoint đều **không tồn tại** → frontend fallback về `Math.random()` hardcode.

| Endpoint cần thêm | Path | Service | Trạng thái |
|---|---|---|---|
| Scan KPI Stats | `GET /api/v1/scans/stats` | scan-service | ❌ THIẾU |
| Weekly Activity | `GET /api/v1/scans/stats/weekly` | scan-service | ❌ THIẾU |

---

## 2. Chi tiết Thay đổi

### 2.1 [HIGH] `GET /api/v1/scans/stats` — Scan KPI Dashboard

**Gateway routing** — thêm vào `apps/osv/internal/gateway/router.go`:

```go
// Scan Dashboard Stats
mux.Handle("GET /api/v1/scans/stats",
    protected(proxy.Forward("scan-service:8083")))
mux.Handle("GET /api/v1/scans/stats/weekly",
    protected(proxy.Forward("scan-service:8083")))
```

**Response schema `ScanStats`:**
```json
{
  "active_scans": 3,
  "completed_today": 12,
  "total_findings": 47,
  "scheduled_scans": 5
}
```

**Scan-service handler implementation (Go):**
```go
// services/scan-service/internal/handler/stats.go
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    active, _    := h.repo.CountByStatus(ctx, "running")
    completed, _ := h.repo.CountCompletedToday(ctx)
    findings, _  := h.repo.CountFindingsToday(ctx)
    scheduled, _ := h.repo.CountScheduledActive(ctx)

    json.NewEncoder(w).Encode(ScanStats{
        ActiveScans:    active,
        CompletedToday: completed,
        TotalFindings:  findings,
        ScheduledScans: scheduled,
    })
}
```

**Cache khuyến nghị**: Redis TTL 30 giây (frontend refetch interval = 30s).

---

### 2.2 [HIGH] `GET /api/v1/scans/stats/weekly` — Weekly Activity Chart

Response schema — array 7 items `WeeklyActivity`:
```json
[
  { "day": "Mon", "scans": 8,  "findings": 34 },
  { "day": "Tue", "scans": 12, "findings": 56 },
  { "day": "Wed", "scans": 5,  "findings": 21 },
  { "day": "Thu", "scans": 15, "findings": 72 },
  { "day": "Fri", "scans": 9,  "findings": 41 },
  { "day": "Sat", "scans": 3,  "findings": 11 },
  { "day": "Sun", "scans": 2,  "findings": 7  }
]
```

**Handler implementation (Go):**
```go
// services/scan-service/internal/handler/stats.go
func (h *Handler) GetWeeklyActivity(w http.ResponseWriter, r *http.Request) {
    now := time.Now().UTC()
    result := make([]WeeklyActivity, 7)
    for i := 0; i < 7; i++ {
        day := now.AddDate(0, 0, -(6 - i))
        scans, _    := h.repo.CountByDay(r.Context(), day, "completed")
        findings, _ := h.repo.CountFindingsByDay(r.Context(), day)
        result[i] = WeeklyActivity{
            Day:      day.Format("Mon"),
            Scans:    scans,
            Findings: findings,
        }
    }
    json.NewEncoder(w).Encode(result)
}
```

**Quan trọng:** Data phải ổn định từ DB — **KHÔNG dùng random**. Cache Redis TTL 5 phút.

**SQL query tham khảo:**
```sql
SELECT 
    to_char(date_trunc('day', started_at), 'Dy') AS day,
    COUNT(*) FILTER (WHERE status = 'completed') AS scans,
    SUM(finding_count) AS findings
FROM scans
WHERE started_at >= NOW() - INTERVAL '7 days'
GROUP BY date_trunc('day', started_at)
ORDER BY date_trunc('day', started_at);
```

---

### 2.3 [MEDIUM] `ScansListResponse` — Đảm bảo `stats` field

`GET /api/v1/scans` response cần có `stats` block (có thể đã có, cần xác nhận):

```json
{
  "scans": [...],
  "total": 47,
  "page": 1,
  "page_size": 20,
  "stats": {
    "active_scans": 3,
    "completed_today": 12,
    "total_findings_today": 47,
    "scheduled_scans": 5
  }
}
```

Có thể tái sử dụng logic từ `GetStats()` handler.

---

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

1. `GET /api/v1/scans/stats` trả về `ScanStats` — HTTP 200 trong < 200ms (có cache).
2. Response không bao giờ chứa dữ liệu random — mọi số liệu phải có nguồn từ DB.
3. `GET /api/v1/scans/stats/weekly` trả về array **đúng 7 items** theo thứ tự Mon–Sun.
4. `day` values là tên ngày tiếng Anh viết tắt: `"Mon"`, `"Tue"`, ..., `"Sun"`.
5. `GET /api/v1/scans/stats/weekly` không conflict với `GET /api/v1/scans/stats` (route ordering).
6. `GET /api/v1/scans` response có `stats` field — không breaking change với existing consumers.
7. Cả 2 endpoints yêu cầu Bearer token — trả `401` nếu không có token.
8. `active_scans` trong stats phản ánh số scans đang chạy thực tế (status = running).
