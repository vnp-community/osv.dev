# SOL-003: Giải quyết Lỗi Auth MFA Method Mismatch

## Nguyên nhân
Trong Gateway, cấu hình `proxy.ForwardRewrite` bảo toàn HTTP Method ban đầu. Test script gọi `GET /api/v1/auth/mfa/setup`, gateway forward nó thành `GET /api/v1/auth/totp/setup` và đưa xuống `identity-service`. Tuy nhiên `identity-service` chỉ mở route `POST /api/v1/auth/totp/setup`.
Tương tự cho `/api/v1/auth/mfa/confirm` không tìm thấy route.

## Kế hoạch thực thi (Implementation Plan)

### Bước 1: Sửa Router trong Identity Service
Mở file `services/identity-service/adapter/handler/http/totp_handler.go` và `router.go`.
Thay đổi:
1. Cho phép `GET /api/v1/auth/totp/setup` (hoặc tạo alias `r.Get("/totp/setup", h.Setup)`) để trả về mã QR và secret.
2. Cho phép `POST /api/v1/auth/totp/verify` làm điểm đến của MFA confirm.

### Bước 2: Cập nhật Gateway
Mở file `apps/osv/internal/gateway/router.go`. Đảm bảo có cả hai mappings:
```go
// Setup MFA (GET)
mux.Handle("GET /api/v1/auth/mfa/setup", protected(proxy.ForwardRewrite(
    "/api/v1/auth/mfa/setup",
    "/api/v1/auth/totp/setup",
    "identity-service:8081",
)))

// Confirm MFA (POST)
mux.Handle("POST /api/v1/auth/mfa/confirm", protected(proxy.ForwardRewrite(
    "/api/v1/auth/mfa/confirm",
    "/api/v1/auth/totp/verify", // Tên gốc trong identity-service
    "identity-service:8081",
)))
```

Bằng cách này, HTTP method và URL path sẽ hoàn toàn khớp với kỳ vọng của client.
