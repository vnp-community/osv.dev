# TASK-V7-005: Khắc phục lỗi Crash Nil Pointer Profile Handlers

## Mục tiêu
Fix lỗi `500 Internal Server Error` với phản hồi rỗng ở các API lấy session người dùng (`/api/v1/profile/sessions`) và cài đặt thông báo (`/api/v1/profile/notifications/settings`).

## Chi tiết lỗi
- Handler xử lý profile trong `services/identity-service/adapter/handler/http/profile_handler.go` yêu cầu các dependency `sessionRepo` và `notifRepo`.
- Tuy nhiên, trong quá trình khởi tạo ứng dụng tại `services/identity-service/embedded.go` (hàm `WireEmbedded`), cấu trúc `RouterDeps` truyền vào `NewRouter` đang bị **thiếu** khai báo cho `SessionRepo` và `NotifPrefRepo`.
- Kết quả là chúng mang giá trị `nil`, và khi handler sử dụng chúng sẽ gây ra panic Nil Pointer Dereference, được Catch bởi HTTP server và trả về status code 500.

## Hướng dẫn thực thi
1. Mở file `services/identity-service/embedded.go`. Tìm đến khu vực khởi tạo biến `router := httpHandler.NewRouter(httpHandler.RouterDeps{...})`.
2. Bổ sung hai trường bị thiếu vào struct:
   ```go
   SessionRepo:   sessionRepo,
   NotifPrefRepo: pgRepo.NewNotifPrefRepo(dbPool), // Kiểm tra tên hàm khởi tạo cho NotifPrefRepo nếu cần
   ```
3. Nếu cần thiết, kiểm tra thêm trong `services/identity-service/cmd/server/main.go` xem có gặp lỗi thiếu hụt tương tự hay không.
4. Biên dịch lại service và xác nhận API đã trả về đúng mã trạng thái 200 kèm body (chứa mảng rỗng `[]` nếu chưa có dữ liệu).


**STATUS**: COMPLETED
