# BUG-014: Profile Management — Toàn Bộ Chưa Implement

**ID**: BUG-014  
**Domain**: Profile / User Settings  
**Mức độ**: 🔴 CRITICAL  
**Loại**: `404 Not Found`  
**Phát hiện**: 2026-06-23  
**Trạng thái**: OPEN  

## Endpoints Bị Lỗi

| Method | Endpoint | HTTP Status |
|---|---|---|
| `GET`   | `/api/v1/profile`                        | **404** |
| `PATCH` | `/api/v1/profile`                        | **404** |
| `POST`  | `/api/v1/profile/change-password`        | **404** |
| `GET`   | `/api/v1/profile/sessions`               | **404** |
| `GET`   | `/api/v1/profile/notifications/settings` | **404** |
| `PUT`   | `/api/v1/profile/notifications/settings` | **404** |

## Endpoint Đang Hoạt Động (Liên Quan)

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v1/auth/me` | ✅ **200** |

> **Quan sát**: Server có dữ liệu user (qua `/auth/me`) nhưng chưa expose qua `/profile`. `/profile` thường là alias hoặc superset của `/auth/me`.

## Tác Động

**CRITICAL vì**: `UserProfile.tsx` là trang cài đặt cá nhân của mọi user. Toàn bộ tab trong trang này không hoạt động:

- **Profile tab**: Không load được thông tin user để hiển thị.
- **Security tab**: Button "Change Password" fail.
- **Sessions tab**: Danh sách sessions trống — user không biết đang login từ đâu.
- **Notifications tab**: Không load/save cài đặt notification.

## Giải Pháp Đề Xuất

```
GET /api/v1/profile
→ 200 OK
{
  "id": "...",
  "name": "Carol Anderson",
  "email": "carol@company.com",
  "role": "admin",
  "department": "Security Operations",
  "job_title": "CISO",
  "phone": "+84 901 234 567",
  "timezone": "Asia/Ho_Chi_Minh",
  "mfa_enabled": true,
  "avatar_url": null,
  "created_at": "..."
}

PATCH /api/v1/profile
Body: { "name": "...", "department": "...", "phone": "...", "timezone": "..." }
→ 200 OK { updated profile }

POST /api/v1/profile/change-password
Body: { "current_password": "...", "new_password": "...", "confirm_password": "..." }
→ 200 OK { "success": true }
→ 400 { "error": "Current password incorrect" }

GET /api/v1/profile/sessions
→ 200 OK
{
  "items": [{
    "id": "...",
    "device": "Chrome on macOS",
    "ip": "192.168.1.100",
    "location": "Ho Chi Minh City, VN",
    "last_active": "2026-06-23T12:00:00Z",
    "current": true
  }],
  "total": 3
}

GET /api/v1/profile/notifications/settings
→ 200 OK
{
  "items": [{
    "id": "critical_findings",
    "label": "Critical Finding Alerts",
    "desc": "...",
    "enabled": true
  }]
}

PUT /api/v1/profile/notifications/settings
Body: { "items": [{ "id": "critical_findings", "enabled": false }] }
→ 200 OK { "items": [...] }
```

## File UI Bị Ảnh Hưởng

- `src/features/auth/components/UserProfile.tsx`
- `src/features/auth/hooks/useProfile.ts` (mới tạo trong Phase 3 refactor)
- `src/mocks/handlers/profile.handlers.ts` (MSW handler đã ready)

## Lưu Ý Quan Trọng

Hooks `useSessions` và `useNotificationSettings` trong `useProfile.ts` đã được tạo và sẵn sàng consume API. MSW handler `profile.handlers.ts` cũng đã có fixture data. Chỉ cần server implement là UI sẽ hoạt động ngay.
