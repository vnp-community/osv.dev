# TASK-002: Khởi tạo AlertsHandler cho Notification Service

## Mục tiêu
Fix lỗi 404 cho các endpoint V1 Notification do thiếu dependency khi nhúng.

## Các Endpoints Bị Ảnh Hưởng
- `GET /api/v1/notifications`
- `GET /api/v1/notifications/unread-count`
- `POST /api/v1/notifications/mark-all-read`

## Hướng dẫn thực thi

1. **Kiểm tra Notification Router:**
   - Mở file `services/notification-service/embedded.go`
   - Phát hiện dòng: `r := deliverhttp.SetupRouter(whHandler, shHandler, ihHandler, nil, nil, rhHandler, dhHandler)` (Tham số thứ 4 là `AlertsHandler` đang là `nil`).

2. **Khởi tạo Dependency:**
   - Trong `embedded.go`, cần khởi tạo UseCase và Handler cho Alerts.
   - Thêm logic khởi tạo `NotificationRepo`, `AlertUseCase` và `AlertsHandler` (nếu có trong `internal/delivery/http`).
   - Nếu Notification Service hoàn toàn chưa code logic cho phần in-app notification này (AlertsHandler chưa tồn tại), cần phải tạo mới `AlertsHandler` chứa các API endpoint trên với mock return data hợp lệ, để tránh lỗi 404.

3. **Truyền vào SetupRouter:**
   - Sửa dòng `SetupRouter` truyền `AlertsHandler` vào thay vì `nil`.

## Acceptance Criteria (AC)
- [x] Code compile thành công.
- [x] Lỗi 404 biến mất đối với các endpoint `v1/notifications` khi chạy automation test.
