# Giải pháp Xử lý Bug & Seed UI API v7

Dựa trên quá trình rà soát mã nguồn ở `services/` và `apps/osv/`, dưới đây là danh sách phân tích nguyên nhân và giải pháp kỹ thuật cụ thể cho từng lỗi được đề cập trong `bug_report_missing_and_empty.md`.

---

## 1. Dữ liệu trống / 0 (Cần Seed dữ liệu)
Các API này không bị lỗi logic (trả về HTTP 200 OK) nhưng dữ liệu rỗng làm UI không hiển thị đúng thiết kế.

- **`/api/v1/scans/history`**: 
  - **Giải pháp:** Cần chạy script Seed để tạo vài bản ghi lịch sử quét (scan history) vào bảng `scans` (status `completed` hoặc `failed`).
- **`/api/v1/sla/overview`**: 
  - **Giải pháp:** Dữ liệu này phụ thuộc vào `finding_repo_sla.go`. Cần seed vài bản ghi `findings` với `sla_status` là `breached` hoặc `at_risk`.
- **`/api/v1/ai/triage/queue`**: 
  - **Giải pháp:** Liên quan đến `ai-service`. Cần seed data vào bảng `triage_queue` hoặc cung cấp payload phù hợp để AI AI-Triage worker sinh ra queue.
- **`/api/v1/webhooks/deliveries`**: 
  - **Giải pháp:** Cần dùng script gọi tới một webhook tự định nghĩa để tạo ra delivery logs, từ đó điền vào bảng `webhook_deliveries` của `notification-service`.
- **`/api/v1/notifications/unread-count`**: 
  - **Giải pháp:** Mặc dù bảng `inapp_alerts` đã có dữ liệu, nhưng có thể các alert đó đã bị đánh dấu `is_read = true` hoặc truy vấn tính count không map đúng `user_id`. Cần check/seed lại `is_read = false`.

---

## 2. Lỗi HTTP (404 Not Found / 405 Method Not Allowed / Lỗi Router)
Đây là các API bị thiếu routing proxy ở Gateway hoặc thiếu đăng ký Handler tại Service.

- **`/api/v1/risk-acceptances`**
  - **Nguyên nhân:** File `embedded.go` của `finding-service` có khởi tạo `RiskAcceptanceHandler`, nhưng file `cmd/server/main.go` của `finding-service` lại **không** wire handler này vào trong router. Do đó khi container `finding-service` chạy độc lập, nó không nhận route này.
  - **Giải pháp:** Cập nhật `finding-service/cmd/server/main.go`, khởi tạo `riskAcceptanceRepo`, truyền `riskAcceptanceHandler` vào router.
- **`/api/v1/products`**
  - **Nguyên nhân:** Gateway proxy hiện forward toàn bộ `/api/v1/products` sang `finding-service`. Tuy nhiên, khi chạy độc lập, `finding-service` (với `productHandler == nil`) chỉ xử lý đúng endpoint con `/grades`, còn lại các endpoint CRUD gốc không có mặt (chúng ở bên `product-service`).
  - **Giải pháp:** Tại `services/gateway-service/internal/proxy/ovs_routes.go`, thêm mapping chi tiết:
    ```go
    {PathPrefix: "/api/v1/products/grades", Upstream: "finding-service"},
    {PathPrefix: "/api/v1/products",        Upstream: "product-service"},
    ```
- **`/api/v1/reports` & `/api/v1/reports/templates`**
  - **Nguyên nhân:** Tương tự risk acceptances, `reportHandler` được wire đúng trong `embedded.go` nhưng bị sót trong file `finding-service/cmd/server/main.go`.
  - **Giải pháp:** Bổ sung `reportHandler` vào hàm khởi tạo Router trong `cmd/server/main.go` của `finding-service`.
- **`/api/v1/jira/config` & `/api/v1/integrations/jira`**
  - **Nguyên nhân:** Router của gateway monolith (`services/gateway-service/internal/proxy/ovs_routes.go`) **thiếu hoàn toàn** các mapping proxy cho nhánh `jira-service` (trong khi bản microservices ở `apps/osv/internal/gateway/router.go` lại có).
  - **Giải pháp:** Sao chép các cấu hình routing cho JIRA từ `osv/internal/gateway/router.go` sang `ovs_routes.go`.
- **`/api/v1/audit-log`, `/search/recent`, `/search/suggested`**
  - **Nguyên nhân:** Tương tự, thiếu proxy mapping trong mảng `OVSRoutes` tại `gateway-service/internal/proxy/ovs_routes.go` để chuyển hướng tới `search-service` và identity/audit-log.
  - **Giải pháp:** Bổ sung proxy upstream configs.
- **`/api/v1/webhooks/stats` (405 Method Not Allowed)**
  - **Nguyên nhân:** Lỗi method HTTP. Request gửi `GET`, nhưng code định nghĩa route có thể là `POST` hoặc ngược lại.
  - **Giải pháp:** Sửa lại khai báo HTTP Method tại `notification-service/internal/delivery/http/router.go`.

---

## 3. Lỗi Server (500 Internal Error / Crash do Nil Pointer)
- **`/api/v1/profile/sessions` & `/api/v1/profile/notifications/settings`**
  - **Nguyên nhân:** Trong `services/identity-service/embedded.go` (và có thể cả `cmd/server/main.go`), khi khởi tạo struct `RouterDeps`, lập trình viên đã quên truyền thuộc tính `SessionRepo` và `NotifPrefRepo`. Hậu quả là chúng mang giá trị `nil`. Khi `ProfileHandler` được gọi và thực thi `h.sessionRepo.ListByUserID` hoặc `h.notifRepo.GetPreferences`, hệ thống ném ra lỗi **Nil Pointer Dereference**, gây Panic và trả về HTTP 500 với nội dung trống.
  - **Giải pháp:** Tại `services/identity-service/embedded.go` (dòng 124+), sửa hàm `WireEmbedded` để truyền đầy đủ các trường bị thiếu:
    ```go
    router := httpHandler.NewRouter(httpHandler.RouterDeps{
        // ...
        SessionRepo:   sessionRepo,
        NotifPrefRepo: pgRepo.NewNotifPrefRepo(dbPool), // cần tạo repo này trước
        // ...
    })
    ```
    *(Kiểm tra tương tự với `identity-service/cmd/server/main.go`)*.
