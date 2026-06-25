# SOL-002: Giải quyết Lỗi Method Not Allowed (405)

## Nguyên nhân
Các HTTP method gửi từ client (hoặc Test Script) không khớp với HTTP Method được định nghĩa tại Backend Router.

## Các API Bị Lỗi & Kế Hoạch Fix

### 1. `POST /api/v1/scans/import`
- **Tình trạng:** Automation test gửi `POST`.
- **Hành động:** Kiểm tra `scan-service/internal/delivery/http/router.go`. Nếu endpoint này được định nghĩa là `PUT` hoặc thiếu, hãy cập nhật thành `POST` và map đúng handler `ImportScan`.

### 2. `GET /api/v1/findings/{id}/notes`
- **Tình trạng:** Automation test gửi `GET`.
- **Hành động:** Kiểm tra `finding-service/internal/delivery/http/router.go`. Tìm kiếm route `/{id}/notes`. Nếu mới chỉ có `POST` (Tạo note), cần bổ sung Method `GET` hoặc nếu test script yêu cầu sai, thì sửa test script.

### 3. `PUT /api/v1/sla/config`
- **Tình trạng:** Automation test gửi `PUT`.
- **Hành động:** Kiểm tra `sla-service/internal/delivery/http/router.go`. Nếu backend đang dùng `PATCH` hoặc `POST`, hãy thống nhất lại sử dụng `PUT` để đáp ứng toàn vẹn configuration.

### 4. `PATCH /api/v1/products/{id}`
- **Tình trạng:** Automation test gửi `PATCH`.
- **Hành động:** Kiểm tra `asset-service/internal/delivery/http/router.go` (hoặc product-service). Backend thường cài đặt `PUT /{id}`, cần thêm alias `r.Patch("/{id}", h.UpdateProduct)`.

### 5. `GET /api/v1/webhooks/stats`
- **Tình trạng:** Automation test gửi `GET`.
- **Hành động:** Gateway proxy `/api/v1/webhooks` tới `notification-service`. Cần check `notification-service/internal/delivery/http/router.go` xem đã có `r.Get("/stats", ...)` hay `r.Get("/stats/hourly", ...)` chưa. Nếu API design là `/stats/hourly`, hãy thêm alias.
