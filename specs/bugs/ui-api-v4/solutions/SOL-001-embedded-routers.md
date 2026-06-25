# SOL-001: Giải quyết bẫy Embedded Routers (Lỗi 404)

## Nguyên nhân
Trong kiến trúc Modular Monolith của `apps/osv`, có hai vấn đề lớn khiến các endpoint trả về `404 Not Found`:
1. **Quên Mount Route**: Trong `search-service/embedded.go`, hàm `WireEmbedded` chỉ mount `mux.Handle("/api/v2/cves", ...)` mà quên mount `/api/v1/search/recent` và `/api/v1/search/suggested`.
2. **Thiếu Dependency (truyền nil)**: Trong `notification-service/embedded.go`, hàm `WireEmbedded` gọi `SetupRouter(..., nil, nil, ...)` trong đó truyền `nil` cho `AlertsHandler`. Điều này khiến file router không khởi tạo các endpoint `/api/v1/notifications/*`.
3. **Double-Prefix Route**: Trong `ai-service/cmd/server/embed.go`, `aiRouter` bên trong đã có `r.Route("/api/v1/ai")`, nhưng lại được mount dưới dạng `mainMux.Handle("/api/v1/ai/", aiRouter)`. Hệ quả là router đòi hỏi URL path có dạng `/api/v1/ai/api/v1/ai/...`.
4. **Sử dụng Mux ảo (Mock)**: Các service khác (Scan, Finding) có thể chưa hoàn thiện hàm `WireEmbedded` mà đang phụ thuộc vào `cmd/server/embed.go` trả về `http.NewServeMux()` giả lập với duy nhất `/health`.

## Kế hoạch thực thi (Implementation Plan)

### Bước 1: Sửa lỗi Double Prefix của AI Service
- Mở file: `/services/ai-service/cmd/server/embed.go`
- Sửa cấu hình mount `aiRouter`:
  ```go
  // Thay vì:
  // mainMux.Handle("/api/v1/ai/", aiRouter)
  // Sửa thành (vì aiRouter đã định nghĩa Route /api/v1/ai ở bên trong):
  mainMux.Handle("/", aiRouter)
  ```

### Bước 2: Bổ sung Mount Route cho Search Service
- Mở file: `/services/search-service/embedded.go`
- Tìm đến khối `// 7. Mount router on mux...`
- Bổ sung thêm các route v1:
  ```go
  mux.Handle("/api/v1/search/", router)
  mux.Handle("/api/v1/search/recent", router)
  mux.Handle("/api/v1/search/suggested", router)
  ```

### Bước 3: Khởi tạo AlertsHandler cho Notification Service
- Mở file: `/services/notification-service/embedded.go`
- Kiểm tra xem thư mục `internal/delivery/http` có hàm `NewAlertsHandler` không. Khởi tạo `AlertsHandler` và truyền nó vào `SetupRouter` thay vì truyền `nil`. Nếu service chưa có AlertsHandler repository/usecase, hãy implement nó hoặc bổ sung graceful stub trả về 200/503.

### Bước 4: Kiểm tra và Refactor Scan, Finding, Asset Services
- Kiểm tra các file `cmd/server/embed.go` và `embedded.go` của các services này. Đảm bảo orchestrator (`apps/osv/internal/config/wire.go`) gọi hàm `WireEmbedded` nạp đầy đủ DB Pool và thực thi `NewRouter()` của service đó.
