# BUG-013: JIRA Integration — Tất Cả Routes Chưa Implement

**ID**: BUG-013  
**Domain**: Integrations  
**Mức độ**: 🔴 HIGH  
**Loại**: `404 Not Found`  
**Phát hiện**: 2026-06-23  
**Trạng thái**: OPEN  

## Endpoints Bị Lỗi

| Method | Endpoint | HTTP Status |
|---|---|---|
| `GET`  | `/api/v1/jira/config`       | **404** |
| `POST` | `/api/v1/jira/config/test`  | **404** |
| `GET`  | `/api/v1/integrations/jira` | **404** |
| `PUT`  | `/api/v1/integrations/jira` | **404** |

## Mô Tả

Toàn bộ JIRA integration không có backend. `JiraConfig.tsx` sẽ render form rỗng và không thể lưu config.

### Vấn Đề Trùng Lặp Path

Có 2 paths được spec cho cùng mục đích:
- `/api/v1/jira/config` — path cũ (legacy?)
- `/api/v1/integrations/jira` — path mới (RESTful hơn)

Cần **thống nhất một path duy nhất** và cập nhật cả `api_endpoints.md` lẫn `endpoints.ts`.

## Giải Pháp Đề Xuất

Dùng path `/api/v1/integrations/jira` (chuẩn hơn):

```
GET /api/v1/integrations/jira
→ 200 OK
{
  "enabled": true,
  "host": "https://jira.company.com",
  "project_key": "SEC",
  "api_token": "***masked***",
  "auto_create": true,
  "default_priority": "High"
}

PUT /api/v1/integrations/jira
Body: { "host": "...", "api_token": "...", "project_key": "...", ... }
→ 200 OK { updated config }

POST /api/v1/integrations/jira/test
Body: { "host": "...", "api_token": "..." }
→ 200 OK { "success": true, "jira_version": "9.4.1" }
→ 400 { "error": "Connection failed: ..." }
```

## Actions Cần Thiết

1. Server implement `/api/v1/integrations/jira` GET + PUT + POST `/test`
2. Remove duplicate `/jira/config` entries khỏi `api_endpoints.md`
3. Update `src/shared/api/endpoints.ts` để dùng nhất quán path mới

## File UI Bị Ảnh Hưởng

- `src/features/integrations/components/JiraConfig.tsx`
- `src/shared/api/endpoints.ts` (field `integrations.jira`)
