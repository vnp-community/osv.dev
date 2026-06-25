# Change Request 006: Reports v1 Path, SLA Overview, Jira Config Method

**Cập nhật:** 2026-06-18  
**Status:** New — Các gaps nhỏ nhưng critical: sai path prefix, sai HTTP method, thiếu endpoint.

## 1. Bối cảnh

Ba vấn đề độc lập được nhóm chung do đều nhỏ gọn và có thể fix tại gateway layer:

| # | Endpoint frontend | Backend hiện tại | Trạng thái |
|---|---|---|---|
| 6.1 | `GET /api/v1/reports` | `GET /api/v2/reports` | ❌ **Version prefix mismatch** — frontend gọi v1, gateway route v2 |
| 6.1 | `POST /api/v1/reports` | `POST /api/v2/reports` | ❌ |
| 6.1 | `GET /api/v1/reports/{id}` | `GET /api/v2/reports/{id}` | ❌ |
| 6.1 | `DELETE /api/v1/reports/{id}` | `DELETE /api/v2/reports/{id}` | ❌ |
| 6.1 | `GET /api/v1/reports/{id}/download` | `GET /api/v2/reports/{id}/download` | ❌ |
| 6.2 | `GET /api/v1/sla/overview` | **THIẾU** | ❌ |
| 6.3 | `PUT /api/v1/jira/config` | `POST /api/v1/jira/config` | ❌ **Method mismatch** — frontend dùng PUT, backend POST |

## 2. Chi tiết Thay đổi

### 2.1 [HIGH] Reports — Thêm v1 aliases cho `/api/v1/reports/*`

Frontend hoàn toàn sử dụng `/api/v1/reports/*`. Gateway hiện chỉ có `/api/v2/reports/*`.

**Thêm vào `apps/osv/internal/gateway/router.go`**:

```go
// ═══════════════════════════════════════════════════════
// Reports v1 aliases (finding-service:8085)
// ═══════════════════════════════════════════════════════
mux.Handle("GET /api/v1/reports",
    protected(transform.UserScopeFilter(proxy.Forward("finding-service:8085"))))
mux.Handle("POST /api/v1/reports",
    protected(rl.Limit("5/minute")(proxy.Forward("finding-service:8085"))))
mux.Handle("GET /api/v1/reports/{id}",
    protected(proxy.Forward("finding-service:8085")))
mux.Handle("DELETE /api/v1/reports/{id}",
    protected(proxy.Forward("finding-service:8085")))
mux.Handle("GET /api/v1/reports/{id}/download",
    protected(proxy.ForwardWithTimeout("finding-service:8085", 30*time.Second)))
```

**Finding-service** đã có routes `/api/v2/reports/*`. Cần thêm v1 aliases hoặc gateway forward đúng path.

> **Lưu ý**: Có thể dùng gateway path rewrite `/api/v1/reports` → `/api/v2/reports` thay vì thêm handler mới trong finding-service.

**Frontend report creation request** (`POST /api/v1/reports`):
```json
{
  "name": "Security Report Q2 2026",
  "type": "executive_summary",
  "config": { "product_ids": ["..."], "date_range": "90d" }
}
```

**Response `202 Accepted`** — `Report` schema:
```json
{
  "id": "rpt-001",
  "name": "Security Report Q2 2026",
  "type": "executive_summary",
  "status": "pending",
  "created_at": "2026-06-18T...",
  "created_by": "user-123"
}
```

**Download** — `GET /api/v1/reports/{id}/download?format=pdf`:
- Query param `format`: `pdf | html | csv | xlsx`
- Response: `application/octet-stream` (binary file)

### 2.2 [HIGH] SLA Overview — Thêm `GET /api/v1/sla/overview`

Frontend gọi `GET /api/v1/sla/overview` để hiển thị tổng quan SLA compliance. Endpoint này hoàn toàn **không tồn tại** trong backend.

**Giải pháp**: Implement trong Gateway BFF hoặc sla-service.

**Option A (khuyến nghị) — In-gateway BFF** tương tự `dashBFF.HandleDashboardSLA`:

```go
// apps/osv/internal/gateway/bff/sla_bff.go
func (b *SLABFF) HandleSLAOverview(w http.ResponseWriter, r *http.Request) {
    // Gọi sla-service GET /api/v2/sla-dashboard
    // Trích xuất summary fields
    // Trả về SLASummary schema
}
```

```go
// apps/osv/internal/gateway/router.go
mux.Handle("GET /api/v1/sla/overview", protected(http.HandlerFunc(slaBFF.HandleSLAOverview)))
```

**Response `SLASummary` schema**:
```json
{
  "total_active_findings": 245,
  "compliance_percent": 87.5,
  "breached": 12,
  "at_risk": 31,
  "ok": 202
}
```

**Option B** — Thêm endpoint vào sla-service và route qua gateway:
```go
mux.Handle("GET /api/v1/sla/overview", protected(proxy.Forward("sla-service:8086")))
```

### 2.3 [HIGH] Jira Config — Thêm `PUT /api/v1/jira/config`

Frontend gửi `PUT /api/v1/jira/config` để cập nhật Jira integration config. Backend gateway chỉ có `POST /api/v1/jira/config`.

**Thêm vào gateway**:
```go
mux.Handle("PUT /api/v1/jira/config", adminOnly(proxy.Forward("jira-service:8088")))
```

**Jira-service** cần xử lý `PUT /jira/config` với upsert logic:
```json
// Request body
{
  "url": "https://company.atlassian.net",
  "project_key": "SEC",
  "username": "jira-bot@company.com",
  "api_token": "ATATT3x..."
}
```

Nếu jira-service chỉ hỗ trợ POST (create/upsert), có thể alias tại gateway:
```go
// Alias PUT -> POST cho jira-service (jira-service xử lý upsert logic)
mux.Handle("PUT /api/v1/jira/config", adminOnly(proxy.Forward("jira-service:8088")))
```

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

### Reports v1
1. `GET /api/v1/reports` trả về `{ reports: [...], total: N }` — không bị `404`.
2. `POST /api/v1/reports` với body hợp lệ tạo report, trả về `202 Accepted` với Report object.
3. `GET /api/v1/reports/{id}` trả về Report detail — `200` hoặc `404`.
4. `DELETE /api/v1/reports/{id}` xóa report — `204 No Content`.
5. `GET /api/v1/reports/{id}/download?format=pdf` trả về file PDF — `200 application/octet-stream`.

### SLA Overview
6. `GET /api/v1/sla/overview` trả về `SLASummary` với `total_active_findings`, `compliance_percent`, `breached`, `at_risk`, `ok` — HTTP 200.

### Jira Config
7. `PUT /api/v1/jira/config` với body đầy đủ cập nhật/tạo Jira config — HTTP 200.
8. `GET /api/v1/jira/config` trả về config hiện tại — HTTP 200.
9. `POST /api/v1/jira/config/test` test kết nối Jira — trả về `{ success, message }`.
