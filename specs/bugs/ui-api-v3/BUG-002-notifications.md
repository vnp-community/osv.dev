# BUG-002: Notifications — List, Unread Count, Mark All Read Chưa Implement

**ID**: BUG-002  
**Domain**: Notifications  
**Mức độ**: 🔴 HIGH  
**Loại**: `404 Not Found`  
**Phát hiện**: 2026-06-23  
**Trạng thái**: OPEN  

## Endpoints Bị Lỗi

| Method | Endpoint | HTTP Status |
|---|---|---|
| `GET`  | `/api/v1/notifications`              | **404** |
| `GET`  | `/api/v1/notifications/unread-count` | **404** |
| `POST` | `/api/v1/notifications/mark-all-read`| **404** |

## Endpoints Đang Hoạt Động

| Method | Endpoint | HTTP Status |
|---|---|---|
| `GET`  | `/api/v1/notifications/stream` | ✅ 200 (SSE) |
| `PATCH`| `/api/v1/notifications/{id}/read` | ✅ (pass — 404 do test ID) |

## Mô Tả

Server chỉ implement SSE stream và per-notification PATCH, nhưng thiếu 3 endpoints core:

1. **List notifications** — `NotificationCenter.tsx` cần endpoint này để load danh sách notifications với pagination.
2. **Unread count** — Header badge counter không hoạt động.
3. **Mark all read** — Button "Đánh dấu tất cả đã đọc" sẽ fail.

## Tác Động

- `NotificationCenter.tsx` render rỗng (không có data để hiển thị).
- Bell icon trên header không hiển thị số unread.
- User phải mark từng notification một — không thể bulk mark.

## Endpoint Spec Đề Xuất

```
GET /api/v1/notifications?page=1&limit=20&read=false
→ 200 OK
{
  "items": [{ "id": "...", "title": "...", "type": "critical_finding", "read": false, "created_at": "..." }],
  "total": 42,
  "unread_count": 5
}

GET /api/v1/notifications/unread-count
→ 200 OK { "count": 5 }

POST /api/v1/notifications/mark-all-read
→ 200 OK { "marked": 5 }
```

## File UI Bị Ảnh Hưởng

- `src/features/notifications/components/NotificationCenter.tsx`
- `src/features/notifications/hooks/useNotifications.ts`
- `src/shared/components/layout/Header.tsx` (bell icon badge)
