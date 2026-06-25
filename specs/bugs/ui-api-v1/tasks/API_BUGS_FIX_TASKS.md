# Bảng Tác Vụ Thực Thi: Fix Client API Bugs (UI-API-V1)

Tài liệu này chia nhỏ các giải pháp từ `api_bug_solutions.md` thành các tác vụ (tasks) code chi tiết để AI (hoặc Developer) có thể bắt đầu quá trình thực thi và sửa lỗi trên từng service độc lập.

## 1. Identity Service & API Gateway
- [x] **[Task 1.1] Update Auth/Me DTO:** 
  - **File:** `services/identity-service/internal/adapter/http/dto.go` (hoặc tương đương). 
  - **Action:** Thêm trường `CreatedAt time.Time` với tag ``json:"created_at"`` vào struct `UserResponse` và đảm bảo mapper copy giá trị từ domain entity sang.
- [x] **[Task 1.2] Sửa Status Code cho Logout:** 
  - **File:** Handler xử lý `/auth/logout` (trong gateway hoặc identity-service).
  - **Action:** Sửa từ `200 OK` thành `204 No Content`. Xóa phần ghi body response.
- [x] **[Task 1.3] Bổ sung DTO cho RBAC:** 
  - **File:** Các DTO liên quan tới RBAC trong `identity-service`.
  - **Action:** Bổ sung `permission_categories` vào `RBACMatrixResponse`. Bổ sung `name`, `display_name`, `user_count`, `permissions` vào `Role` struct.
- [x] **[Task 1.4] Tính toán `user_count` cho Role:**
  - **File:** `infra/postgres/role_repo.go`.
  - **Action:** Sửa câu lệnh SQL của `GetRoles` để thực hiện `LEFT JOIN users` và `COUNT(users.id)`, trả về giá trị cho field `user_count`.
- [x] **[Task 1.5] Fix Status Filter (Admin Users):**
  - **File:** `infra/postgres/user_repo.go` (Hàm `ListUsers`).
  - **Action:** Thêm logic append điều kiện `WHERE status = $x` vào câu lệnh SQL nếu tham số query `status` không rỗng.

## 2. API Gateway (Routing) & BFF
- [x] **[Task 2.1] Bypass Auth cho Public Stats:**
  - **File:** `apps/osv/internal/gateway/auth/middleware.go` hoặc router config.
  - **Action:** Đưa đường dẫn `/api/v1/public/stats` (hoặc tương đương) vào danh sách các endpoint không yêu cầu xác thực.
- [x] **[Task 2.2] Fix lỗi 500 SLA Dashboard:**
  - **File:** `apps/osv/internal/gateway/bff/dashboard.go` (hoặc tương đương gọi SLA-service).
  - **Action:** Fix lỗi unmarshal. Cập nhật struct `SLADashboardResponse` map chính xác với cấu trúc JSON của SLA-service trả về.

## 3. Data Service & Search Service (Core APIs)
- [x] **[Task 3.1] Bắt lỗi Null/Pointer (CVE 500):**
  - **File:** `search-service/internal/adapter/opensearch/client.go` (hoặc nơi gọi OpenSearch query CVE).
  - **Action:** Thêm kiểm tra `if hit.Source == nil` hoặc `if len(hits) == 0` để tránh panic `runtime error: invalid memory address or nil pointer dereference`.
- [x] **[Task 3.2] Cập nhật Schema Semantic Search:**
  - **File:** `data-service/internal/delivery/http/...` (Handler của `/semantic`).
  - **Action:** Trả về thêm `total` (int) và `query_embedding_ms` (int/float) trong JSON response `SemanticSearchResponse`.
- [x] **[Task 3.3] Sửa Schema KEV:**
  - **File:** Handler xử lý KEV (`data-service/...`).
  - **Action:** 
    - Cập nhật struct `KEVEntry` bổ sung: `cve_id`, `vendor`, `vulnerability_name`, `date_added`, `due_date`, `known_ransomware_campaign_use`.
    - API List KEV cần trả về `data`, `page_size`, và `stats` thay vì mảng phẳng hoặc format hiện tại.
- [x] **[Task 3.4] Fix EPSS Schema:**
  - **File:** `data-service/...` (Handler EPSS).
  - **Action:** 
    - Sửa `EPSSTopResponse` để bao gồm `eps`, `percentile`, `date`.
    - Sửa `EPSSDistributionResponse` để trả về pointer cho `mean`/`median` (để cover trường hợp null).
- [x] **[Task 3.5] Fix CWE & Taxonomy Response:**
  - **File:** Handler CWE.
  - **Action:** Wrapper response vào list: thêm `CweList`, `Page`, `PageSize`. Đảm bảo list null `Mitigations` không làm vỡ map json.
- [x] **[Task 3.6] Fix Vendors Browse API:**
  - **File:** Handler vendors (`data-service/...`).
  - **Action:** Endpoint `/api/v2/vendors` map kết quả struct Elasticsearch sang mảng chuỗi `[]string` đơn giản thay vì mảng object.

## 4. Finding, Scan & SLA Service
- [x] **[Task 4.1] Đồng bộ Finding Status Enum:**
  - **File:** `finding-service/internal/domain/finding_state.go` và DTO của Finding.
  - **Action:** So sánh OpenAPI spec và struct, sửa cho khớp hoa/thường. Bổ sung trường `created_by`. Sửa query filter SQL để lấy đúng trạng thái `Active`.
- [x] **[Task 4.2] Fix Schema Scans List & Stats:**
  - **File:** DTO Scans trong `scan-service` và Handler.
  - **Action:** Thêm field `page_size` và `stats` vào List Response. Sửa logic handler `/scans/stats` để trả về đúng cấu trúc (gồm `active_scans`, `completed_today`, ...).
- [x] **[Task 4.3] Xử lý Nested SLA Config:**
  - **File:** DTO SLA Config trong `finding-service` hoặc `sla-service`.
  - **Action:** Khai báo cấu trúc JSON rõ ràng cho struct `Global` (`critical_days`, `high_days`,...) và `ProductOverrides`.

## 5. Notification & AI Services
- [x] **[Task 5.1] Thêm route GET Webhook Deliveries:**
  - **File:** HTTP Router của `notification-service`.
  - **Action:** Định nghĩa route GET `/webhook_deliveries` trỏ đến handler tương ứng (hiện tại chưa có nên trả về 405).
- [x] **[Task 5.2] Update Webhook Schema:**
  - **File:** DTO Webhook trong `notification-service`.
  - **Action:** Map đầy đủ các trường `id`, `name`, `url`, `events`, `secret`, `active`, `created_at`. `secret` có thể mã hóa thành `***`.
- [x] **[Task 5.3] Handle AI Failover (503):**
  - **File:** Các HTTP Handler trong `ai-service` (Triage & Enrichment).
  - **Action:** Khi upstream LLM bị down, trả về HTTP Status 202 Accepted (queued for processing) thay vì 503, hoặc implement fallback response.

---
> **Hướng dẫn cho AI/Dev:**
> Để thực hiện, hãy chọn 1 group tasks (Ví dụ: Group 1 Identity Service), sử dụng CodeGraph/grep để tìm file, đề xuất file thay đổi cụ thể, chỉnh sửa và tạo commit/PR tương ứng. Đánh dấu `[x]` khi tác vụ hoàn tất.
