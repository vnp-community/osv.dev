# TASK-V7-004: Cập nhật HTTP Method cho Webhooks Stats

## Mục tiêu
Khắc phục lỗi `405 Method Not Allowed` khi frontend gọi API `/api/v1/webhooks/stats`.

## Chi tiết lỗi
Theo báo cáo từ `tests/client`, API này nhận request với method `GET`, nhưng cấu hình backend có thể đã thiết lập nhầm sang một HTTP method khác (ví dụ `POST` hoặc `PUT`), dẫn đến router trả về mã lỗi 405.

## Hướng dẫn thực thi
1. Kiểm tra file cấu hình route của notification service tại `services/notification-service/internal/delivery/http/router.go`.
2. Định vị route map tới handler thống kê của webhook (có thể là `r.Get("/api/v1/webhooks/stats", ...)` hoặc tương đương).
3. Sửa lại Method đảm bảo nó đồng bộ với `api_endpoints.md` (cụ thể là method `GET`).
4. Biên dịch lại service và kiểm chứng bằng test.


**STATUS**: COMPLETED
