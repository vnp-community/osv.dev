# TASK-010 — finding-service: Thêm `/api/v1/reports/*` v1 Routes

**Bug**: [BUG-009](../BUG-009-reports.md)  
**Solution**: [SOL-008](../solutions/SOL-008-reports.md)  
**Priority**: 🟡 P2  
**Effort**: ~15 phút  
**Status**: `[x] DONE`

---

## Mô tả

`GET /api/v1/reports` và `GET /api/v1/reports/templates` trả `404`. Finding-service chỉ có `/api/v2/reports/*` routes — chưa có v1 compat path. Gateway forward đến `/api/v1/reports` nhưng finding-service không handle.

---

## File cần sửa

**File**: [`services/finding-service/internal/delivery/http/router.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/router.go)

**File 2** (nếu cần): [`services/finding-service/internal/delivery/http/report_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/report_handler.go)

---

## Thay đổi 1 — router.go: Thêm v1 report routes

**Tìm** section report endpoints (dòng ~216):

```go
    // ── Report endpoints ──
    if report != nil {
        r.Route("/api/v2/reports", func(r chi.Router) {
            r.Get("/templates", report.GetTemplates)
            r.Post("/generate", report.Create)
            r.Get("/", report.List)
            r.Get("/{id}", report.Get)
            r.Get("/{id}/download", report.Download)
            r.Delete("/{id}", report.Delete)
        })
    }
```

**Thêm v1 routes** bên trên (literal paths TRƯỚC wildcard):

```go
    // ── Report endpoints ──
    if report != nil {
        // v1 compatibility routes (gateway forwards /api/v1/reports/* → finding-service)
        // QUAN TRỌNG: /templates và /{id}/download PHẢI trước /{id}
        r.Get("/api/v1/reports/templates", report.GetTemplates)         // literal TRƯỚC
        r.Get("/api/v1/reports/{id}/download", report.Download)         // literal TRƯỚC
        r.Get("/api/v1/reports", report.List)
        r.Post("/api/v1/reports", report.Create)
        r.Get("/api/v1/reports/{id}", report.Get)
        r.Delete("/api/v1/reports/{id}", report.Delete)

        // v2 routes (giữ nguyên)
        r.Route("/api/v2/reports", func(r chi.Router) {
            r.Get("/templates", report.GetTemplates)
            r.Post("/generate", report.Create)
            r.Get("/", report.List)
            r.Get("/{id}", report.Get)
            r.Get("/{id}/download", report.Download)
            r.Delete("/{id}", report.Delete)
        })
    }
```

---

## Thay đổi 2 — report_handler.go: Đảm bảo GetTemplates static

**Tìm** `GetTemplates` handler. Nếu đang cố query DB → có thể fail. **Đảm bảo** trả static list:

```go
// GetTemplates — GET /api/v1/reports/templates
func (h *ReportHandler) GetTemplates(w http.ResponseWriter, r *http.Request) {
    templates := []map[string]string{
        {"id": "executive_summary", "name": "Executive Summary", "format": "PDF"},
        {"id": "technical_report",  "name": "Technical Report",  "format": "PDF"},
        {"id": "csv_export",        "name": "CSV Export",        "format": "CSV"},
        {"id": "json_export",       "name": "JSON Export",       "format": "JSON"},
        {"id": "excel_report",      "name": "Excel Report",      "format": "XLSX"},
    }
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data": templates,
    })
}
```

## Thay đổi 3 — report_handler.go: List trả `[]` thay vì `null`

**Tìm** `List` handler và **thêm nil guard**:

```go
func (h *ReportHandler) List(w http.ResponseWriter, r *http.Request) {
    reports, err := h.repo.List(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list reports")
        return
    }

    // Defensive: never nil
    if reports == nil {
        reports = make([]*Report, 0)
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data":  reports,
        "total": len(reports),
    })
}
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/reports/templates` trả `200` với list templates
- [ ] `GET /api/v1/reports` trả `200` với `{"data": []}`
- [ ] Templates endpoint không cần DB (static list)
- [ ] `go build ./...` trong finding-service không có lỗi

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service
go build ./...

curl -s -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/reports/templates" | jq '.data | length'
# Expected: 5 (hoặc số templates defined)

curl -s -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/reports" | jq '.data | type'
# Expected: "array"
```
