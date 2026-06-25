# Giải pháp Khắc phục Bug API Client (UI-API-V1)
**Ngày tạo:** 2026-06-19
**Tham chiếu:** `01-architecture.md`, `02-technical-design.md`, `api_client_bugs_20260619.md`

Dựa trên thiết kế kiến trúc (Clean Architecture) và Technical Design của hệ thống, dưới đây là các giải pháp đề xuất để khắc phục các lỗi API đã được báo cáo.

---

## 1. Authentication & Identity Service
**[AUTH-01] Thiếu trường `created_at` trong `/auth/me`**
- **Giải pháp:** Cập nhật struct DTO `UserResponse` tại `services/identity-service/internal/adapter/http/dto.go`. Đảm bảo trường `CreatedAt time.Time` được map từ domain entity sang DTO và có JSON struct tag ``json:"created_at"``.

**[AUTH-02] Logout trả về 200 thay vì 204**
- **Giải pháp:** Trong HTTP Handler của API Gateway hoặc Auth-Service (nơi xử lý route `/auth/logout`), thay đổi lệnh `w.WriteHeader(http.StatusOK)` thành `w.WriteHeader(http.StatusNoContent)`. Đảm bảo không ghi body nào sau khi set header 204.

**[ADMIN-01 đến ADMIN-06] Lỗi Schema RBAC & Bộ lọc Users**
- **Giải pháp:** 
  - Tại `services/identity-service/internal/usecase/rbac_usecase.go`, đảm bảo `RBACMatrixResponse` được khởi tạo đầy đủ mảng `permission_categories`. 
  - Cập nhật struct `Role` để bao gồm JSON tags cho `name`, `display_name`, `user_count`, và `permissions`.
  - Thực hiện query `COUNT(user_id)` và `GROUP BY role` ở `infra/postgres/role_repo.go` để lấy đúng số lượng `user_count`.
  - Fix hàm repository filter users (`ListUsers`), thêm điều kiện `WHERE status = 'locked'` nếu có query param truyền vào, tránh lấy toàn bộ.

---

## 2. API Gateway & Dashboard BFF
**[STAT-01] Public Stats bị chặn bởi 401 Unauthorized**
- **Giải pháp:** Sửa file `apps/osv/internal/gateway/auth/middleware.go`. Bổ sung route `/api/v1/public/stats` vào danh sách `publicRoutes` hoặc `bypassPaths` để bỏ qua việc kiểm tra JWT/API Key (trước khi gọi `extractBearer`).

**[DASH-01] SLA Dashboard báo 500 INTERNAL_ERROR**
- **Giải pháp:** Lỗi này xảy ra ở BFF (Gateway) khi gọi sang SLA-Service. Schema trả về từ SLA-service đang khác với struct định nghĩa ở BFF. Cần đồng bộ struct `SLADashboardResponse` ở cả 2 service hoặc sửa logic parse JSON (handle missing fields/null values bằng con trỏ).

---

## 3. Data-Service & Search-Service (CVE / KEV / EPSS)
**[CVE-01, CVE-03] Lỗi 500 khi Search và xem chi tiết CVE**
- **Giải pháp:** Lỗi 500 thường xuất phát từ nil pointer dereference khi OpenSearch trả về dữ liệu thiếu hoặc kết nối lỗi. Tại `services/search-service/internal/adapter/opensearch/client.go` và Data-service, cần kiểm tra kỹ lỗi trả về (`err != nil`). Bổ sung `zerolog` để bắt stack trace, xử lý fallback nếu query bị fail (ví dụ trả về list rỗng thay vì 500).

**[CVE-02] Semantic Search Schema Error**
- **Giải pháp:** Cập nhật DTO `SemanticSearchResponse` thêm ``json:"total"`` và ``json:"query_embedding_ms"``. Lỗi `NoneType is not subscriptable` cho thấy AI-service/Embedding trả về rỗng, Data-service cần handle case này và trả về lỗi 400 thay vì panic 500.

**[KEV-01 đến KEV-03] KEV Schema thiếu dữ liệu & Lỗi Filter**
- **Giải pháp:** Mở rộng struct `KEVEntry` ở `services/data-service/internal/domain/kev.go` để mapping đầy đủ các cột từ Postgres (như `vendor`, `date_added`, `known_ransomware_campaign_use`). Tại HTTP Adapter, wrap kết quả vào object `{ "data": [...], "page_size": 20, "stats": {...} }`. Nếu danh sách rỗng, trả về `[]KEVEntry{}` thay vì `nil` để tránh lỗi iterable NoneType.

**[EPSS-01, EPSS-02] Lỗi Schema EPSS**
- **Giải pháp:** Đồng bộ lại struct `EPSSTopResponse` và `EPSSDistributionResponse` trong adapter HTTP của Data-service. Dùng `float64` pointers (`*float64`) cho `mean_epss` và `median_epss` để có thể trả lời `null` khi chưa có data, hoặc query database để tính trung bình nếu đang bị rỗng.

---

## 4. Taxonomy (Data-Service)
**[TAX-01] Lỗi Pagination CWE**
- **Giải pháp:** Response trả về mảng trực tiếp thay vì bọc trong object. Sửa logic handler `ListCWEs` thành: `return c.JSON(http.StatusOK, CWEListResponse{ CweList: results, Page: req.Page, PageSize: req.PageSize })`.

**[TAX-02] Thiếu thông tin chi tiết CWE**
- **Giải pháp:** Các trường `Likelihood`, `Mitigations`, `CapecPatterns` có thể là mảng rỗng (JSON `[]`) chứ không được để `null`. Khởi tạo `[]string{}` nếu dữ liệu trong DB là nil.

**[TAX-03, TAX-04] API Vendors trả về sai kiểu**
- **Giải pháp:** OpenAPI yêu cầu mảng string `["apache", "microsoft"]`. Hiện tại `infra/postgres` đang trả về mảng object `[{vendor: "apache", product_count: 1}]`. Cần thêm một vòng lặp map kết quả trong `usecase` để trích xuất mảng string trước khi return về HTTP adapter.

---

## 5. Finding & Scan Services
**[FND-01, FND-02] Trạng thái và thiếu field Findings**
- **Giải pháp:** Cập nhật `FindingState` trong `services/finding-service/internal/domain/finding_state.go` để match 100% với Enum của OpenAPI. Đảm bảo query SQL trong `ListByProduct` sử dụng điều kiện `WHERE status = $1` thay vì filter bằng RAM.

**[SCN-01, SCN-02] Lỗi Schema List & Stats của Scans**
- **Giải pháp:** Bổ sung việc gọi hàm `repo.GetScanStats(ctx)` trong Usecase và gộp kết quả vào response object của `ScansListResponse` (trường `stats`) tại HTTP adapter. Đảm bảo struct đầy đủ `active_scans`, `completed_today`.

**[SLA-01 đến SLA-03] SLA Config & Overview Schema**
- **Giải pháp:** Kiểm tra việc unmarshal JSONB từ cột Postgres ra `SLAConfig` struct trong `finding-service` hoặc `sla-service`. Các nested struct (`global`, `product_overrides`) cần được định nghĩa tường minh với JSON tags. 

---

## 6. AI Reports & Notification Services
**[AI-01, AI-02] Lỗi 503 cho AI Triage và Enrichment**
- **Giải pháp:** Service AI phụ thuộc vào LLM provider (OpenAI/Ollama). Nếu LLM gateway timeout hoặc down, API trả về 503. Cần triển khai failover hoặc trả về HTTP 202 Accepted (đưa vào Queue xử lý bất đồng bộ) theo đúng thiết kế event-driven thay vì block HTTP request chờ kết quả.

**[WHK-01, WHK-02] Lỗi Webhook Schema và 405**
- **Giải pháp:** 
  - Tại `services/notification-service`, cập nhật DTO của Webhook để expose đầy đủ ID, Name, URL. (Chú ý không trả về `Secret` dạng raw mà chỉ che giấu `***` hoặc bỏ qua tùy theo OpenAPI specs).
  - Lỗi 405 (Method Not Allowed) cho GET `/webhook_deliveries` nghĩa là handler cho GET chưa được register trên router của HTTP adapter. Cần bổ sung route `r.Get("/webhook_deliveries", h.ListDeliveries)`.
