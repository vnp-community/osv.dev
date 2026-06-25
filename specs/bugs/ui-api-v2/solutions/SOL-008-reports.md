# SOL-008 — Reports: Kiểm tra và Fix Handlers (P2)

**Bug**: [BUG-009](../BUG-009-reports.md)  
**Service**: `finding-service`  
**Endpoints**: `GET /api/v1/reports`, `GET /api/v1/reports/templates`  
**HTTP Error**: `404 Not Found`

**Status**: `✅ Implemented` — via [TASK-010](../../tasks/TASK-010-*.md)

---

## Root Cause

Routes đã đăng ký tại [`router.go:225`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go#L225):

```go
mux.Handle("GET /api/v1/reports/templates", protected(proxy.Forward("finding-service:8085")))  // CR-010
mux.Handle("GET /api/v1/reports", protected(proxy.Forward("finding-service:8085")))
```

Finding-service router tại [`router.go:216`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/router.go#L216) có:

```go
r.Route("/api/v2/reports", func(r chi.Router) {
    r.Get("/templates", report.GetTemplates)
    r.Post("/generate", report.Create)
    r.Get("/", report.List)
    // ...
})
```

**Vấn đề**: Chỉ có `/api/v2/reports` mà không có `/api/v1/reports` — gateway forward đến `/api/v1/reports` nhưng finding-service không handle path này.

---

## Giải pháp

### Bước 1: Thêm v1 report routes vào finding-service router

File: [`services/finding-service/internal/delivery/http/router.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/router.go)

```go
// ── Report endpoints (v1 compatibility) ──
if report != nil {
    // v1 routes — PHẢI có trước wildcard /{id}
    r.Get("/api/v1/reports/templates", report.GetTemplates)  // literal TRƯỚC wildcard
    r.Get("/api/v1/reports", report.List)
    r.Post("/api/v1/reports", report.Create)
    r.Get("/api/v1/reports/{id}/download", report.Download)
    r.Get("/api/v1/reports/{id}", report.Get)
    r.Delete("/api/v1/reports/{id}", report.Delete)
}
```

### Bước 2: Kiểm tra GetTemplates handler

File: [`services/finding-service/internal/delivery/http/report_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/report_handler.go)

```go
// GetTemplates — GET /api/v1/reports/templates
func (h *ReportHandler) GetTemplates(w http.ResponseWriter, r *http.Request) {
    // Templates là static list — không cần DB call
    templates := []ReportTemplate{
        {ID: "executive", Name: "Executive Summary", Format: "PDF"},
        {ID: "technical", Name: "Technical Report", Format: "PDF"},
        {ID: "csv_export", Name: "CSV Export", Format: "CSV"},
        {ID: "json_export", Name: "JSON Export", Format: "JSON"},
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data": templates,
    })
}

type ReportTemplate struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    Format string `json:"format"`
}
```

### Bước 3: Fix Report List — Trả empty array thay vì null

```go
// List — GET /api/v1/reports
func (h *ReportHandler) List(w http.ResponseWriter, r *http.Request) {
    reports, err := h.repo.List(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list reports")
        return
    }
    
    if reports == nil {
        reports = make([]*Report, 0)
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data":  reports,  // never null
        "total": len(reports),
    })
}
```

---

## Response Schema

```json
// GET /api/v1/reports/templates
{
  "data": [
    { "id": "executive", "name": "Executive Summary", "format": "PDF" },
    { "id": "technical", "name": "Technical Report", "format": "PDF" },
    { "id": "csv_export", "name": "CSV Export", "format": "CSV" }
  ]
}

// GET /api/v1/reports
{
  "data": [],
  "total": 0
}
```

---

## Files cần sửa

| File | Thay đổi |
|------|----------|
| [`router.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/router.go) | Thêm `/api/v1/reports/*` routes |
| [`report_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/report_handler.go) | Fix nil → [] trong List, đảm bảo GetTemplates static |
