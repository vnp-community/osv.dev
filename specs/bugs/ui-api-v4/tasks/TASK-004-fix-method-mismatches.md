# TASK-004: Khắc phục Lỗi Method Mismatches (405)

## Mục tiêu
Điều chỉnh lại các Router Method ở các Microservices để khớp với Spec của Automation Test.

## Hướng dẫn thực thi
Tiến hành kiểm tra và sửa trong các thư mục `internal/delivery/http/router.go` hoặc `*_handler.go`:

1. **Scan Service:**
   - Request từ test: `POST /api/v1/scans/import`
   - Sửa: Đảm bảo có `r.Post("/import", h.ImportScan)` trong `scan_handler.go`.

2. **Finding Service:**
   - Request từ test: `GET /api/v1/findings/{id}/notes`
   - Sửa: Thêm route xử lý `GET` list notes cho 1 finding.

3. **SLA Service:**
   - Request từ test: `PUT /api/v1/sla/config`
   - Sửa: Kiểm tra nếu router đang dùng `POST` hay `PATCH`, đổi thành `PUT`.

4. **Asset/Product Service:**
   - Request từ test: `PATCH /api/v1/products/{id}`
   - Sửa: Thêm alias `r.Patch("/{id}", h.UpdateProduct)` để hỗ trợ cập nhật một phần.

5. **Notification Service:**
   - Request từ test: `GET /api/v1/webhooks/stats`
   - Sửa: Trong `SetupRouter`, sửa hoặc thêm `r.Get("/stats", dh.GetWebhookHourlyStats)`.

6. **Admin / System:**
   - Request từ test: `GET /api/v1/admin/users/{id}`
   - Sửa: Sửa trong gateway admin router (nếu có).

## Acceptance Criteria (AC)
- [x] Hoàn thành rà soát và mapping đúng HTTP method cho 6 endpoints bị lỗi 405.
- [x] Chạy lại Unit Test hoặc Automation test trả về 200/201.
