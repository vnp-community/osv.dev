# TASK-BE-P0-02 — Implement scan-service Routes `/api/v1/scans`

**Phase:** Sprint 1 — P0 Unblock  
**Nguồn giải pháp:** [`solutions/SOL-002_implement-scan-routes.md`](../solutions/SOL-002_implement-scan-routes.md)  
**Ưu tiên:** 🔴 P0 — Blocking (trang Scans hoàn toàn 404)  
**Phụ thuộc:** Không có  
**Status:** ✅ **DONE** — 2026-06-19

---

## Mục tiêu

Fix `GET /api/v1/scans` trả 404. Gateway routes đã đúng — vấn đề là `scan-service:8084` chưa implement và register HTTP handler cho `/api/v1/scans`.

---

## Điều tra trước khi code

```bash
# 1. Kiểm tra scan-service có chạy không
docker ps | grep scan
docker logs osv-backend-scan-service-1 --tail 50

# 2. Tìm router file trong scan-service
grep -r "chi.NewRouter\|http.NewServeMux\|r.Get\|r.Post" \
  services/scan-service/internal/ --include="*.go" -l

# 3. Kiểm tra handler file có sẵn không
ls services/scan-service/internal/delivery/http/

# 4. Test trực tiếp scan-service (bỏ qua gateway)
docker exec osv-backend-gateway-1 \
  curl http://scan-service:8084/api/v1/scans
```

---

## Files cần sửa

### [FIND & MODIFY] `services/scan-service/internal/delivery/http/scan_handler.go`

Tìm file handler cho scans:
```bash
grep -r "ListScans\|GET.*scans\|/api/v1/scans" \
  services/scan-service/ --include="*.go" -l
```

Nếu chưa có handler, tạo mới:

```go
// services/scan-service/internal/delivery/http/scan_handler.go
package http

import (
    "net/http"
    "strconv"

    "github.com/go-chi/chi/v5"
    "github.com/rs/zerolog"
)

type ScanHandler struct {
    scanRepo ScanRepository
    log      zerolog.Logger
}

// GET /api/v1/scans
// Query params: status, page, page_size
func (h *ScanHandler) ListScans(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    page     := parseInt(q.Get("page"), 1)
    pageSize := parseInt(q.Get("page_size"), 20)
    status   := q.Get("status") // pending|queued|running|completed|failed|cancelled

    filter := ScanFilter{
        Status:   status,
        Page:     page,
        PageSize: pageSize,
    }

    scans, total, err := h.scanRepo.List(r.Context(), filter)
    if err != nil {
        h.log.Error().Err(err).Msg("list scans failed")
        respondError(w, http.StatusInternalServerError, "failed to list scans")
        return
    }

    stats, _ := h.scanRepo.Stats(r.Context()) // graceful — stats optional

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "scans":     scans,
        "total":     total,
        "page":      page,
        "page_size": pageSize,
        "stats":     stats,
    })
}

// GET /api/v1/scans/scheduled
func (h *ScanHandler) ListScheduled(w http.ResponseWriter, r *http.Request) {
    scans, err := h.scanRepo.ListScheduled(r.Context())
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list scheduled scans")
        return
    }
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "scheduled": scans,
        "total":     len(scans),
    })
}

// GET /api/v1/scans/{id}
func (h *ScanHandler) GetScan(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    scan, err := h.scanRepo.GetByID(r.Context(), id)
    if err != nil {
        respondError(w, http.StatusNotFound, "scan not found")
        return
    }
    respondJSON(w, http.StatusOK, scan)
}

// POST /api/v1/scans
func (h *ScanHandler) CreateScan(w http.ResponseWriter, r *http.Request) {
    var req CreateScanRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    // Validate required fields
    if len(req.Targets) == 0 {
        respondError(w, http.StatusBadRequest, "targets is required")
        return
    }
    scan, err := h.scanRepo.Create(r.Context(), req)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to create scan")
        return
    }
    respondJSON(w, http.StatusCreated, scan)
}

// POST /api/v1/scans/{id}/cancel
func (h *ScanHandler) CancelScan(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if err := h.scanRepo.Cancel(r.Context(), id); err != nil {
        respondError(w, http.StatusInternalServerError, "failed to cancel scan")
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
```

### Response types (theo OpenAPI spec):

```go
// types.go
type ScanResponse struct {
    ID           string   `json:"id"`
    Name         string   `json:"name"`
    Type         string   `json:"type"`   // nmap_full|nmap_discovery|zap|agent|import
    Status       string   `json:"status"` // pending|queued|running|completed|failed|cancelled
    Targets      []string `json:"targets"`
    Progress     int      `json:"progress"`      // 0-100
    FindingCount int      `json:"finding_count"`
    CreatedBy    string   `json:"created_by"`
    CreatedAt    string   `json:"created_at"`
    StartedAt    *string  `json:"started_at,omitempty"`
    CompletedAt  *string  `json:"completed_at,omitempty"`
}

type ScanStatsResponse struct {
    ActiveScans        int `json:"active_scans"`
    CompletedToday     int `json:"completed_today"`
    TotalFindingsToday int `json:"total_findings_today"`
    ScheduledScans     int `json:"scheduled_scans"`
}
```

### [FIND & MODIFY] Router file — register scan routes

```bash
# Tìm file đăng ký routes
grep -r "r\.Get\|chi\.NewRouter\|mux\.Handle" \
  services/scan-service/ --include="*.go" -l
```

Thêm vào router:
```go
// Trong router setup function của scan-service
// THÊM literal route trước wildcard
r.Get("/api/v1/scans/scheduled", scanHandler.ListScheduled) // TRƯỚC /{id}
r.Get("/api/v1/scans", scanHandler.ListScans)
r.Post("/api/v1/scans", scanHandler.CreateScan)
r.Get("/api/v1/scans/{id}", scanHandler.GetScan)
r.Post("/api/v1/scans/{id}/cancel", scanHandler.CancelScan)
r.Get("/api/v1/scans/{id}/results/nmap", scanHandler.GetNmapResults)
r.Get("/api/v1/scans/{id}/results/zap", scanHandler.GetZapResults)
```

### [CHECK] Database migration

```bash
# Kiểm tra bảng scans có tồn tại không
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "SELECT tablename FROM pg_tables WHERE tablename = 'scans';"

# Nếu không tồn tại, chạy migration
# Tìm migration file
ls services/scan-service/migrations/
```

Nếu cần tạo migration:
```sql
-- services/scan-service/migrations/YYYYMMDD_create_scans.sql
CREATE TABLE IF NOT EXISTS scans (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(255) NOT NULL DEFAULT '',
    type          VARCHAR(50) NOT NULL,
    status        VARCHAR(50) NOT NULL DEFAULT 'pending',
    targets       TEXT[] NOT NULL DEFAULT '{}',
    progress      INT DEFAULT 0,
    finding_count INT DEFAULT 0,
    created_by    UUID,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW(),
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    error_message TEXT,
    config        JSONB
);

CREATE INDEX IF NOT EXISTS idx_scans_status ON scans(status);
CREATE INDEX IF NOT EXISTS idx_scans_created_at ON scans(created_at DESC);
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/scans` trả HTTP 200 (không còn 404)
- [ ] Response có `{ "scans": [], "total": 0, "page": 1, "page_size": 20, "stats": {...} }`
- [ ] `GET /api/v1/scans?status=running` hoạt động (filter theo status)
- [ ] `GET /api/v1/scans/scheduled` trả HTTP 200
- [ ] `GET /api/v1/scans/{id}` với UUID hợp lệ trả 200 hoặc 404 (không phải 500)
- [ ] `POST /api/v1/scans` với body hợp lệ trả 201

## Verification

```bash
# List scans (empty ok)
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/scans
# Expected: HTTP 200

# Scheduled
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/scans/scheduled
# Expected: HTTP 200

# Filter
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/scans?status=running&page_size=5"
# Expected: HTTP 200 (không phải 404)
```
