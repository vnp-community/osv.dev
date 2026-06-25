# BUG-009 — Reports: Report Center (API 404)

**Nguồn gốc**: [`ui-crash/BUG-reports.md`](../../../ui/specs/bugs/ui-crash/BUG-reports.md)  
**Loại**: 🔴 API 404 — Endpoints chưa deploy  
**Priority**: P2

**Status**: `✅ Fixed` — TASK-010 / SOL-008 (fixed 2026-06-20)

---

## Thông tin

| Field | Value |
|-------|-------|
| **Route** | `/reports` |
| **HTTP Status** | `404 Not Found` |
| **Số lần xuất hiện** | 6 lần |

---

## Endpoints bị lỗi

| Endpoint | Số lần 404 |
|----------|------------|
| `GET /api/v1/reports` | 3 lần |
| `GET /api/v1/reports/templates` | 3 lần |

---

## Root Cause (Backend)

Cả hai endpoints của Report module chưa được implement hoặc route chưa đăng ký. Frontend gọi đồng thời cả danh sách reports và danh sách templates để hiển thị Report Center.

**Error logs**:
```
Failed to load resource: 404 — /api/v1/reports/templates
Failed to load resource: 404 — /api/v1/reports
```

---

## Backend Fix Required

### Cần implement các endpoints

```
GET    /api/v1/reports              # List generated reports
POST   /api/v1/reports              # Generate new report
GET    /api/v1/reports/:id          # Get single report
DELETE /api/v1/reports/:id          # Delete report

GET    /api/v1/reports/templates    # List available report templates
GET    /api/v1/reports/templates/:id # Get single template
```

### Expected Response — Reports List

```json
{
  "data": [
    {
      "id": "report-001",
      "name": "Monthly Security Report - June 2026",
      "template_id": "tmpl-executive",
      "status": "ready",
      "created_at": "2026-06-01T00:00:00Z",
      "download_url": "/api/v1/reports/report-001/download"
    }
  ],
  "pagination": { "page": 1, "pageSize": 20, "total": 5 }
}
```

### Expected Response — Templates List

```json
{
  "data": [
    {
      "id": "tmpl-executive",
      "name": "Executive Summary",
      "description": "High-level security overview for management",
      "format": "pdf"
    },
    {
      "id": "tmpl-technical",
      "name": "Technical Report",
      "description": "Detailed technical findings",
      "format": "pdf"
    }
  ]
}
```
