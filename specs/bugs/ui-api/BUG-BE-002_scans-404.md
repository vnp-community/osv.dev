# BUG-BE-002 — Scans Endpoints Trả 404 (Route Chưa Register)

| Trường | Giá trị |
|---|---|
| **ID** | BUG-BE-002 |
| **Severity** | 🔴 Critical |
| **Priority** | P0 |
| **Component** | Backend / API Gateway / Scan Service |
| **Endpoints** | `GET /api/v1/scans`, `GET /api/v1/scans/scheduled`, `GET /api/v1/scans/{id}` |
| **Phát hiện** | 2026-06-19 |
| **Server** | https://c12.openledger.vn |
| **Status** | Open |

---

## Mô tả

Tất cả Scans endpoints đều trả về `404 page not found`. Route `/api/v1/scans` chưa được đăng ký trong API gateway, khiến toàn bộ chức năng Active Scanning không hoạt động.

## Tái hiện

```bash
# Liệt kê danh sách scans
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/scans
# => HTTP 404 — "404 page not found"

# Filter running scans
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/scans?status=running"
# => HTTP 404 — "404 page not found"

# Scheduled scans
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/scans/scheduled
# => HTTP 404 — "404 page not found"
```

## Expected Response (theo spec)

```json
GET /api/v1/scans → HTTP 200
{
  "scans": [
    {
      "id": "uuid",
      "name": "string",
      "type": "nmap_full | nmap_discovery | zap | agent | import",
      "status": "pending | queued | running | completed | failed | cancelled",
      "targets": ["string"],
      "progress": 0,
      "finding_count": 0,
      "created_by": "string"
    }
  ],
  "total": 0,
  "page": 1,
  "page_size": 10,
  "stats": {
    "active_scans": 0,
    "completed_today": 0,
    "total_findings_today": 0,
    "scheduled_scans": 0
  }
}
```

## Các Endpoint Bị Ảnh Hưởng

| Endpoint | Method | Status |
|---|---|---|
| `/api/v1/scans` | GET | 404 |
| `/api/v1/scans` | POST | Chưa test |
| `/api/v1/scans/scheduled` | GET | 404 |
| `/api/v1/scans/{id}` | GET | 404 |
| `/api/v1/scans/{id}/cancel` | POST | 404 |
| `/api/v1/scans/{id}/results` | GET | 404 |

## Fix

Đăng ký route group `/api/v1/scans` trong API gateway router. Kiểm tra scan service container có đang chạy không:

```bash
docker ps | grep scan
docker logs osv-backend-scan-service-1 --tail 50
```

## Ảnh hưởng

- Trang `/scans` (Scan Dashboard) trả 404, không load được
- Trang `/scans/history` không load
- Không thể xem trạng thái scan đang chạy
- "Running scan" không click được trên UI (do API 404)
