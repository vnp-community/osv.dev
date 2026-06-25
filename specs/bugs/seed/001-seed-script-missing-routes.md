# BUG-SEED-001: Missing Routes \u0026 Upstream Config Errors in Gateway

**Status:** Open
**Component:** `apps/osv` (Gateway) / `services/*`
**Reported From:** `02_push_seed_data.py` (Seed script)

Dựa trên phân tích log lỗi tại `tests/seed/report/push_errors.log` và đối chiếu với cấu trúc Gateway (`apps/osv/internal/gateway/router.go`) cùng `01-architecture.md`, dưới đây là danh sách các bugs gây ra tình trạng failed khi seed dữ liệu lên môi trường dev (`c12.openledger.vn`):

## 1. Identity Service: Lỗi 404 Admin Users
**Log lỗi:** `user ... failed: HTTP 404 — {"error":"not_found","message":"no route matches this path"}`
- **URL Path bị lỗi:** `POST /api/v1/admin/users` (và `/bulk`)
- **Nguyên nhân:**
  Trong spec `TASK-SEED-001-D` đã định nghĩa rõ các endpoint `POST /api/v1/admin/users` và `POST /api/v1/admin/users/bulk` và thậm chí file spec đã được đánh dấu `COMPLETED`. Tuy nhiên, khi kiểm tra `apps/osv/internal/gateway/router.go`, các route này **hoàn toàn chưa được khai báo**. Hiện Gateway chỉ map `GET /api/v1/admin/users` và `POST /api/v1/admin/users/invite`. Điều này làm cho Gateway từ chối request seed với mã 404.
- **Đề xuất fix:** Áp dụng đầy đủ code từ `TASK-SEED-001-D` (Bước 5), bổ sung `mux.Handle("POST /api/v1/admin/users", ...)` và `mux.Handle("POST /api/v1/admin/users/bulk", ...)` vào Gateway router. Đồng thời rà soát xem `identity-service` đã thực sự có các hàm handler này chưa.

## 2. Identity Service: Lỗi 404 API Keys (Admin cấp)
**Log lỗi:** `api_key '...' failed: HTTP 404 — {"error":"not_found","message":"no route matches this path"}`
- **URL Path bị lỗi:** `POST /api/v1/admin/users/{id}/api-keys`
- **Nguyên nhân:**
  Giống như lỗi số 1, `TASK-SEED-001-D` (Bước 3 và 4) ĐÃ LÊN KẾ HOẠCH cho handler `POST /api/v1/admin/users/{id}/api-keys` (hàm `CreateAPIKeyForUser` trong `admin_handler.go`). Mặc dù spec báo `COMPLETED`, code thực tế trong `identity-service/adapter/handler/http/admin_handler.go` và `apps/osv/internal/gateway/router.go` lại **thiếu hoàn toàn** hàm và route này.
- **Đề xuất fix:** Implement handler `CreateAPIKeyForUser` trong `admin_handler.go`, bổ sung route vào identity router và map nó lên Gateway theo chuẩn của `TASK-SEED-001-D`.

## 3. Product/Finding Service: Lỗi 404 Entity Creation
**Log lỗi:** `product_type ... failed: HTTP 404 — 404 page not found` (Tương tự cho `product`, `engagement`, `test`, `finding`, `finding_group`, `finding_note`).
- **URL Path bị lỗi:** 
  - `POST /api/v2/product-types`
  - `POST /api/v2/products`
  - `POST /api/v2/engagements`
  - `POST /api/v2/tests`
  - `POST /api/v2/finding-groups`
  - `POST /api/v2/findings`
  - `POST /api/v2/findings/{id}/notes`
- **Nguyên nhân:**
  Gateway đã map các route POST `/api/v2/product-types`, `/products`, `/engagements`, `/tests` tới `finding-service:8085`. Nhưng `finding-service` lại trả về `404 page not found`. Nguyên nhân là `finding-service` mới chỉ implement API Bulk (`POST /api/v2/product-types/bulk`) theo `TASK-SEED-002`, mà **quên (hoặc chưa) implement** các handler POST đơn lẻ cho các entity này trong file `internal/delivery/http/router.go` của `finding-service`.
- **Đề xuất fix:** Bổ sung các API POST (Single Create) vào `finding-service` hoặc yêu cầu Seed script chuyển sang dùng Bulk creation API.

## 4. SLA \u0026 Notification Services: Lỗi 404 Missing Handlers
**Log lỗi:** `sla_config ...`, `notif_rule ...`, `subscription ...`, `webhook ...` failed: HTTP 404
- **URL Path bị lỗi:**
  - `POST /api/v2/sla-configurations`
  - `POST /api/v2/notification-rules`
  - `POST /api/v2/subscriptions`
  - `POST /api/v1/webhooks`
- **Nguyên nhân:**
  Gateway đã khai báo proxy tới `sla-service:8086` và `notification-service:8087`. Việc server trả về `404 page not found` (không phải JSON error response từ Gateway) có nghĩa là request đã đi tới các service này nhưng service lại chưa map route tương ứng (chưa implement).
- **Đề xuất fix:** Hoàn thiện code trong `sla-service` và `notification-service` để handle các method POST cho config.

## 5. Asset Service: Lỗi 502 Upstream Not Configured
**Log lỗi:** `asset ... failed: HTTP 502 — {"error":"upstream_not_configured"}`
- **URL Path bị lỗi:** `POST /api/v2/assets`
- **Nguyên nhân:**
  Lỗi 502 với thông báo đặc thù của Gateway ReverseProxy. Lỗi này xảy ra khi Gateway nhận request nhưng khi thực hiện resolve DNS / Proxy tới `asset-service:8091` thì không thấy service đó chạy trong mạng của Docker Compose. Do `asset-service` chưa được build hoặc chưa được bật trong file `docker-compose.server.yml` trên máy chủ `c12.openledger.vn`.
- **Đề xuất fix:** Deploy `asset-service` trên host, cấu hình đúng file `docker-compose.server.yml` và đảm bảo service chạy ở port `8091`.
