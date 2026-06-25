# TASK-001: Khắc phục lỗi Embedded Router cho Search & AI Service

## Mục tiêu
Fix lỗi 404 cho các endpoint của AI Service và Search Service bằng cách sửa lỗi cấu hình mount path trong các file nhúng.

## Các Endpoints Bị Ảnh Hưởng
- `GET /api/v1/search/recent` (Search Service)
- `GET /api/v1/search/suggested` (Search Service)
- Các endpoint thuộc `GET /api/v1/ai/*` (AI Service)

## Hướng dẫn thực thi

1. **AI Service:**
   - Mở file `services/ai-service/cmd/server/embed.go`
   - Tìm đoạn mount: `mainMux.Handle("/api/v1/ai/", aiRouter)`
   - Sửa thành: `mainMux.Handle("/", aiRouter)` (vì `aiRouter` đã định nghĩa sẵn tiền tố `/api/v1/ai` bên trong chi.Router).

2. **Search Service:**
   - Mở file `services/search-service/embedded.go`
   - Tìm khối `// 7. Mount router on mux...`
   - Thêm dòng:
     ```go
     mux.Handle("/api/v1/search/", router)
     mux.Handle("/api/v1/search/recent", router)
     mux.Handle("/api/v1/search/suggested", router)
     ```
   - Chắc chắn rằng hàm `NewRouter` trong `search-service/internal/delivery/http/search_handler.go` cũng không bị đụng độ Double-Prefix (kiểm tra lại).

## Acceptance Criteria (AC)
- [x] Code compile thành công.
- [x] Khi chạy `docker-compose.server.yml`, các endpoint trên không còn trả về 404.
