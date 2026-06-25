# SOL-002 — Fix Scans 404 (BUG-BE-002)

| Trường | Giá trị |
|---|---|
| **Bug** | [BUG-BE-002](../BUG-BE-002_scans-404.md) |
| **Service** | `services/scan-service` (port :8084) |
| **Gateway** | `apps/osv/internal/gateway/router.go` |
| **Priority** | P0 — Blocking |
| **Estimated effort** | 4–8h |
| **Kiến trúc** | [architecture.md §3.6](../../../01-architecture.md) — Scan-Service |

---

## Root Cause

Từ `apps/osv/internal/gateway/router.go` — routes đã được mount tại line 137–145:

```go
// Line 137-145 trong router.go — đã có trong code
mux.Handle("GET /api/v1/scans", protected(proxy.Forward("scan-service:8084")))
mux.Handle("POST /api/v1/scans", protected(proxy.Forward("scan-service:8084")))
mux.Handle("GET /api/v1/scans/scheduled", protected(proxy.Forward("scan-service:8084")))
mux.Handle("GET /api/v1/scans/{id}", protected(proxy.Forward("scan-service:8084")))
mux.Handle("POST /api/v1/scans/{id}/cancel", protected(proxy.Forward("scan-service:8084")))
```

**Routes đã đúng trong gateway**, nhưng `scan-service:8084` không trả lời → 404 từ proxy.

**Nguyên nhân thực**: scan-service container không chạy hoặc không expose `/api/v1/scans` handler.

---

## Điều Tra

```bash
# SSH vào server 172.20.2.48
# Kiểm tra scan-service có đang chạy không:
docker ps | grep scan
docker logs osv-backend-scan-service-1 --tail 100

# Kiểm tra có thể reach scan-service từ gateway:
docker exec osv-backend-gateway-1 curl http://scan-service:8084/api/v1/scans
# Nếu "connection refused" → service không chạy
# Nếu 404 → service chạy nhưng route chưa register
```

---

## Giải Pháp

### Case A — scan-service không chạy

```bash
# Khởi động lại:
docker compose -f docker-compose.server.yml up -d scan-service
docker logs osv-backend-scan-service-1 --tail 50
```

### Case B — scan-service chạy nhưng route `/api/v1/scans` chưa register

Cần thêm route handler trong `services/scan-service`. Theo arch §3.6, scan-service có structure:
```
internal/delivery/http/ → HTTP handlers
```

Tìm file router trong scan-service:
```bash
find services/scan-service -name "*.go" | xargs grep -l "chi.NewRouter\|http.NewServeMux"
```

Thêm route handler cho `/api/v1/scans` (GET list):

```go
// services/scan-service/internal/delivery/http/scan_handler.go (THÊM MỚI hoặc CHỈNH SỬA)

// ListScans handles GET /api/v1/scans
// Spec: returns { scans: [...], total: N, page: N, page_size: N, stats: {...} }
func (h *ScanHandler) ListScans(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    
    filter := ScanFilter{
        Status:   q.Get("status"),   // pending|queued|running|completed|failed
        Page:     parseInt(q.Get("page"), 1),
        PageSize: parseInt(q.Get("page_size"), 20),
    }
    
    scans, total, err := h.scanRepo.List(r.Context(), filter)
    if err != nil {
        log.Error().Err(err).Msg("list scans failed")
        respondError(w, http.StatusInternalServerError, "failed to list scans")
        return
    }
    
    // Build stats
    stats, _ := h.scanRepo.Stats(r.Context())
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "scans":     scans,
        "total":     total,
        "page":      filter.Page,
        "page_size": filter.PageSize,
        "stats":     stats,
    })
}

// Scan response schema (theo OpenAPI spec):
type ScanResponse struct {
    ID           string   `json:"id"`
    Name         string   `json:"name"`
    Type         string   `json:"type"` // "nmap_full|nmap_discovery|zap|agent|import"
    Status       string   `json:"status"` // "pending|queued|running|completed|failed|cancelled"
    Targets      []string `json:"targets"`
    Progress     int      `json:"progress"` // 0-100
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

Đăng ký route trong scan-service router:
```go
// services/scan-service/internal/delivery/http/router.go (hoặc tương đương)
r.Get("/api/v1/scans", h.ListScans)
r.Post("/api/v1/scans", h.CreateScan)
r.Get("/api/v1/scans/scheduled", h.ListScheduled)
r.Get("/api/v1/scans/{id}", h.GetScan)
r.Post("/api/v1/scans/{id}/cancel", h.CancelScan)
r.Get("/api/v1/scans/{id}/results/nmap", h.GetNmapResults)
r.Get("/api/v1/scans/{id}/results/zap", h.GetZapResults)
```

### Database Schema (nếu chưa có migration)

```sql
-- Kiểm tra bảng scans có tồn tại không:
SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename = 'scans';

-- Nếu chưa có:
CREATE TABLE IF NOT EXISTS scans (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(255) NOT NULL,
    type          VARCHAR(50) NOT NULL,  -- nmap_full|nmap_discovery|zap|agent|import
    status        VARCHAR(50) NOT NULL DEFAULT 'pending',
    targets       TEXT[],
    progress      INT DEFAULT 0,
    finding_count INT DEFAULT 0,
    created_by    UUID,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    error_message TEXT
);
```

---

## Xác Nhận Fix

```bash
# Sau khi deploy:
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/scans
# Expected: HTTP 200 { "scans": [], "total": 0, ... }

curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/scans?status=running"
# Expected: HTTP 200 (không còn 404/500)
```

## Không Thay Đổi

- `apps/osv/internal/gateway/router.go` — routes đã đúng, KHÔNG sửa
- nginx `c12.openledger.vn.conf` — proxy `/api/` đã đúng, KHÔNG sửa

## Liên Quan

- [architecture.md §3.6](../../../01-architecture.md) — Scan-Service structure
- [technical-design.md §6.2](../../../02-technical-design.md) — Import Pipeline
