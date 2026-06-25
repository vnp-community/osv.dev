# BUG-003: Lỗi Mismatch Method tại Auth MFA

## Overview
Hai endpoints liên quan đến MFA Setup & Confirm đều báo `404 Not Found` mặc dù đã được thêm vào gateway trong `SOL-001`.

## Các API Bị Ảnh Hưởng
- `GET /api/v1/auth/mfa/setup`
- `POST /api/v1/auth/mfa/confirm`

## Phân tích nguyên nhân
### 1. `GET /api/v1/auth/mfa/setup`
Gateway `apps/osv/internal/gateway/router.go` được cấu hình proxy như sau:
```go
mux.Handle("GET /api/v1/auth/mfa/setup", protected(proxy.ForwardRewrite(
    "/api/v1/auth/mfa/setup",
    "/api/v1/auth/totp/setup",
    "identity-service:8081",
)))
```
Tuy nhiên, trong `identity-service/adapter/handler/http/totp_handler.go`, endpoint `/api/v1/auth/totp/setup` lại được đăng ký dưới dạng `POST` (Tạo secret mới + gen QR):
```go
// Setup handles POST /api/v1/auth/totp/setup
```
Hệ quả: Khi request `GET /api/v1/auth/mfa/setup` tới Gateway, nó proxy y nguyên method `GET` tới Identity Service. Do Identity Service chỉ hỗ trợ `POST` ở route `/api/v1/auth/totp/setup`, nên trả về lỗi (405 Method Not Allowed hoặc 404).

### 2. `POST /api/v1/auth/mfa/confirm`
Gateway chưa định nghĩa mapping hoặc identity-service đang ánh xạ sai path. (Identity service đang có `/api/v1/auth/totp/verify` dưới dạng `POST`).

## Giải pháp đề xuất
1. Cần sửa logic gateway mapping để ánh xạ `GET /api/v1/auth/mfa/setup` -> API chuẩn trong identity-service, hoặc sửa lại giao thức nếu specs quy định là POST.
2. Thêm routing `POST /api/v1/auth/mfa/confirm` -> `/api/v1/auth/totp/verify` của `identity-service`.
