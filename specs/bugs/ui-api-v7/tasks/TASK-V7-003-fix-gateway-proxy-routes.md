# TASK-V7-003: Khắc phục lỗi định tuyến ở Gateway Proxy

## Mục tiêu
Fix các lỗi HTTP 404 cho một loạt các API do cấu hình Proxy của Gateway Monolith (`osv_routes.go`) bị lệch/thiếu so với bản Microservices (`osv/internal/gateway/router.go`).

## Các API bị ảnh hưởng và nguyên nhân
1. **`/api/v1/products`**: Hiện Gateway đang map toàn bộ prefix này về `finding-service` nhằm mục đích trỏ đúng cho `/grades`. Tuy nhiên, các route gốc (`/products`) lại nằm bên `product-service` nên gặp 404.
2. **`/api/v1/jira/*` và `/api/v1/integrations/jira`**: Router `ovs_routes.go` hoàn toàn thiếu cấu hình để proxy các request này sang `jira-service`.
3. **`/api/v1/audit-log` và `/api/v1/search/*`**: Thiếu cấu hình upstream tương tự để gọi sang `search-service` hoặc Identity log.

## Hướng dẫn thực thi
1. Mở file `services/gateway-service/internal/proxy/ovs_routes.go`.
2. Sửa lại mapping của `products` thành 2 rule phân biệt (Rule cụ thể hơn phải đứng trước):
   ```go
   {PathPrefix: "/api/v1/products/grades", Upstream: "finding-service"},
   {PathPrefix: "/api/v1/products",        Upstream: "product-service"},
   ```
3. Copy toàn bộ các rule proxy cho JIRA từ `apps/osv/internal/gateway/router.go` (lưu ý có sử dụng alias rewrite `integrations/jira` -> `jira/config` nếu cần) và thêm vào `OVSRoutes`.
4. Bổ sung các rule cho `audit-log` và `search`.
5. Chạy lại test suite để xác minh không còn API nào trả về lỗi 404 (do thiếu route) nữa.


**STATUS**: COMPLETED
