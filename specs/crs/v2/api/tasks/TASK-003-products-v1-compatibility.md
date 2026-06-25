# AI Task 003: Products v1 Compatibility (SOL-003)

**Status**: ✅ COMPLETED — 2026-06-18

## Checklist Công Việc

### 1. Xác định Scope
- [x] Liệt kê tất cả các API mà frontend gọi tới (dùng `openapi.yaml`). Tập trung vào các `/api/v1/products` và `/api/v1/engagements`.
- [x] So sánh với danh sách routes hiện có trong `apps/osv/internal/gateway/router.go`. Liệt kê các route còn thiếu.

### 2. API Gateway Routing
- [x] Mở `apps/osv/internal/gateway/router.go`.
- [x] Đăng ký route: Forward các request `GET, POST, PUT, DELETE /api/v1/products/{id}` về `finding-service`.
- [x] Đăng ký route: Forward các request `GET, POST /api/v1/engagements/{id}` về `finding-service`.
- [x] Đảm bảo thứ tự route: literal paths (`/api/v1/products/grades`, `/api/v1/products/types`) phải được khai báo TRƯỚC wildcard paths (`/api/v1/products/{id}`).

### 3. finding-service — Bổ sung Endpoints
- [x] Mở `services/finding-service/internal/delivery/http/product_handler.go`. Kiểm tra các handler `List`, `Get`, `Create`, `Update`, `Delete` đã có chưa.
- [x] Kiểm tra `services/finding-service/internal/delivery/http/engagement_handler.go`. Thêm handler `CreateForProduct` cho `POST /products/{id}/engagements`.
- [x] Cập nhật router của `finding-service` trong `services/finding-service/internal/delivery/http/router.go` với tất cả các routes mới.

### 4. Verify Build
- [x] Chạy `go build ./...` tại `apps/osv/` để đảm bảo không có lỗi compile.
- [x] Chạy `go build ./...` tại `services/finding-service/` để đảm bảo không có lỗi compile.
- [x] Kiểm tra log server trong development để đảm bảo route đã được đăng ký.

## CR-003 Gap Fixes (thực thi 2026-06-18)

### ✅ Gateway — New routes in `apps/osv/internal/gateway/router.go`
- [x] `PATCH /api/v1/products/{id}` → finding-service:8085 (CR-003 partial update)
- [x] `POST /api/v1/products/{id}/engagements` → finding-service:8085 (CR-003 create engagement for product)

### ✅ finding-service — Router updates in `internal/delivery/http/router.go`
- [x] `PATCH /{id}` alias (reuses existing `Update` handler) trong `/api/v1/products` route group
- [x] `POST /{id}/engagements` → `engagement.CreateForProduct` trong `/api/v1/products` route group

### ✅ finding-service — New handler in `internal/delivery/http/engagement_handler.go`
- [x] `CreateForProduct(w, r)` — lấy product_id từ URL path param thay vì request body
  - Nhận `{name, engagement_type, version, build_id, commit_hash, branch_tag, deduplication_on_engagement}` từ body
  - Inject product_id từ chi.URLParam(r, "id")
  - Gọi `getOrCreate.Execute(...)` với đầy đủ thông tin
  - Return 201 Created với EngagementResponse

### ✅ Đã có sẵn (không cần thay đổi)
- `GET /api/v1/products` → finding-service ✅
- `POST /api/v1/products` → finding-service ✅
- `GET /api/v1/products/{id}` → finding-service ✅
- `PUT /api/v1/products/{id}` → finding-service ✅
- `DELETE /api/v1/products/{id}` → finding-service ✅
- `GET /api/v1/products/{id}/engagements` → finding-service ✅
- `GET /api/v1/engagements` → finding-service ✅
- `POST /api/v1/engagements` → finding-service ✅
- `GET /api/v1/engagements/{id}` → finding-service ✅
- `POST /api/v1/engagements/{id}/close` → finding-service ✅
- `POST /api/v1/engagements/{id}/reopen` → finding-service ✅
- `GET /api/v1/engagements/{id}/tests` → finding-service ✅
- `GET /api/v1/products/grades` → finding-service ✅
- `GET /api/v1/products/types` → finding-service ✅

## Build Status
- `apps/osv/internal/gateway/...` → ✅ Build OK
- `services/finding-service/internal/delivery/http/...` → ✅ Build OK

## Hoàn thành
Chạy thử hệ thống trong môi trường development (`make dev` hoặc `docker-compose up`). Kiểm tra bằng curl hoặc Postman xem các API `PATCH /api/v1/products/{id}` và `POST /api/v1/products/{id}/engagements` trả về kết quả mong đợi.
