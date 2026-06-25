# Báo cáo Bugs Tích hợp API (Client Tests)
**Ngày tạo:** 2026-06-19
**Nguồn dữ liệu:** `tests/client/report/api_test_report_20260619_174037.txt`

Dưới đây là danh sách các lỗi (bugs) được phân loại theo domain dựa trên kết quả chạy bộ test client API.

---

## 1. Authentication & Admin (`auth`, `admin_extended`, `admin_notifications`)
- **[AUTH-01] `/auth/me` Schema Incomplete:** Payload của user trả về bị thiếu trường `created_at`.
- **[AUTH-02] Logout Status Code Incorrect:** `POST /api/v1/auth/logout` trả về mã lỗi `200` thay vì chuẩn REST là `204 No Content`.
- **[ADMIN-01] RBAC Matrix Response Incomplete:** Thiếu trường `permission_categories` trong `RBACMatrixResponse`.
- **[ADMIN-02] RBAC Role Schema Invalid:** Các object role bị thiếu các trường `name`, `display_name`, `user_count`, và `permissions`.
- **[ADMIN-03] RBAC Roles Missing Admin:** Role `admin` bị thiếu trong danh sách trả về.
- **[ADMIN-04] Admin Users Status Filter Error:** Bộ lọc người dùng bị khóa (`status=locked`) không hoạt động, API vẫn trả về các users không bị khóa (VD: `0a9606c5-...`, `8a10a585-...`).
- **[ADMIN-05] Admin Health Services Incorrect Type:** Trường `health_services` trả về kiểu `list` thay vì `object` như định nghĩa.
- **[ADMIN-06] Admin Role 0 Schema:** Object phân quyền đầu tiên bị thiếu trường `name` hoặc `permissions`.

## 2. Dashboard & Public Stats (`dashboard`, `public_stats`)
- **[DASH-01] SLA Dashboard 500 Error:** Endpoint SLA Dashboard trả về `500 INTERNAL_ERROR` với thông báo `"Failed to parse SLA response"`.
- **[STAT-01] Public Stats Requires Auth:** Endpoint public stats trả về lỗi `401 Unauthorized` trong khi lẽ ra endpoint này không cần xác thực (public).

## 3. CVE & Vulnerability Intelligence (`cve`, `kev_epss`)
- **[CVE-01] CVE Search 500 Errors:** 
  - `GET /cves/search` lỗi `500: {"error":"search failed"}`.
  - Filter theo severity, KEV only, hoặc sort theo EPSS desc đều bị lỗi `500`.
- **[CVE-02] Semantic Search Schema Errors:** 
  - Trả về thiếu `total` và `query_embedding_ms` trong `SemanticSearchResponse`.
  - Xử lý nội bộ lỗi `Exception: 'NoneType' object is not subscriptable` do dữ liệu không đúng format.
- **[CVE-03] CVE Detail 500 Error:** `GET /cves/CVE-2021-44228` trả về `500: {"error":"internal error"}`.
- **[CVE-04] CVE Export 500 Error:** Chức năng export JSON trả về mã `500`.
- **[KEV-01] KEV Schema Incomplete:** Object KEV entry thiếu hàng loạt các trường bắt buộc: `cve_id`, `vendor`, `vulnerability_name`, `date_added`, `due_date`, và `known_ransomware_campaign_use`.
- **[KEV-02] KEV Endpoint Schema:** Root response thiếu `data`, `page_size`, và `stats`.
- **[KEV-03] KEV Ransomware Filter:** Filter ransomware trả về lỗi xử lý do schema trả về là `NoneType` thay vì list.
- **[EPSS-01] EPSS Top Schema:** Thiếu `cves` và `total` trong `EPSSTopResponse`.
- **[EPSS-02] EPSS Distribution Schema:** Thiếu `buckets`, `total_cves`, `mean_epss`, và `median_epss`. Trường `mean_epss` và `median_epss` bị trả về `NoneType`.

## 4. Taxonomy (`taxonomy`)
- **[TAX-01] CWE List Pagination & Schema:** Endpoint list CWE bị thiếu mảng `cwe_list` và các tham số phân trang (`page`, `page_size`). Lỗi này cũng xảy ra trên endpoint search CWE.
- **[TAX-02] CWE Detail Schema:** Chi tiết CWE thiếu các trường `likelihood`, `mitigations`, `capec_patterns`, và `related_cve_count`. `mitigations` bị sai type (`NoneType` thay vì list).
- **[TAX-03] Vendors Endpoint Invalid Response:** API trả về danh sách objects (ví dụ: `{'vendor': '...', 'product_count': 1}`) thay vì mảng các chuỗi (array of strings) theo chuẩn OpenAPI spec.
- **[TAX-04] Vendors Prefix Search Error:** Search API (prefix) bị lỗi nội bộ (`Exception: 'dict' object has no attribute 'lower'`) do nguyên nhân từ TAX-03.

## 5. Findings, Scans & Assets (`findings_scans`, `scan_stats`, `assets_products`)
- **[FND-01] Findings Item Schema:** Object Finding thiếu `created_by`. Trạng thái trả về `'Active'` không thuộc tập valid enum.
- **[FND-02] Findings Status Filter Failing:** Filter bằng `status=Active` trả về cả những finding không phải active.
- **[FND-03] Findings Stats 400 Error:** Endpoint statistics của finding trả về mã `400 Bad Request`.
- **[SCN-01] Scans List Schema:** Thiếu field `page_size` và object `stats` trong danh sách trả về.
- **[SCN-02] Scans Stats Schema:** API `/scans/stats` trả về thiếu các trường `active_scans`, `completed_today`, `total_findings_today`, và `scheduled_scans`.
- **[SLA-01] SLA Config Schema Incomplete:** Object SLA Config thiếu `global` và `product_overrides`.
- **[SLA-02] SLA Config Global Schema:** Thiếu `critical_days`, `high_days`, `medium_days`, và `low_days`. Object `product_overrides` không phải là mảng.
- **[SLA-03] SLA Overview Schema:** Thiếu các field `total_active_findings`, `compliance_percent`, và `ok`.
- **[AST-01] Assets List Invalid Type:** Response của asset list không phải là kiểu mảng.
- **[PRD-01] Products List Schema:** Thiếu mảng `products` và field `total`. Field `types` không phải là mảng string.

## 6. AI Reports & Webhooks (`ai_reports`, `ai_triage_queue`, `webhook_deliveries`)
- **[AI-01] AI Triage Queue 503 Errors:** Tất cả các truy vấn vào hàng đợi AI Triage (list, status, severity filter, pending filter) đều gặp lỗi `503 SERVICE_UNAVAILABLE` ("Upstream service unavailable").
- **[AI-02] AI Enrichment 503 Errors:** Các API về AI enrichment (bao gồm endpoint chi tiết của CVE) đều lỗi `503`.
- **[WHK-01] Webhook Schema Incomplete:** Object webhook thiếu toàn bộ các trường cốt lõi như `id`, `name`, `url`, `events`, `secret`, `active`, và `created_at`.
- **[WHK-02] Webhook Deliveries 405 Errors:** Endpoint về `webhook_deliveries` trả về HTTP `405 Method Not Allowed`, cho thấy phương thức GET hiện chưa được implement trên route này hoặc backend định nghĩa sai HTTP Method.
