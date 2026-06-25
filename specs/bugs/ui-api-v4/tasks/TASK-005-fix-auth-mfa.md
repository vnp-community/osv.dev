# TASK-005: Khắc phục lỗi Auth MFA Mismatch

## Mục tiêu
Fix lỗi 404 cho `mfa/setup` và `mfa/confirm`.

## Hướng dẫn thực thi

1. **Sửa Gateway (`apps/osv/internal/gateway/router.go`)**:
   - Map `POST /api/v1/auth/mfa/confirm` thành `POST /api/v1/auth/totp/verify` xuống `identity-service:8081`.

2. **Sửa Identity Service (`services/identity-service/adapter/handler/http/totp_handler.go`)**:
   - Hàm xử lý TOTP Setup hiện tại là POST (Tạo secret và gửi QR).
   - Test Script đang gửi `GET /auth/mfa/setup`. 
   - Giải pháp: Thêm alias `r.Get("/totp/setup", h.Setup)` trong `identity-service/adapter/handler/http/router.go` để hỗ trợ request GET cho setup MFA.

## Acceptance Criteria (AC)
- [x] Test Script không còn báo 404 khi gọi MFA setup và confirm.
- [x] Gateway định tuyến đúng method và route cho MFA.
