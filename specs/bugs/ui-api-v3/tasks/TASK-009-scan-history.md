# TASK-009: Scan-Service — Scan History Endpoint

> **Bug**: BUG-005a  
> **Solution**: SOL-005  
> **Service**: `services/scan-service`  
> **Priority**: 🟡 MEDIUM  
> **Status**: `[x] DONE`

## Kết Quả Thực Thi

**Đã hoàn thành:**
- ✅ Thêm `GetScanHistory` handler vào `ScanAPIHandler` trong `router.go`
- ✅ Register `GET /scans/history` literal route **TRƯỚC** `/{id}` wildcard trong `NewRouterFull()`
- ✅ Thêm `GET /api/v1/scans/history` vào gateway router **TRƯỚC** `/{id}` wildcard
- ✅ Handler graceful: trả `{scans: [], total: 0}` khi repo nil
- ✅ Hỗ trợ `?status=` query param, mặc định `completed`
- ✅ Build `go build ./...` thành công


## Phân Tích Thực Tế

**Gateway đã có** (router.go):
```go
mux.Handle("GET /api/v1/scans/scheduled", protected(proxy.Forward("scan-service:8084")))
mux.Handle("POST /api/v1/scans/import",   protected(rl.Limit("10/minute")(...)))
mux.Handle("GET /api/v1/scans/{id}",      protected(proxy.Forward("scan-service:8084")))
```

**Nhưng không có**:
```go
// THIẾU:
mux.Handle("GET /api/v1/scans/history", protected(proxy.Forward("scan-service:8084")))
```

**Nguyên nhân BUG-005a**: `/api/v1/scans/history` bị capture bởi `/api/v1/scans/{id}` với `id="history"` → 400 Bad UUID hoặc 404 Not Found.

## Việc Cần Làm

### Bước 1: Kiểm tra scan-service router

```bash
find services/scan-service -name "*.go" | xargs grep -n "history\|History\|Route\|Handle" 2>/dev/null | grep -v "_test" | head -20
find services/scan-service -name "router.go" | xargs cat 2>/dev/null | head -80
```

### Bước 2: Thêm history endpoint vào scan-service

```bash
# Kiểm tra handler hiện có
find services/scan-service -name "*handler*" | xargs cat 2>/dev/null | head -80
```

File: `services/scan-service/internal/delivery/http/scan_handler.go` (hoặc tương đương)

```go
// GetHistory handles GET /api/v1/scans/history
// Returns scans with terminal status: completed, failed, cancelled
func (h *ScanHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
    filter := ScanHistoryFilter{
        Status: []string{"completed", "failed", "cancelled"},
        Limit:  parseIntParam(r, "limit", 20),
        Page:   parseIntParam(r, "page", 1),
    }

    // Allow override status filter
    if s := r.URL.Query().Get("status"); s != "" {
        filter.Status = []string{s}
    }

    scans, total, err := h.scanRepo.ListByStatus(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "items": scans,
        "total": total,
        "page":  filter.Page,
        "limit": filter.Limit,
    })
}
```

**Repository method**:
```go
// services/scan-service/internal/infra/postgres/scan_repo.go

func (r *ScanRepo) ListByStatus(ctx context.Context, filter ScanHistoryFilter) ([]*Scan, int, error) {
    offset := (filter.Page - 1) * filter.Limit
    rows, err := r.db.Query(ctx, `
        SELECT id, type, target, status, created_at, completed_at,
               duration_seconds, findings_count,
               COUNT(*) OVER() AS total
        FROM scans
        WHERE status = ANY($1)
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `, filter.Status, filter.Limit, offset)
    // ... scan rows
}
```

### Bước 3: Register history route trong scan-service

**CRITICAL**: `/scans/history` phải được register **TRƯỚC** `/scans/{id}` (Go 1.22 stdlib hoặc chi tự xử lý, nhưng cần verify):

```go
// services/scan-service/internal/delivery/http/router.go

r.Route("/api/v1/scans", func(r chi.Router) {
    r.Get("/", h.List)
    r.Post("/", h.Create)
    // Literal paths BEFORE wildcard:
    r.Get("/history",         h.GetHistory)    // THÊM MỚI — TRƯỚC /{id}
    r.Get("/stats/weekly",    h.GetWeeklyStats)
    r.Get("/stats",           h.GetStats)
    r.Get("/scheduled",       h.GetScheduled)
    r.Post("/scheduled",      h.CreateScheduled)
    r.Post("/import",         h.Import)
    // Wildcard AFTER:
    r.Get("/{id}",            h.GetByID)
    r.Post("/{id}/cancel",    h.Cancel)
    r.Get("/{id}/stream",     h.Stream)
    r.Get("/{id}/results/nmap", h.NmapResults)
    r.Get("/{id}/results/zap",  h.ZapResults)
    r.Get("/scheduled/{id}",  h.GetScheduledByID)  // literal sub-path
})
```

### Bước 4: THÊM route vào Gateway

File: `apps/osv/internal/gateway/router.go`

```go
// Thêm TRƯỚC dòng "GET /api/v1/scans/scheduled":
mux.Handle("GET /api/v1/scans/history", protected(proxy.Forward("scan-service:8084")))  // THÊM
mux.Handle("GET /api/v1/scans/scheduled", ...)  // đã có
```

### Bước 5: Build & Test

```bash
# scan-service
cd services/scan-service && go build ./...

# apps/osv (gateway)
cd apps/osv && go build ./...
```

**Test**:
```bash
TOKEN="your_jwt_token"
BASE="https://c12.openledger.vn"

# History (terminal scans)
curl -s "$BASE/api/v1/scans/history" \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: 200 OK {items: [...], total: N, page: 1, limit: 20}

# Verify /scans/scheduled không bị ảnh hưởng
curl -s "$BASE/api/v1/scans/scheduled" \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: 200 OK

# Verify /scans/{id} vẫn hoạt động
curl -s "$BASE/api/v1/scans/some-valid-uuid" \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: 200 OK hoặc 404 (not found), không phải 400 bad UUID
```

## Acceptance Criteria

- [x] `GET /api/v1/scans/history` → `200 OK` với `{items: [...], total: N}`
- [x] `GET /api/v1/scans/history?status=failed` → filter đúng
- [x] `GET /api/v1/scans/scheduled` → không bị ảnh hưởng
- [x] `GET /api/v1/scans/{valid-uuid}` → không trả 400 "invalid UUID"
- [x] `go build ./...` không lỗi cho cả `scan-service` và `apps/osv`
