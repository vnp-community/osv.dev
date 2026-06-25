# TASK-V7-002: Khởi tạo các handler bị thiếu trong finding-service

## Mục tiêu
Khắc phục lỗi `404 Not Found` trên các endpoint `/api/v1/risk-acceptances`, `/api/v1/reports`, và `/api/v1/reports/templates` khi `finding-service` chạy ở chế độ độc lập (standalone container).

## Chi tiết lỗi
- File `services/finding-service/embedded.go` (được dùng khi chạy Monolith) đã khởi tạo `RiskAcceptanceHandler` và `ReportHandler`.
- Tuy nhiên, file `services/finding-service/cmd/server/main.go` (được dùng khi chạy service riêng biệt) lại hoàn toàn bỏ quên hai handler này, làm cho router không được wire các API tương ứng.

## Hướng dẫn thực thi
1. Mở file `services/finding-service/cmd/server/main.go`.
2. Khởi tạo `riskAcceptanceRepo` (hoặc các repo/usecase liên quan).
3. Khởi tạo `riskAcceptanceHandler` và `reportHandler`.
4. Truyền hai handler này vào hàm `deliveryhttp.NewRouter(...)`.
5. Đảm bảo cấu trúc code ở `cmd/server/main.go` tương tự như logic khởi tạo ở `embedded.go`.
6. Biên dịch lại (build) `finding-service` và sử dụng tập script python (`tests/client`) để kiểm tra lại lỗi 404 đã được khắc phục.


**STATUS**: COMPLETED
