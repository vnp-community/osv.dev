# BUG-001: Embedded Mock Routers Returning 404

## Overview
Sau khi deploy backend thành công (bao gồm gateway và các services), kết quả chạy script `test_all_endpoints.py` vẫn báo lỗi `404 Not Found` ở 17 endpoints.
Nguyên nhân gốc rễ là do cấu trúc nhúng (`embedded`) của các services khi chạy trong orchestrator `apps/osv`.

File `cmd/server/embed.go` của hầu hết các services (ví dụ: `search-service`, `ai-service`, `notification-service`, `jira-service`) đang sử dụng `http.NewServeMux()` thuần túy và chỉ đăng ký đúng 1 endpoint `/health`. Các file này **chưa hề gọi** hàm `NewRouter()` thật của service đó, biến toàn bộ bản build nhúng thành các Mockup Server.

Theo quy định: *"Mọi mock data phải lấy từ server và dữ liệu trả về thông qua server API, do đó, code với mock data được coi là các bugs."*

## Các API Bị Ảnh Hưởng (404)

### Nhóm Notifications (`notification-service`)
- `GET /api/v1/notifications`
- `GET /api/v1/notifications/unread-count`
- `POST /api/v1/notifications/mark-all-read`

### Nhóm AI (`ai-service`)
- `GET /api/v1/ai/triage/queue`
- `GET /api/v1/ai/enrichment`
- `POST /api/v1/ai/enrichment/trigger`
- `GET /api/v1/ai/insights`

### Nhóm Jira Integrations (`jira-service` / `integration-service`)
- `GET /api/v1/jira/config`
- `POST /api/v1/jira/config/test`
- `GET /api/v1/integrations/jira`
- `PUT /api/v1/integrations/jira`

### Nhóm Search & Core (`search-service` / `gateway-service`)
- `GET /api/v1/search/recent`
- `GET /api/v1/search/suggested`
- `GET /api/v1/audit-log`
- `GET /api/v1/products/grades`
- `GET /api/v2/browse`
- `GET /api/v2/dbinfo`

## Giải pháp đề xuất
Chỉnh sửa file `cmd/server/embed.go` của TẤT CẢ các service (trừ `identity-service` vì `identity-service/embedded.go` đang sử dụng `chi.Router` thật):
Thay vì:
```go
mux := http.NewServeMux()
mux.HandleFunc("/health", ...)
```
Phải refactor để gọi đến `adapter/handler/http` hoặc `delivery/http` `NewRouter(...)` tương ứng, sao cho bản build nhúng (embedded) hoạt động giống hệt như bản standalone binary.
