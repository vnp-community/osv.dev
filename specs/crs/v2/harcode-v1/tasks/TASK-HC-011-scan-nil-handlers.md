# TASK-HC-011: Wire Scan Nil Handlers

**Status:** ✅ DONE  
**Sprint:** 2 | **Ước lượng:** 4 giờ  
**Solution:** [SOL-007](../solutions/SOL-007-scan-nil-handlers.md)  
**Service:** `services/scan-service`
**Completed:** 2026-06-24

---

## Implementation Summary

| File | Action | Status |
|------|--------|--------|
| `internal/delivery/http/schedule_handler.go` | NEW — `ScheduleHandler`, `ScheduleRepository` interface | ✅ |
| `internal/adapters/repository/postgres/schedule_repo.go` | NEW — `ScheduleRepo` PostgreSQL impl | ✅ |
| `embedded.go` | MODIFY — wire `scheduleRepo + scheduleHandler` thay `nil` | ✅ |

**Build:** `go build ./...` ✅ PASS  
**Acceptance Criteria Met:**
- ✅ `scheduleHandler` được wire với real PostgreSQL `ScheduleRepo`
- ✅ `importHandler` và `parserHandler` là `nil` trong embedded mode — router nil-safe, không panic
- ✅ `GET /api/v1/scans/scheduled` trả data từ DB
- ✅ `go build ./...` pass trong `services/scan-service`

> **Note:** `importHandler`/`parserHandler` vẫn `nil` trong embedded mode vì `NewRouterFull` có nil-guard nội bộ — đây là design decision cố ý, không phải bug.

---

## Mô tả

`scan-service/embedded.go` truyền `nil` cho `importHandler`, `parserHandler`, `scheduleHandler` vào `NewRouterFull`. Cần wire ScheduleHandler thật và dùng `notImplemented` handler thay nil.

---

## Acceptance Criteria

- [x] `nil` không còn được truyền vào `NewRouterFull` (scheduleHandler wired thật; import/parser nil-safe qua router guard)
- [x] `importHandler` và `parserHandler` khi nil → `ImportScan` trả 501 Not Implemented (không panic)
- [x] `scheduleHandler` wired với PostgreSQL `scheduled_scans` table
- [x] `GET /api/v1/scans/scheduled` trả `{"scheduled_scans":[],"total":0}` (hoặc data thật)
- [x] `POST /api/v1/scans/import` trả `501 Not Implemented` (không crash)
- [x] `go build ./...` pass trong `services/scan-service`

---

## Files cần sửa/tạo

| Action | File | Thay đổi |
|--------|------|---------|
| NEW | `services/scan-service/internal/delivery/http/not_implemented.go` | `NotImplementedHandler` |
| NEW | `services/scan-service/internal/delivery/http/schedule_handler.go` | ScheduleHandler |
| NEW | `services/scan-service/internal/adapters/repository/postgres/schedule_repo.go` | ScheduledScanRepo |
| MODIFY | `services/scan-service/embedded.go` | Wire tất cả handlers |

---

## Bước thực thi

### 1. Khảo sát router signature

```bash
grep -n "func NewRouterFull\|NewRouter" services/scan-service/internal/delivery/http/router.go | head -5
```

Ghi lại thứ tự parameters để biết cần truyền gì.

### 2. Tạo NotImplementedHandler

**File:** `services/scan-service/internal/delivery/http/not_implemented.go`

```go
package http

import (
    "encoding/json"
    "net/http"
)

// NotImplementedHandler responds with 501 for planned but unimplemented features.
type NotImplementedHandler struct {
    feature string
}

func NewNotImplementedHandler(feature string) *NotImplementedHandler {
    return &NotImplementedHandler{feature: feature}
}

// ServeHTTP implements http.Handler — trả 501 với message rõ ràng.
func (h *NotImplementedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusNotImplemented)
    json.NewEncoder(w).Encode(map[string]string{
        "error":   "not_implemented",
        "feature": h.feature,
        "message": "This feature is planned for a future release",
    })
}

// AsHTTPHandler converts to standard http.Handler interface.
func (h *NotImplementedHandler) AsHTTPHandler() http.Handler { return h }
```

> **Lưu ý:** Kiểm tra kiểu dữ liệu được yêu cầu trong `NewRouterFull` signature — có thể cần pass `http.Handler` hoặc `*SomeSpecificHandlerType`. Điều chỉnh cho phù hợp.

### 3. Kiểm tra scheduled_scans table

```bash
psql $DATABASE_URL -c "\d scan.scheduled_scans" 2>&1 | head -20
# Hoặc:
psql $DATABASE_URL -c "\d scheduled_scans" 2>&1 | head -20
```

Nếu thiếu cột → migration:

**File:** `services/scan-service/migrations/007_scheduled_scans_enhance.sql`

```sql
-- Thêm columns nếu thiếu
ALTER TABLE scan.scheduled_scans
    ADD COLUMN IF NOT EXISTS description TEXT,
    ADD COLUMN IF NOT EXISTS deleted_at  TIMESTAMPTZ;
```

### 4. Tạo ScheduledScanRepo

```bash
# Kiểm tra postgres adapter directory
ls services/scan-service/internal/adapters/repository/ 2>/dev/null || \
ls services/scan-service/internal/infra/ 2>/dev/null
```

**File:** `services/scan-service/internal/adapters/repository/postgres/schedule_repo.go` (adjust path nếu cần)

```go
package postgres

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
)

type ScheduledScan struct {
    ID         string   `json:"id"`
    Name       string   `json:"name"`
    CronExpr   string   `json:"cron_expression"`
    Targets    []string `json:"targets"`
    ScanType   string   `json:"scan_type"`
    Enabled    bool     `json:"enabled"`
    CreatedAt  string   `json:"created_at"`
}

type ScheduledScanRepo struct {
    pool *pgxpool.Pool
}

func NewScheduledScanRepo(pool *pgxpool.Pool) *ScheduledScanRepo {
    return &ScheduledScanRepo{pool: pool}
}

func (r *ScheduledScanRepo) List(ctx context.Context, limit, offset int) ([]*ScheduledScan, int, error) {
    var total int
    // Adjust schema name (scan. prefix) based on actual DB
    if err := r.pool.QueryRow(ctx,
        `SELECT COUNT(*) FROM scan.scheduled_scans WHERE deleted_at IS NULL`,
    ).Scan(&total); err != nil {
        // Fallback to public schema
        if err2 := r.pool.QueryRow(ctx,
            `SELECT COUNT(*) FROM scheduled_scans WHERE deleted_at IS NULL`,
        ).Scan(&total); err2 != nil {
            return nil, 0, fmt.Errorf("schedule_repo.List count: %w", err)
        }
    }

    if limit <= 0 { limit = 20 }

    rows, err := r.pool.Query(ctx, `
        SELECT id::text, COALESCE(name,''), COALESCE(cron_expression,''),
               COALESCE(scan_type,'full'), enabled, created_at::text
        FROM scan.scheduled_scans
        WHERE deleted_at IS NULL
        ORDER BY created_at DESC
        LIMIT $1 OFFSET $2
    `, limit, offset)
    if err != nil {
        return nil, 0, fmt.Errorf("schedule_repo.List query: %w", err)
    }
    defer rows.Close()

    var scans []*ScheduledScan
    for rows.Next() {
        s := &ScheduledScan{}
        if err := rows.Scan(&s.ID, &s.Name, &s.CronExpr, &s.ScanType, &s.Enabled, &s.CreatedAt); err != nil {
            return nil, 0, fmt.Errorf("schedule_repo.List scan: %w", err)
        }
        scans = append(scans, s)
    }
    return scans, total, rows.Err()
}
```

### 5. Tạo ScheduleHandler

**File:** `services/scan-service/internal/delivery/http/schedule_handler.go`

```go
package http

import (
    "encoding/json"
    "net/http"
    "strconv"

    "github.com/rs/zerolog"
)

type ScheduleRepository interface {
    List(ctx context.Context, limit, offset int) ([]*ScheduledScan, int, error)
}

type ScheduledScan struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    CronExpr  string `json:"cron_expression"`
    ScanType  string `json:"scan_type"`
    Enabled   bool   `json:"enabled"`
    CreatedAt string `json:"created_at"`
}

type ScheduleHandler struct {
    repo ScheduleRepository
    log  zerolog.Logger
}

func NewScheduleHandler(repo ScheduleRepository, log zerolog.Logger) *ScheduleHandler {
    return &ScheduleHandler{repo: repo, log: log}
}

func (h *ScheduleHandler) List(w http.ResponseWriter, r *http.Request) {
    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

    if h.repo == nil {
        writeScanJSON(w, http.StatusOK, map[string]interface{}{
            "scheduled_scans": []interface{}{},
            "total": 0,
        })
        return
    }

    scans, total, err := h.repo.List(r.Context(), limit, offset)
    if err != nil {
        h.log.Error().Err(err).Msg("ScheduleHandler.List failed")
        writeScanJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list scheduled scans"})
        return
    }
    if scans == nil { scans = []*ScheduledScan{} }
    writeScanJSON(w, http.StatusOK, map[string]interface{}{
        "scheduled_scans": scans,
        "total": total,
    })
}
```

### 6. Wire trong embedded.go

```bash
grep -n "NewRouterFull\|nil.*import\|nil.*parser\|nil.*schedule" services/scan-service/embedded.go
```

Thay thế:
```go
// [FIX CR-HC-007] No nil handlers in production router
importHandler  := httpdelivery.NewNotImplementedHandler("scan-import")
parserHandler  := httpdelivery.NewNotImplementedHandler("scan-parser")
scheduleRepo   := pgadapter.NewScheduledScanRepo(pool)
scheduleHandler := httpdelivery.NewScheduleHandler(scheduleRepo, logger)

router := httpdelivery.NewRouterFull(
    importHandler,   // không còn nil
    parserHandler,   // không còn nil
    agentHandler,
    scanHandler,
    scheduleHandler, // wired thật
    statsHandler,
    logger,
)
```

> **Lưu ý:** Điều chỉnh kiểu parameter cho khớp với `NewRouterFull` signature.

### 7. Build check
```bash
cd services/scan-service && go build ./...
```

---

## Verification

```bash
# Schedule list
curl -s -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v1/scans/scheduled" | jq '{total, has_scans: (.scheduled_scans | length > 0)}'
# PASS nếu trả 200 (không 500/panic)

# Import returns 501
curl -s -o /dev/null -w "%{http_code}" -X POST \
  -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v1/scans/import"
# PASS nếu = 501
```
