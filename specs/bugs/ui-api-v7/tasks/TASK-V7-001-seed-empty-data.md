# TASK-V7-001: Seed dữ liệu cho các API trả về kết quả rỗng

## Mục tiêu
Khắc phục lỗi dữ liệu bị rỗng (`[]`, `{}`, hoặc `0`) trên một số API bằng cách thêm dữ liệu mẫu (seeding) vào PostgreSQL. Điều này đảm bảo UI có dữ liệu thực tế để hiển thị thay vì các giao diện rỗng.

## Danh sách API cần xử lý
- `/api/v1/scans/history`: Seed dữ liệu lịch sử quét vào bảng `scans`.
- `/api/v1/sla/overview`: Seed bản ghi `findings` với trạng thái `sla_status` là `breached` hoặc `at_risk`.
- `/api/v1/ai/triage/queue`: Bổ sung dữ liệu mô phỏng AI (seed vào bảng `triage_queue` hoặc thông qua payload gọi AI worker).
- `/api/v1/webhooks/deliveries`: Gửi webhook giả lập để tạo logs trên bảng `webhook_deliveries`.
- `/api/v1/notifications/unread-count`: Cập nhật ít nhất một cảnh báo trong bảng `inapp_alerts` thành `is_read = false`.

## Hướng dẫn thực thi
1. Kiểm tra lại script seed tự động tại `tests/seed/`.
2. Bổ sung các lệnh `INSERT` phù hợp nếu script hiện tại chưa có.
3. Chạy script seed và sử dụng `tests/client` (python) để xác minh API trả về kết quả `total > 0`.


**STATUS**: COMPLETED
