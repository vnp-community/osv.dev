# BUG-005: Scans — History 404, Import 405

**ID**: BUG-005  
**Domain**: Scanning  
**Mức độ**: 🔴 HIGH  
**Phát hiện**: 2026-06-23  
**Trạng thái**: OPEN  

## Endpoints Bị Lỗi

| Method | Endpoint | HTTP Status | Loại Lỗi |
|---|---|---|---|
| `GET`  | `/api/v1/scans/history` | **404** | Route missing |
| `POST` | `/api/v1/scans/import`  | **405** | Method Not Allowed |

## Endpoints Đang Hoạt Động

| Method | Endpoint | Status |
|---|---|---|
| `GET`  | `/api/v1/scans`              | ✅ 500 (route OK, server error) |
| `POST` | `/api/v1/scans`              | ✅ 400 |
| `GET`  | `/api/v1/scans/scheduled`    | ✅ 200 |
| `GET`  | `/api/v1/scans/{id}`         | ✅ pass |
| `GET`  | `/api/v1/scans/{id}/results/nmap` | ✅ pass |
| `GET`  | `/api/v1/scans/{id}/results/zap`  | ✅ pass |

## Mô Tả

### `/scans/history` — 404
`ScanHistory.tsx` dùng endpoint này để hiển thị lịch sử scan đã hoàn thành. Server không có route này.

> ⚠️ **Xung đột**: Path `/scans/history` là static nhưng router có thể đang bắt nó nhầm vào route `/scans/{id}` (với `id = "history"`). Đây là pattern routing priority bug phổ biến.

### `/scans/import` — 405
Route tồn tại nhưng không accept `POST`. Có thể server đang dùng `PUT` hoặc `multipart/form-data` với method khác.

## Giải Pháp Đề Xuất

```
GET /api/v1/scans/history?page=1&limit=20&status=completed
→ 200 OK
{
  "items": [{ "id": "...", "target": "...", "status": "completed", "completed_at": "...", "findings_count": 5 }],
  "total": 43,
  "page": 1
}
```

Với `/scans/import`: Kiểm tra lại router — nếu dùng PUT, UI cần đổi method.
