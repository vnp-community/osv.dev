# SOL-007: Wire Nil Handlers — scan-service

**CR:** CR-HC-007 | **Priority:** 🟡 Medium | **Sprint:** 2  
**Service:** `services/scan-service`

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-011
**Note:** ScheduleRepo thật (PostgreSQL) wire vào scan-service embedded
**Build:** ✅ `go build ./...` passes

---

---

## Context phân tích code

**File:** `scan-service/embedded.go:54-58`
```go
router := httpdelivery.NewRouterFull(
    nil,          // importHandler — not wired in embedded mode
    nil,          // parserHandler — not wired in embedded mode
    agentHandler,
    scanHandler,
    nil,          // scheduleHandler — not wired yet
    statsHandler,
    logger,
)
```

**Cần kiểm tra:**
```bash
grep -n "NewRouterFull" scan-service/internal/delivery/http/router.go
```

**Chiến lược:** Dùng `notImplemented()` cho importHandler và parserHandler (ít urgent), wire thật `scheduleHandler` vì API đang được test.

---

## Solution

### Bước 1: Thêm `notImplemented` helper trong router

**File:** `scan-service/internal/delivery/http/router.go`

```go
// notImplemented returns a handler that responds with 501 Not Implemented.
// Use for routes that are planned but not yet implemented.
func notImplemented(feature string) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusNotImplemented)
        json.NewEncoder(w).Encode(map[string]string{
            "error":   "not implemented",
            "feature": feature,
            "hint":    "This feature is planned for a future release",
        })
    })
}
```

### Bước 2: Wire scheduleHandler thật

**Kiểm tra scheduled_scans table tồn tại:**
```sql
-- scan-service/migrations/006_scheduled_scans_enhance.sql
ALTER TABLE scan.scheduled_scans
    ADD COLUMN IF NOT EXISTS description TEXT,
    ADD COLUMN IF NOT EXISTS user_id     UUID,
    ADD COLUMN IF NOT EXISTS next_run    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_run    TIMESTAMPTZ;
```

**File:** `scan-service/internal/adapters/repository/postgres/schedule_repo.go`

```go
package postgres

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v5/pgxpool"
)

type ScheduledScanRepo struct {
    pool *pgxpool.Pool
}

func NewScheduledScanRepo(pool *pgxpool.Pool) *ScheduledScanRepo {
    return &ScheduledScanRepo{pool: pool}
}

type ScheduledScan struct {
    ID          string  `json:"id"`
    Name        string  `json:"name"`
    CronExpr    string  `json:"cron_expression"`
    Targets     []string `json:"targets"`
    ScanType    string  `json:"scan_type"`
    Enabled     bool    `json:"enabled"`
    UserID      *string `json:"user_id"`
    NextRun     *string `json:"next_run"`
    LastRun     *string `json:"last_run"`
    CreatedAt   string  `json:"created_at"`
}

func (r *ScheduledScanRepo) List(ctx context.Context, userID string, limit, offset int) ([]*ScheduledScan, int, error) {
    var total int
    err := r.pool.QueryRow(ctx,
        `SELECT COUNT(*) FROM scan.scheduled_scans WHERE deleted_at IS NULL`,
    ).Scan(&total)
    if err != nil {
        return nil, 0, fmt.Errorf("schedule_repo.List count: %w", err)
    }

    rows, err := r.pool.Query(ctx, `
        SELECT id::text, COALESCE(name,''), COALESCE(cron_expression,''),
               COALESCE(targets, '[]'::jsonb), COALESCE(scan_type,'full'),
               enabled, user_id::text, next_run::text, last_run::text, created_at::text
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
        var targetsJSON []byte
        if err := rows.Scan(&s.ID, &s.Name, &s.CronExpr, &targetsJSON, &s.ScanType,
            &s.Enabled, &s.UserID, &s.NextRun, &s.LastRun, &s.CreatedAt); err != nil {
            return nil, 0, fmt.Errorf("schedule_repo.List scan: %w", err)
        }
        json.Unmarshal(targetsJSON, &s.Targets)
        scans = append(scans, s)
    }
    return scans, total, rows.Err()
}
```

**File:** `scan-service/internal/delivery/http/schedule_handler.go`

```go
package http

import (
    "encoding/json"
    "net/http"
    "strconv"
    "github.com/rs/zerolog"
)

type ScheduleRepository interface {
    List(ctx context.Context, userID string, limit, offset int) ([]*ScheduledScan, int, error)
    Create(ctx context.Context, s *ScheduledScan) error
    Delete(ctx context.Context, id string) error
}

type ScheduleHandler struct {
    repo ScheduleRepository
    log  zerolog.Logger
}

func NewScheduleHandler(repo ScheduleRepository, log zerolog.Logger) *ScheduleHandler {
    return &ScheduleHandler{repo: repo, log: log}
}

func (h *ScheduleHandler) List(w http.ResponseWriter, r *http.Request) {
    if h.repo == nil {
        writeScanJSON(w, http.StatusOK, map[string]interface{}{
            "scheduled_scans": []interface{}{},
            "total":           0,
        })
        return
    }
    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    if limit <= 0 {
        limit = 20
    }
    offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

    scans, total, err := h.repo.List(r.Context(), r.Header.Get("X-User-ID"), limit, offset)
    if err != nil {
        h.log.Error().Err(err).Msg("ScheduleHandler.List")
        writeScanJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list scheduled scans"})
        return
    }
    writeScanJSON(w, http.StatusOK, map[string]interface{}{
        "scheduled_scans": scans,
        "total":           total,
    })
}
```

### Bước 3: Wire trong embedded.go

**File sửa:** `scan-service/embedded.go`

```go
// [FIX CR-HC-007] Wire real handlers — no nil handlers in production router
if pool != nil {
    scheduleRepo := pgadapter.NewScheduledScanRepo(pool)
    scheduleHandler = httpdelivery.NewScheduleHandler(scheduleRepo, logger)
    logger.Info().Msg("scan-service: ScheduleHandler wired (PostgreSQL)")
} else {
    scheduleHandler = httpdelivery.NewScheduleHandler(nil, logger) // graceful empty
}

router := httpdelivery.NewRouterFull(
    notImplementedHandler("scan-import"),  // [FIX CR-HC-007] no nil
    notImplementedHandler("scan-parser"),  // [FIX CR-HC-007] no nil
    agentHandler,
    scanHandler,
    scheduleHandler,  // [FIX CR-HC-007] wired thật
    statsHandler,
    logger,
)

// notImplementedHandler helper
func notImplementedHandler(feature string) *httpdelivery.NotImplementedHandler {
    return httpdelivery.NewNotImplementedHandler(feature)
}
```

---

## Files cần tạo/sửa

| Action | File |
|--------|------|
| NEW | `scan-service/migrations/006_scheduled_scans_enhance.sql` |
| NEW | `scan-service/internal/adapters/repository/postgres/schedule_repo.go` |
| NEW | `scan-service/internal/delivery/http/schedule_handler.go` |
| NEW | `scan-service/internal/delivery/http/not_implemented.go` |
| MODIFY | `scan-service/embedded.go` — wire all handlers |
| MODIFY | `scan-service/internal/delivery/http/router.go` — support nil-safe |

---

## Verification

```bash
# Build
cd services/scan-service && go build ./...

# Test no panic on import
curl -X POST -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v1/scans/import"
# Expect: 501 Not Implemented (không panic)

# Test schedule list
curl -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v1/scans/scheduled"
# Expect: {"scheduled_scans":[],"total":0}
```
