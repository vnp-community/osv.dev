# Backend API Specifications

> **Cập nhật:** 2026-06-24 — Thêm endpoints: accept-invite, platform settings, jira-issues mapping, scheduled scans, batch enrich, search history  
> Tài liệu này mô tả tất cả REST API endpoints được cung cấp bởi backend services.  
> Nguồn: source code thực tế từ các router/handler files.  
> **Chú ý về Auth:** Tất cả protected routes đều yêu cầu header `Authorization: Bearer <JWT>` hoặc `X-API-Key` được gateway kiểm tra. Headers `X-User-ID`, `X-User-Role`, `X-User-Permissions` được *inject bởi gateway* vào upstream services — không cần client gửi.

---

## Mục lục

1. [apps/osv — API Gateway (`:8080`)](#appsosv--api-gateway-port-8080)
2. [services/identity-service (`:8081`)](#servicesidentity-service-port-8081)
3. [services/data-service (`:8082`)](#servicesdata-service-port-8082)
4. [services/finding-service (`:8085`)](#servicesfinding-service-port-8085)
5. [services/sla-service (`:8086`)](#servicessla-service-port-8086)
6. [services/notification-service (`:8087`)](#servicesnotification-service-port-8087)
7. [services/jira-service (`:8088`)](#servicesjira-service-port-8088)
8. [services/audit-service (`:8090`)](#servicesaudit-service-port-8090)
9. [services/scan-service (`:8084`)](#servicesscan-service-port-8084)
10. [services/search-service (`:8083`)](#servicessearch-service-port-8083)
11. [services/ranking-service](#servicesranking-service)
12. [services/ai-service (`:9103`)](#servicesai-service-port-9103)
13. [services/asset-service (`:8091`)](#servicesasset-service-port-8091)
14. [services/product-service](#servicesproduct-service)
15. [services/report-service](#servicesreport-service)

---

## apps/osv — API Gateway (port :8080)

Gateway là entry-point duy nhất từ client. Nó xác thực JWT/API-Key rồi forward requests đến upstream services.

**Source:** `apps/osv/internal/gateway/router.go`

### Public Endpoints (không cần auth)

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/health` | — (in-gateway) | Health check |
| `GET` | `/readyz` | — (in-gateway) | Readiness check |
| `GET` | `/api/v2/schema` | — (in-gateway) | OpenAPI schema |
| `GET` | `/api/v2/scan-types` | scan-service:8084 | Danh sách loại scan |
| `POST` | `/api/v1/auth/login` | identity-service:8081 | Đăng nhập |
| `POST` | `/api/v1/auth/refresh` | identity-service:8081 | Refresh JWT token |
| `GET` | `/api/v1/auth/accept-invite` | identity-service:8081 | Activate user account qua invitation token — TASK-HC-014 |
| `GET` | `/api/v1/kev` | data-service:8082 | Danh sách KEV |
| `GET` | `/api/v1/kev/sync/status` | data-service:8082 | KEV sync status |
| `GET` | `/api/v1/kev/check` | data-service:8082 | Kiểm tra CVE có trong KEV |
| `GET` | `/api/v1/kev/stats` | data-service:8082 | KEV stats |
| `GET` | `/api/v1/kev/ransomware` | data-service:8082 | Ransomware KEV list |
| `GET` | `/api/v1/kev/{cveId}` | data-service:8082 | Chi tiết KEV entry |
| `GET` | `/api/v1/epss/top` | data-service:8082 | Top EPSS scores |
| `GET` | `/api/v1/epss/distribution` | data-service:8082 | Phân phối EPSS |
| `GET` | `/api/v1/cve/last/{n}` | data-service:8082 | N CVEs mới nhất |
| `GET` | `/api/v1/cve/recent/{timeframe}` | data-service:8082 | CVEs gần đây |
| `GET` | `/api/v1/cve/search` | data-service:8082 | Tìm kiếm CVE |
| `GET` | `/api/v1/cve/{id}` | data-service:8082 | Chi tiết CVE |
| `GET` | `/api/v1/dbinfo` | data-service:8082 | Database info |
| `GET` | `/v1/` | data-service:8082 | Legacy OSV v1 API |
| `GET` | `/api/v2/public/stats` | in-gateway BFF (PublicBFF) | Public platform statistics (rate-limited 60/min, cached 5min) — CR-013 |
| `GET` | `/api/v1/public/stats` | in-gateway BFF (PublicBFF) | Public platform statistics (alias v1) — CR-013 |

### Protected Endpoints — Auth

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v1/auth/me` | identity-service:8081 | Thông tin user hiện tại |
| `POST` | `/api/v1/auth/logout` | identity-service:8081 | Đăng xuất |
| `POST` | `/api/v1/auth/totp/setup` | identity-service:8081 | Cài đặt TOTP |
| `POST` | `/api/v1/auth/totp/verify` | identity-service:8081 | Xác minh TOTP |
| `DELETE` | `/api/v1/auth/totp` | identity-service:8081 | Vô hiệu hóa TOTP |
| `GET` | `/api/v1/auth/mfa/setup` | identity-service:8081 (rewrite → /totp/setup) | MFA setup alias — CR-001 |
| `POST` | `/api/v1/auth/mfa/confirm` | identity-service:8081 (rewrite → /totp/verify) | MFA confirm alias — CR-001 |
| `GET` | `/api/v1/profile` | identity-service:8081 | Lấy hồ sơ |
| `PATCH` | `/api/v1/profile` | identity-service:8081 | Cập nhật hồ sơ |
| `POST` | `/api/v1/profile/change-password` | identity-service:8081 | Đổi mật khẩu |
| `GET` | `/api/v1/api-keys` | identity-service:8081 (rewrite → /auth/api-keys) | Danh sách API keys |
| `POST` | `/api/v1/api-keys` | identity-service:8081 (rewrite → /auth/api-keys) | Tạo API key |
| `DELETE` | `/api/v1/api-keys/{id}` | identity-service:8081 (rewrite → /auth/api-keys/{id}) | Xóa API key |

### Protected Endpoints — Admin (role: Admin)

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v1/admin/health` | in-gateway BFF (HealthBFF) | Tổng quan health hệ thống |
| `GET` | `/api/v1/admin/settings` | identity-service:8081 → `platform_settings` DB | Đọc platform settings — TASK-HC-009 |
| `PUT` | `/api/v1/admin/settings` | identity-service:8081 → `platform_settings` DB | Cập nhật toàn bộ platform settings — TASK-HC-009 |
| `PATCH` | `/api/v1/admin/settings` | identity-service:8081 → `platform_settings` DB | Partial update platform settings — TASK-HC-009 |
| `GET` | `/api/v1/admin/users` | identity-service:8081 | Danh sách users |
| `POST` | `/api/v1/admin/users` | identity-service:8081 | Tạo user mới (rate-limit 20/min) |
| `POST` | `/api/v1/admin/users/bulk` | identity-service:8081 | Bulk create users (rate-limit 5/min) |
| `POST` | `/api/v1/admin/users/invite` | identity-service:8081 | Mời user mới |
| `GET` | `/api/v1/admin/users/{id}` | identity-service:8081 | Chi tiết user — CR-001 |
| `PATCH` | `/api/v1/admin/users/{id}` | identity-service:8081 | Cập nhật user |
| `POST` | `/api/v1/admin/users/{id}/unlock` | identity-service:8081 | Mở khóa tài khoản |
| `POST` | `/api/v1/admin/users/{id}/reset-password` | identity-service:8081 | Reset mật khẩu |
| `POST` | `/api/v1/admin/users/{id}/api-keys` | identity-service:8081 | Tạo API key cho user |
| `POST` | `/api/v1/admin/users/{id}/roles` | identity-service:8081 | Gán role cho user — SEED-001-D |
| `GET` | `/api/v1/admin/roles` | identity-service:8081 → `rbac_roles` DB | RBAC matrix từ DB — TASK-HC-010 |
| `GET` | `/api/v1/audit-log` | audit-service:8090 | Audit log (v1) |
| `GET` | `/api/v2/jira-configurations/{product_id}` | jira-service:8088 | Cấu hình Jira |
| `POST` | `/api/v2/jira-configurations` | jira-service:8088 | Tạo/cập nhật Jira config |
| `DELETE` | `/api/v2/jira-configurations/{product_id}` | jira-service:8088 | Xóa Jira config |
| `POST` | `/api/v2/jira-configurations/test` | jira-service:8088 | Test Jira kết nối |
| `GET` | `/api/v2/jira-issues` | jira-service:8088 | Liệt kê JIRA issue mappings — TASK-HC-013 |
| `POST` | `/api/v2/jira-issues` | jira-service:8088 | Tạo JIRA issue mapping (finding ↔ JIRA key) — TASK-HC-013 |
| `GET` | `/api/v2/jira-issues/{finding_id}` | jira-service:8088 | Lấy JIRA issue của finding — TASK-HC-013 |
| `DELETE` | `/api/v2/jira-issues/{id}` | jira-service:8088 | Xóa JIRA issue mapping — TASK-HC-013 |

### Protected Endpoints — Dashboard

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v1/dashboard` | in-gateway BFF (DashboardBFF) | Dashboard tổng quan |
| `GET` | `/api/v1/dashboard/sla` | in-gateway BFF (DashboardBFF) | Dashboard SLA |

### Protected Endpoints — Findings (v1)

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v1/findings` | finding-service:8085 | Danh sách findings |
| `GET` | `/api/v1/findings/stats` | finding-service:8085 | Thống kê findings |
| `GET` | `/api/v1/findings/{id}` | finding-service:8085 | Chi tiết finding |
| `PATCH` | `/api/v1/findings/{id}` | finding-service:8085 | Cập nhật finding |
| `POST` | `/api/v1/findings/bulk/reopen` | finding-service:8085 | Bulk reopen |
| `POST` | `/api/v1/findings/bulk/assign` | finding-service:8085 | Bulk assign |
| `POST` | `/api/v1/findings/bulk/close` | finding-service:8085 | Bulk close |
| `GET` | `/api/v1/findings/{id}/notes` | finding-service:8085 | Ghi chú của finding |
| `POST` | `/api/v1/findings/{id}/notes` | finding-service:8085 | Thêm ghi chú |
| `GET` | `/api/v1/findings/{id}/audit` | finding-service:8085 | Audit trail của finding |

### Protected Endpoints — Findings (v2)

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v2/findings` | finding-service:8085 | Danh sách findings (v2, user-scoped) |
| `POST` | `/api/v2/findings` | finding-service:8085 | Tạo finding |
| `GET` | `/api/v2/findings/severity_count` | finding-service:8085 | Đếm theo severity |
| `POST` | `/api/v2/findings/bulk` | finding-service:8085 | Bulk update (rate-limit 10/min) |
| `POST` | `/api/v2/findings/bulk-create` | finding-service:8085 | Bulk create (rate-limit 10/min) — SEED-003 |
| `POST` | `/api/v2/findings/import` | finding-service:8085 | Import findings từ file (rate-limit 5/min, max 500MB) — SEED-003 |
| `GET` | `/api/v2/findings/{id}` | finding-service:8085 | Chi tiết finding |
| `PUT` | `/api/v2/findings/{id}` | finding-service:8085 | Cập nhật finding |
| `PATCH` | `/api/v2/findings/{id}` | finding-service:8085 | Partial update |
| `DELETE` | `/api/v2/findings/{id}` | finding-service:8085 | Xóa finding |
| `POST` | `/api/v2/findings/{id}/close` | finding-service:8085 | Đóng finding |
| `POST` | `/api/v2/findings/{id}/reopen` | finding-service:8085 | Mở lại finding |
| `POST` | `/api/v2/findings/{id}/accept-risk` | finding-service:8085 | Chấp nhận rủi ro |
| `POST` | `/api/v2/findings/{id}/false-positive` | finding-service:8085 | Đánh dấu false positive |
| `POST` | `/api/v2/findings/{id}/out-of-scope` | finding-service:8085 | Đánh dấu ngoài phạm vi |
| `GET` | `/api/v2/findings/{id}/duplicates` | finding-service:8085 | Findings trùng lặp |
| `GET` | `/api/v2/findings/{id}/notes` | finding-service:8085 | Ghi chú (v2) |
| `POST` | `/api/v2/findings/{id}/notes` | finding-service:8085 | Thêm ghi chú (v2) |
| `GET` | `/api/v2/finding-groups` | finding-service:8085 | Nhóm findings |
| `POST` | `/api/v2/finding-groups` | finding-service:8085 | Tạo nhóm |

### Protected Endpoints — Products

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v1/products` | finding-service:8085 | Danh sách products (v1) |
| `POST` | `/api/v1/products` | finding-service:8085 | Tạo product |
| `GET` | `/api/v1/products/grades` | finding-service:8085 | Product grades |
| `GET` | `/api/v1/products/types` | finding-service:8085 | Product types |
| `GET` | `/api/v1/products/{id}` | finding-service:8085 | Chi tiết product |
| `PUT` | `/api/v1/products/{id}` | finding-service:8085 | Cập nhật product |
| `PATCH` | `/api/v1/products/{id}` | finding-service:8085 | Partial update product — CR-003 |
| `DELETE` | `/api/v1/products/{id}` | finding-service:8085 | Xóa product |
| `GET` | `/api/v1/products/{id}/engagements` | finding-service:8085 | Engagements của product |
| `POST` | `/api/v1/products/{id}/engagements` | finding-service:8085 | Tạo engagement cho product — CR-003 |
| `GET` | `/api/v2/products` | finding-service:8085 | Danh sách products (v2, user-scoped) |
| `POST` | `/api/v2/products` | finding-service:8085 | Tạo product (v2, rate-limit 10/min) |
| `POST` | `/api/v2/products/bulk` | finding-service:8085 | Bulk create products — SEED-002 |
| `POST` | `/api/v2/products/import` | finding-service:8085 | Import products (Admin, rate-limit 2/min, max 500MB) — SEED-002 |
| `GET` | `/api/v2/products/{id}` | finding-service:8085 | Chi tiết product (v2) |
| `PUT` | `/api/v2/products/{id}` | finding-service:8085 | Cập nhật product (v2) |
| `DELETE` | `/api/v2/products/{id}` | finding-service:8085 | Xóa product (v2) |
| `POST` | `/api/v2/products/{id}/seed` | finding-service:8085 | Seed product data — SEED-002 |
| `GET` | `/api/v2/products/{id}/members` | finding-service:8085 | Members của product |
| `POST` | `/api/v2/products/{id}/members` | finding-service:8085 | Thêm member |
| `DELETE` | `/api/v2/products/{id}/members/{uid}` | finding-service:8085 | Xóa member |
| `GET` | `/api/v2/product-types` | finding-service:8085 | Danh sách loại product |
| `POST` | `/api/v2/product-types` | finding-service:8085 | Tạo loại product (rate-limit 10/min) |
| `POST` | `/api/v2/product-types/bulk` | finding-service:8085 | Bulk create product types — SEED-002 |
| `GET` | `/api/v2/product-types/{id}` | finding-service:8085 | Chi tiết loại product |
| `PUT` | `/api/v2/product-types/{id}` | finding-service:8085 | Cập nhật loại product |
| `DELETE` | `/api/v2/product-types/{id}` | finding-service:8085 | Xóa loại product |
| `GET` | `/api/v2/product-grades` | finding-service:8085 | Product grades (v2) |
| `GET` | `/api/v2/product-grades/{id}` | finding-service:8085 | Grade của product |

### Protected Endpoints — Engagements & Tests

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v1/engagements` | finding-service:8085 | Danh sách engagements |
| `POST` | `/api/v1/engagements` | finding-service:8085 | Tạo engagement |
| `GET` | `/api/v1/engagements/{id}` | finding-service:8085 | Chi tiết engagement |
| `POST` | `/api/v1/engagements/{id}/close` | finding-service:8085 | Đóng engagement |
| `POST` | `/api/v1/engagements/{id}/reopen` | finding-service:8085 | Mở lại engagement |
| `GET` | `/api/v1/engagements/{id}/tests` | finding-service:8085 | Tests của engagement |
| `GET` | `/api/v2/engagements` | finding-service:8085 | Danh sách engagements (v2) |
| `POST` | `/api/v2/engagements` | finding-service:8085 | Tạo engagement (v2) |
| `GET` | `/api/v2/engagements/{id}` | finding-service:8085 | Chi tiết engagement (v2) |
| `PUT` | `/api/v2/engagements/{id}` | finding-service:8085 | Cập nhật engagement |
| `POST` | `/api/v2/engagements/{id}/close` | finding-service:8085 | Đóng engagement (v2) |
| `POST` | `/api/v2/engagements/{id}/reopen` | finding-service:8085 | Mở lại engagement (v2) |
| `GET` | `/api/v2/tests` | finding-service:8085 | Danh sách tests |
| `POST` | `/api/v2/tests` | finding-service:8085 | Tạo test |
| `GET` | `/api/v2/tests/{id}` | finding-service:8085 | Chi tiết test |
| `PUT` | `/api/v2/tests/{id}` | finding-service:8085 | Cập nhật test |
| `DELETE` | `/api/v2/tests/{id}` | finding-service:8085 | Xóa test |

### Protected Endpoints — Risk Acceptances

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v1/risk-acceptances` | finding-service:8085 | Danh sách (v1) |
| `POST` | `/api/v1/risk-acceptances` | finding-service:8085 | Tạo (v1) |
| `DELETE` | `/api/v1/risk-acceptances/{id}` | finding-service:8085 | Xóa (v1) |
| `GET` | `/api/v2/risk-acceptances` | finding-service:8085 | Danh sách (v2) |
| `POST` | `/api/v2/risk-acceptances` | finding-service:8085 | Tạo (v2) |
| `GET` | `/api/v2/risk-acceptances/{id}` | finding-service:8085 | Chi tiết |
| `PUT` | `/api/v2/risk-acceptances/{id}` | finding-service:8085 | Cập nhật |
| `DELETE` | `/api/v2/risk-acceptances/{id}` | finding-service:8085 | Xóa |
| `POST` | `/api/v2/risk-acceptances/{id}/findings/{fid}/remove` | finding-service:8085 | Gỡ finding khỏi risk acceptance |

### Protected Endpoints — Reports

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v1/reports/templates` | finding-service:8085 | Report templates (v1) — CR-010 |
| `GET` | `/api/v1/reports` | finding-service:8085 | Danh sách reports (v1) |
| `POST` | `/api/v1/reports` | finding-service:8085 | Tạo report (v1, rate-limit 5/min) |
| `GET` | `/api/v1/reports/{id}` | finding-service:8085 | Chi tiết report (v1) |
| `GET` | `/api/v1/reports/{id}/download` | finding-service:8085 | Tải report (v1, timeout 30s) |
| `DELETE` | `/api/v1/reports/{id}` | finding-service:8085 | Xóa report (v1) |
| `GET` | `/api/v2/reports/templates` | finding-service:8085 | Report templates (v2) — CR-010 |
| `GET` | `/api/v2/reports` | finding-service:8085 | Danh sách reports (v2, user-scoped) |
| `POST` | `/api/v2/reports` | finding-service:8085 | Tạo report (v2, rate-limit 5/min) |
| `GET` | `/api/v2/reports/{id}` | finding-service:8085 | Chi tiết report |
| `GET` | `/api/v2/reports/{id}/download` | finding-service:8085 | Tải report (timeout 30s) |
| `DELETE` | `/api/v2/reports/{id}` | finding-service:8085 | Xóa report |

### Protected Endpoints — Scans

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v1/scans` | scan-service:8084 | Danh sách scans |
| `POST` | `/api/v1/scans` | scan-service:8084 | Tạo scan |
| `GET` | `/api/v1/scans/stats/weekly` | scan-service:8084 | Thống kê scan theo tuần — CR-008 |
| `GET` | `/api/v1/scans/stats` | scan-service:8084 | Thống kê scan tổng — CR-008 |
| `GET` | `/api/v1/scans/scheduled` | scan-service:8084 | Danh sách scheduled scans |
| `POST` | `/api/v1/scans/scheduled` | scan-service:8084 | Tạo scheduled scan — SEED-005-C |
| `POST` | `/api/v1/scans/import` | scan-service:8084 | Import scan results (rate-limit 10/min, max 500MB) |
| `GET` | `/api/v1/scans/{id}` | scan-service:8084 | Chi tiết scan |
| `POST` | `/api/v1/scans/{id}/cancel` | scan-service:8084 | Hủy scan |
| `GET` | `/api/v1/scans/{id}/stream` | scan-service:8084 | Stream kết quả scan (SSE, token auth) — CR-007 |
| `GET` | `/api/v1/scans/{id}/results/nmap` | scan-service:8084 | Kết quả Nmap |
| `GET` | `/api/v1/scans/{id}/results/zap` | scan-service:8084 | Kết quả ZAP |
| `GET` | `/api/v1/scans/scheduled/{id}` | scan-service:8084 | Chi tiết scheduled scan — SEED-005-C |
| `PUT` | `/api/v1/scans/scheduled/{id}` | scan-service:8084 | Cập nhật scheduled scan — SEED-005-C |
| `DELETE` | `/api/v1/scans/scheduled/{id}` | scan-service:8084 | Xóa scheduled scan — SEED-005-C |
| `POST` | `/api/v2/import-scan` | scan-service:8084 | Import scan (v2, rate-limit 30/min, max 500MB) |
| `POST` | `/api/v2/reimport-scan` | scan-service:8084 | Reimport scan (rate-limit 30/min, max 500MB) |
| `GET` | `/api/v2/parsers` | scan-service:8084 | Danh sách parsers |
| `GET` | `/api/v2/test-imports` | scan-service:8084 | Test imports |
| `GET` | `/api/v2/test-imports/{id}` | scan-service:8084 | Chi tiết test import |

### Protected Endpoints — Agents (SEED-005-C)

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `POST` | `/api/v1/agents` | scan-service:8084 | Đăng ký agent (Admin only) |
| `GET` | `/api/v1/agents` | scan-service:8084 | Danh sách agents |
| `GET` | `/api/v1/agents/{id}` | scan-service:8084 | Chi tiết agent |
| `POST` | `/api/v1/agents/{id}/reports` | scan-service:8084 | Submit báo cáo từ agent |

### Protected Endpoints — Assets (CR-004)

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v1/assets/tags` | asset-service:8091 | Danh sách tags |
| `POST` | `/api/v1/assets/bulk` | asset-service:8091 | Bulk create assets (rate-limit 5/min) — SEED-005-B |
| `POST` | `/api/v1/assets/import` | asset-service:8091 | Import assets từ file (rate-limit 2/min, max 500MB) — SEED-005-B |
| `GET` | `/api/v1/assets` | asset-service:8091 | Danh sách assets |
| `POST` | `/api/v1/assets` | asset-service:8091 | Tạo asset |
| `GET` | `/api/v1/assets/{id}` | asset-service:8091 | Chi tiết asset |
| `PUT` | `/api/v1/assets/{id}` | asset-service:8091 | Cập nhật asset |
| `DELETE` | `/api/v1/assets/{id}` | asset-service:8091 | Xóa asset |
| `PUT` | `/api/v1/assets/{id}/tags` | asset-service:8091 | Cập nhật tags |
| `GET` | `/api/v1/assets/{id}/risk` | asset-service:8091 | Risk score |
| `GET` | `/api/v1/assets/{id}/history` | asset-service:8091 | Lịch sử |
| `POST` | `/api/v1/assets/{id}/vulnerabilities` | asset-service:8091 | Thêm vulnerabilities cho asset — SEED-005-B |
| `GET` | `/api/v1/assets/{id}/findings` | in-gateway BFF → finding-service:8085 `/api/v2/findings?asset_id={id}` | Findings liên quan đến asset (BFF rewrite) |

### Protected Endpoints — SLA

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v1/sla/config` | sla-service:8086 | Cấu hình SLA |
| `PUT` | `/api/v1/sla/config` | sla-service:8086 | Cập nhật SLA config (Admin) |
| `GET` | `/api/v1/sla/overview` | in-gateway BFF → sla-service:8086 `/api/v2/sla-dashboard` | SLA overview (BFF rewrite) |
| `GET` | `/api/v2/sla-configurations` | sla-service:8086 | Danh sách cấu hình SLA |
| `POST` | `/api/v2/sla-configurations` | sla-service:8086 | Tạo cấu hình SLA |
| `POST` | `/api/v2/sla-configurations/bulk` | sla-service:8086 | Bulk create SLA configs (Admin, rate-limit 5/min) — SEED-006-A |
| `POST` | `/api/v2/sla-configurations/assign-bulk` | sla-service:8086 | Bulk assign SLA configs (Admin, rate-limit 5/min) — SEED-006-A |
| `GET` | `/api/v2/sla-configurations/{id}` | sla-service:8086 | Chi tiết cấu hình SLA |
| `PUT` | `/api/v2/sla-configurations/{id}` | sla-service:8086 | Cập nhật cấu hình SLA |
| `DELETE` | `/api/v2/sla-configurations/{id}` | sla-service:8086 | Xóa cấu hình SLA |
| `POST` | `/api/v2/sla-configurations/{id}/assign/{product_id}` | sla-service:8086 | Gán SLA cho product |
| `GET` | `/api/v2/sla-dashboard` | sla-service:8086 | SLA dashboard |
| `GET` | `/api/v2/sla-violations` | sla-service:8086 | Danh sách vi phạm SLA |
| `GET` | `/api/v2/sla-violations/{product_id}` | sla-service:8086 | Vi phạm SLA theo product |

### Protected Endpoints — Notifications

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v1/notifications/stream` | notification-service:8087 | SSE stream thông báo (token auth) |
| `GET` | `/api/v1/notifications` | notification-service:8087 | In-app notifications |
| `PATCH` | `/api/v1/notifications/{id}/read` | notification-service:8087 | Đánh dấu đã đọc |
| `POST` | `/api/v1/notifications/mark-all-read` | notification-service:8087 | Đánh dấu tất cả đã đọc |
| `GET` | `/api/v1/notifications/unread-count` | notification-service:8087 | Số thông báo chưa đọc |
| `GET` | `/api/v1/webhooks` | notification-service:8087 | Danh sách webhooks |
| `POST` | `/api/v1/webhooks` | notification-service:8087 | Tạo webhook |
| `GET` | `/api/v1/webhooks/deliveries` | notification-service:8087 | Danh sách deliveries (flat) — CR-009 |
| `GET` | `/api/v1/webhooks/stats/hourly` | notification-service:8087 | Webhook stats 24h — CR-009 |
| `POST` | `/api/v1/webhooks/deliveries/{id}/retry` | notification-service:8087 | Retry delivery — CR-009 |
| `DELETE` | `/api/v1/webhooks/{id}` | notification-service:8087 | Xóa webhook |
| `GET` | `/api/v1/webhooks/{id}/deliveries` | notification-service:8087 | Lịch sử gửi của webhook |
| `POST` | `/api/v1/webhooks/{id}/test` | notification-service:8087 | Test webhook (Admin) |
| `GET` | `/api/v2/notification-rules` | notification-service:8087 | Quy tắc thông báo |
| `POST` | `/api/v2/notification-rules` | notification-service:8087 | Tạo quy tắc |
| `POST` | `/api/v2/notification-rules/bulk` | notification-service:8087 | Bulk create notification rules — SEED-006-B |
| `PUT` | `/api/v2/notification-rules/{id}` | notification-service:8087 | Cập nhật quy tắc |
| `DELETE` | `/api/v2/notification-rules/{id}` | notification-service:8087 | Xóa quy tắc |
| `GET` | `/api/v2/system-notification-rules` | notification-service:8087 | Quy tắc hệ thống |
| `PUT` | `/api/v2/system-notification-rules` | notification-service:8087 | Cập nhật quy tắc hệ thống |
| `POST` | `/api/v2/webhooks/bulk` | notification-service:8087 | Bulk create webhooks (rate-limit 5/min) — SEED-006-B |
| `GET` | `/api/v2/subscriptions` | notification-service:8087 | Danh sách subscriptions |
| `POST` | `/api/v2/subscriptions` | notification-service:8087 | Tạo subscription |
| `POST` | `/api/v2/subscriptions/bulk` | notification-service:8087 | Bulk create subscriptions (rate-limit 10/min) — SEED-006-B |
| `DELETE` | `/api/v2/subscriptions/{id}` | notification-service:8087 | Xóa subscription |
| `GET` | `/api/v2/alerts` | notification-service:8087 | Alerts (user-scoped) |
| `GET` | `/api/v2/alerts/count` | notification-service:8087 | Số alerts |
| `POST` | `/api/v2/alerts/{id}/read` | notification-service:8087 | Đánh dấu alert đã đọc |
| `POST` | `/api/v2/alerts/read-all` | notification-service:8087 | Đánh dấu tất cả đã đọc |

### Protected Endpoints — Jira (v2)

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v2/jira-configurations` | jira-service:8088 | Danh sách cấu hình Jira |
| `POST` | `/api/v2/jira-configurations` | jira-service:8088 | Tạo cấu hình |
| `POST` | `/api/v2/jira-configurations/bulk` | jira-service:8088 | Bulk create Jira configs (Admin, rate-limit 3/min) — SEED-006-C |
| `GET` | `/api/v2/jira-configurations/{id}` | jira-service:8088 | Chi tiết cấu hình |
| `PUT` | `/api/v2/jira-configurations/{id}` | jira-service:8088 | Cập nhật cấu hình |
| `DELETE` | `/api/v2/jira-configurations/{id}` | jira-service:8088 | Xóa cấu hình |
| `GET` | `/api/v2/jira-issues` | jira-service:8088 | Danh sách Jira issues |
| `POST` | `/api/v2/jira-issues` | jira-service:8088 | Push finding lên Jira (rate-limit 20/min) |
| `GET` | `/api/v2/jira-issues/{finding_id}` | jira-service:8088 | Jira issues của finding |
| `DELETE` | `/api/v2/jira-issues/{id}` | jira-service:8088 | Xóa Jira issue link |

### Protected Endpoints — Audit Log (v2)

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v2/audit-log` | audit-service:8090 | Danh sách audit log |
| `GET` | `/api/v2/audit-log/{id}` | audit-service:8090 | Chi tiết log entry |
| `GET` | `/api/v2/audit-log/resource/{type}/{id}` | audit-service:8090 | Log theo resource |
| `GET` | `/api/v2/audit-log/actor/{user_id}` | audit-service:8090 | Log theo user |
| `GET` | `/api/v2/audit-log/export` | audit-service:8090 | Export audit log (rate-limit 2/min, timeout 120s) |

### Protected Endpoints — CVE Intel (v2)

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v2/epss/top` | data-service:8082 | Top EPSS |
| `GET` | `/api/v2/epss/distribution` | data-service:8082 | Phân phối EPSS |
| `GET` | `/api/v2/epss/{cveId}` | data-service:8082 | EPSS score của CVE — CR-005 |
| `GET` | `/api/v2/cwe` | data-service:8082 | Danh sách CWE |
| `GET` | `/api/v2/cwe/{id}` | data-service:8082 | Chi tiết CWE — CR-005 |
| `GET` | `/api/v2/capec/{id}` | data-service:8082 | Chi tiết CAPEC — CR-005 |
| `GET` | `/api/v2/vendors` | data-service:8082 | Danh sách vendors |
| `GET` | `/api/v2/dbinfo` | data-service:8082 | Database info (v2 alias) — CR-005 |

### Protected Endpoints — CVE Browse & Search

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `POST` | `/api/v2/cves/search` | search-service:8083 | Full-text search CVE |
| `POST` | `/api/v2/cves/search/semantic` | search-service:8083 | Semantic search CVE |
| `GET` | `/api/v2/cves/export` | search-service:8083 | Export CVEs (timeout 60s) |
| `GET` | `/api/v2/cves/{id}` | search-service:8083 | Chi tiết CVE (search index) |
| `GET` | `/api/v2/browse` | search-service:8083 | Browse vendors |
| `GET` | `/api/v2/browse/{vendor}` | search-service:8083 | Browse products của vendor |
| `GET` | `/api/v2/browse/{vendor}/{product}` | search-service:8083 | Browse CVEs của product |

### Protected Endpoints — CVE Write (data-service, SEED-004)

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `POST` | `/api/v2/cve/custom` | data-service:8082 | Tạo CVE tùy chỉnh (Admin, rate-limit 10/min) |
| `POST` | `/api/v2/cve/bulk-triage` | data-service:8082 | Bulk triage CVEs (rate-limit 20/min) |
| `POST` | `/api/v2/cve/import` | data-service:8082 | Import CVEs từ file (Admin, rate-limit 2/min, max 500MB) |
| `PUT` | `/api/v2/cve/{id}/triage` | data-service:8082 | Triage CVE cụ thể (rate-limit 30/min) |

### Protected Endpoints — Ranking (SEED-004)

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `POST` | `/api/v1/ranking/bulk` | ranking-service:8088 | Bulk create/update rankings (Admin, rate-limit 5/min) |

### Protected Endpoints — Tool Configurations

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v2/tool-configurations` | finding-service:8085 | Danh sách cấu hình tool |
| `POST` | `/api/v2/tool-configurations` | finding-service:8085 | Tạo cấu hình |
| `GET` | `/api/v2/tool-configurations/{id}` | finding-service:8085 | Chi tiết cấu hình |
| `PUT` | `/api/v2/tool-configurations/{id}` | finding-service:8085 | Cập nhật cấu hình |
| `DELETE` | `/api/v2/tool-configurations/{id}` | finding-service:8085 | Xóa cấu hình |

### Protected Endpoints — Metrics

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `GET` | `/api/v2/metrics/products` | finding-service:8085 | Metrics tất cả products |
| `GET` | `/api/v2/metrics/products/{id}` | finding-service:8085 | Metrics của product |
| `GET` | `/api/v2/metrics/findings/trends` | finding-service:8085 | Xu hướng findings |
| `GET` | `/api/v2/metrics/sla-compliance` | finding-service:8085 | Tỷ lệ tuân thủ SLA |

### Protected Endpoints — AI Services

| Method | Path | Upstream | Mô tả |
|--------|------|----------|--------|
| `POST` | `/api/v1/ai/triage` | ai-service:9103 | AI triage finding (legacy, body-param, timeout 60s) |
| `POST` | `/api/v1/ai/enrich` | ai-service:9103 | AI enrich CVE (legacy, body-param, timeout 60s) |
| `GET` | `/api/v1/ai/triage/queue` | ai-service:9103 | Xem hàng đợi triage — CR-014 |
| `POST` | `/api/v1/ai/triage/{findingId}` | ai-service:9103 | AI triage finding theo ID (async 202, timeout 60s) — CR-002 |
| `POST` | `/api/v1/ai/triage/{findingId}/review` | ai-service:9103 | Submit human review cho triage (timeout 30s) — CR-014 |
| `GET` | `/api/v1/ai/enrichment` | ai-service:9103 | Trạng thái enrichment pipeline — CR-002 |
| `POST` | `/api/v1/ai/enrichment/trigger` | ai-service:9103 | Trigger manual enrichment (timeout 60s) — CR-002 |
| `GET` | `/api/v1/ai/enrichment/{cveId}` | ai-service:9103 | Chi tiết enrichment của CVE — CR-002 |

---

## services/identity-service (port :8081)

**Source:** `services/identity-service/adapter/handler/http/router.go`

### Auth Endpoints (Public)

| Method | Path | Handler | Mô tả |
|--------|------|---------|--------|
| `GET` | `/health` | HealthCheck | Health check |
| `GET` | `/health/live` | HealthCheck | Liveness |
| `GET` | `/health/ready` | HealthCheck | Readiness |
| `GET` | `/.well-known/jwks.json` | authH.JWKS | Public JWKS (JWT verification) |
| `POST` | `/api/v1/auth/register` | authH.Register | Đăng ký |
| `POST` | `/api/v1/auth/login` | authH.Login | Đăng nhập |
| `POST` | `/api/v1/auth/refresh` | authH.Refresh | Refresh token |
| `GET` | `/api/v1/auth/providers` | GetProviders | Danh sách OAuth providers |
| `GET` | `/api/v1/auth/accept-invite` | authH.AcceptInvite | Activate user account qua invitation token — **TASK-HC-014** |


### OAuth2 Endpoints

| Method | Path | Handler | Mô tả |
|--------|------|---------|--------|
| `GET` | `/api/v1/auth/oauth/google` | oauthH.InitiateGoogle | Bắt đầu Google OAuth |
| `GET` | `/api/v1/auth/oauth/google/callback` | oauthH.CallbackGoogle | Callback Google OAuth |
| `GET` | `/api/v1/auth/oauth/github` | oauthH.InitiateGitHub | Bắt đầu GitHub OAuth |
| `GET` | `/api/v1/auth/oauth/github/callback` | oauthH.CallbackGitHub | Callback GitHub OAuth |

### Auth Endpoints (Protected — X-User-ID required)

| Method | Path | Handler | Mô tả |
|--------|------|---------|--------|
| `POST` | `/api/v1/auth/logout` | authH.Logout | Đăng xuất |
| `GET` | `/api/v1/auth/me` | authH.Me | Thông tin user |
| `POST` | `/api/v1/auth/totp/setup` | totpH.Setup | Cài đặt TOTP |
| `POST` | `/api/v1/auth/totp/verify` | totpH.Verify | Xác minh TOTP |
| `DELETE` | `/api/v1/auth/totp` | totpH.Disable | Vô hiệu hóa TOTP |
| `POST` | `/api/v1/auth/api-keys` | apiKeyH.CreateAPIKey | Tạo API key |
| `GET` | `/api/v1/auth/api-keys` | apiKeyH.ListAPIKeys | Danh sách API keys |
| `DELETE` | `/api/v1/auth/api-keys/{key_id}` | apiKeyH.RevokeAPIKey | Thu hồi API key |
| `GET` | `/api/v1/auth/profile` | profileH.GetProfile | Lấy hồ sơ |
| `PATCH` | `/api/v1/auth/profile` | profileH.UpdateProfile | Cập nhật hồ sơ |
| `POST` | `/api/v1/auth/profile/change-password` | profileH.ChangePassword | Đổi mật khẩu |
| `GET` | `/api/v1/auth/profile/sessions` | profileH.ListSessions | Danh sách sessions đang hoạt động |
| `DELETE` | `/api/v1/auth/profile/sessions/{sessionId}` | profileH.RevokeSession | Thu hồi session |
| `GET` | `/api/v1/auth/profile/notifications/settings` | profileH.GetNotifSettings | Lấy cài đặt thông báo |
| `PUT` | `/api/v1/auth/profile/notifications/settings` | profileH.UpdateNotifSettings | Cập nhật cài đặt thông báo |


### Admin Endpoints *(Updated — TASK-HC-009, HC-010, HC-014)*

| Method | Path | Handler | Mô tả |
|--------|------|---------|--------|
| `GET` | `/api/v1/admin/users` | adminH.ListUsers | Danh sách users |
| `POST` | `/api/v1/admin/users` | adminH.CreateUser | Tạo user mới |
| `POST` | `/api/v1/admin/users/bulk` | adminH.BulkCreateUsers | Bulk create users |
| `POST` | `/api/v1/admin/users/invite` | adminH.InviteUser | Mời user + tạo invitation token + gửi email — **TASK-HC-014** |
| `GET` | `/api/v1/admin/users/{id}` | adminH.GetUser | Chi tiết user — CR-001 |
| `PATCH` | `/api/v1/admin/users/{id}` | adminH.UpdateUser | Cập nhật user |
| `POST` | `/api/v1/admin/users/{id}/unlock` | adminH.UnlockUser | Mở khóa tài khoản |
| `POST` | `/api/v1/admin/users/{id}/api-keys` | adminH.CreateAPIKeyForUser | Tạo API key cho user — SEED-001 |
| `POST` | `/api/v1/admin/users/{id}/roles` | adminH.AssignRole | Gán role cho user — SEED-001-D |
| `GET` | `/api/v1/admin/roles` | adminH.GetRBACMatrix | RBAC matrix từ `rbac_roles` DB — **TASK-HC-010** |
| `GET` | `/api/v1/admin/settings` | settingsH.GetSettings | Đọc platform settings từ `platform_settings` DB — **TASK-HC-009** |
| `PUT` | `/api/v1/admin/settings` | settingsH.UpdateSettings | Cập nhật platform settings — **TASK-HC-009** |
| `PATCH` | `/api/v1/admin/settings` | settingsH.UpdateSettings | Partial update platform settings — **TASK-HC-009** |

**Query Parameters cho `GET /api/v1/admin/users`:**

| Param | Type | Mô tả |
|-------|------|--------|
| `q` | string | Tìm kiếm theo tên/email |
| `role` | string | Lọc theo role |
| `is_active` | bool | Lọc theo trạng thái |
| `page` | int | Số trang |
| `page_size` | int | Kích thước trang |

**Request Body `POST /api/v1/admin/users/invite`:**
```json
{
  "email": "newuser@example.com",
  "role": "user",
  "username": "newuser"
}
```

**Query params `GET /api/v1/auth/accept-invite`:**

| Param | Mô tả |
|-------|-------|
| `token` | Invitation token (32 bytes hex) |

---

## services/data-service (port :8082)

**Source:** `services/data-service/internal/delivery/http/`

### CVE Endpoints

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/health` | Health check |
| `GET` | `/info` | DB info |
| `GET` | `/cve/{id}` | Chi tiết CVE |
| `GET` | `/cve/last/{n}` | N CVEs mới nhất |
| `GET` | `/cve/recent/{timeframe}` | CVEs gần đây |
| `GET` | `/cve/search` | Tìm kiếm CVE theo CPE |
| `GET` | `/cve/query` | Query CVE |
| `POST` | `/query` | Advanced query |

**Query Parameters cho `GET /cve/search`:**

| Param | Type | Mô tả |
|-------|------|--------|
| `cpe` | string | CPE string |
| `lax` | bool | Tìm kiếm lax |
| `strict` | bool | Tìm kiếm strict |
| `format` | string | Output format |
| `enrich` | bool | Bổ sung thêm dữ liệu |
| `mode` | string | Chế độ tìm kiếm |
| `limit` | int | Giới hạn kết quả |
| `skip` | int | Bỏ qua N kết quả |

### KEV Endpoints

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v1/kev` | Danh sách KEV (v1) |
| `GET` | `/api/v1/kev/check` | Kiểm tra CVE trong KEV (v1) |
| `GET` | `/api/v1/kev/stats` | Thống kê KEV (v1) |
| `GET` | `/api/v1/kev/sync/status` | Trạng thái đồng bộ (v1) |
| `GET` | `/api/v1/kev/ransomware` | Ransomware KEV (v1) |
| `GET` | `/api/v1/kev/{cveId}` | Chi tiết KEV entry (v1) |
| `GET` | `/api/v2/kev` | Danh sách KEV (v2) |
| `GET` | `/api/v2/kev/check` | Kiểm tra CVE trong KEV (v2) |
| `GET` | `/api/v2/kev/stats` | Thống kê KEV (v2) |
| `GET` | `/api/v2/kev/sync/status` | Trạng thái đồng bộ (v2) |
| `GET` | `/api/v2/kev/ransomware` | Ransomware KEV (v2) |
| `GET` | `/api/v2/kev/{cveId}` | Chi tiết KEV entry (v2) |
| `POST` | `/internal/kev/sync` | Trigger sync (internal) |
| `GET` | `/internal/kev/ids` | Lấy tất cả KEV IDs (internal) |

### EPSS Endpoints

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v1/epss/top` | Top EPSS (v1) |
| `GET` | `/api/v1/epss/distribution` | Phân phối EPSS (v1) |
| `GET` | `/api/v2/epss/top` | Top EPSS (v2) |
| `GET` | `/api/v2/epss/distribution` | Phân phối EPSS (v2) |

### Taxonomy Endpoints

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v2/cwe` | Danh sách CWE |
| `GET` | `/cwe/{id}` | Chi tiết CWE |
| `GET` | `/cwe/{id}/capec` | CAPEC liên quan đến CWE |
| `GET` | `/capec/{id}` | Chi tiết CAPEC |
| `GET` | `/capec/{id}/cwe` | CWE liên quan đến CAPEC |

### Vendor Endpoints

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v2/vendors` | Autocomplete vendors |

---

## services/finding-service (port :8085)

**Source:** `services/finding-service/internal/delivery/http/router.go`

### Health

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/health` | Health check |

### Findings (v1)

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v1/findings` | Danh sách findings |
| `GET` | `/api/v1/findings/{id}` | Chi tiết finding |
| `PUT` | `/api/v1/findings/{id}/close` | Đóng finding |
| `PUT` | `/api/v1/findings/{id}/reopen` | Mở lại finding |
| `PUT` | `/api/v1/findings/{id}/false-positive` | Đánh dấu false positive |
| `PUT` | `/api/v1/findings/{id}/risk-accepted` | Chấp nhận rủi ro |
| `GET` | `/api/v1/findings/{id}/notes` | Ghi chú của finding |
| `POST` | `/api/v1/findings/{id}/notes` | Thêm ghi chú |

**Query Parameters cho `GET /api/v1/findings`:**

| Param | Type | Mô tả |
|-------|------|--------|
| `product_id` | UUID | Lọc theo product |
| `severity` | string | Lọc theo severity |
| `active_only` | bool | Chỉ findings đang active |

### Findings (v2) — Bulk ops, Seed & Notes

| Method | Path | Mô tả |
|--------|------|--------|
| `POST` | `/api/v2/findings/bulk-create` | Bulk create findings — SEED-003 |
| `POST` | `/api/v2/findings/import` | Import findings từ file — SEED-003 |
| `POST` | `/api/v2/findings/bulk` | Bulk update |
| `POST` | `/api/v2/findings/bulk_reopen` | Bulk reopen |
| `POST` | `/api/v2/findings/bulk_assign` | Bulk assign |
| `GET` | `/api/v2/findings/stats` | Thống kê findings |
| `GET` | `/api/v2/findings/severity_count` | Đếm theo severity (alias của stats) |
| `GET` | `/api/v2/findings/{id}/notes` | Ghi chú |
| `POST` | `/api/v2/findings/{id}/notes` | Thêm ghi chú |

### Products (v1 compat)

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v1/products` | Danh sách |
| `POST` | `/api/v1/products` | Tạo |
| `GET` | `/api/v1/products/grades` | Grades |
| `GET` | `/api/v1/products/{id}` | Chi tiết |
| `PUT` | `/api/v1/products/{id}` | Cập nhật |
| `PATCH` | `/api/v1/products/{id}` | Partial update (alias PUT) — CR-003 |
| `DELETE` | `/api/v1/products/{id}` | Xóa |
| `GET` | `/api/v1/products/{id}/engagements` | Engagements |
| `POST` | `/api/v1/products/{id}/engagements` | Tạo engagement cho product — CR-003 |

### Products (v2) + Seed

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v2/products` | Danh sách |
| `POST` | `/api/v2/products` | Tạo |
| `GET` | `/api/v2/products/grades` | Product grades |
| `POST` | `/api/v2/products/bulk` | Bulk create — SEED-002 |
| `POST` | `/api/v2/products/import` | Import — SEED-002 |
| `GET` | `/api/v2/products/{id}` | Chi tiết |
| `PUT` | `/api/v2/products/{id}` | Cập nhật |
| `DELETE` | `/api/v2/products/{id}` | Xóa |
| `POST` | `/api/v2/products/{id}/seed` | Seed product — SEED-002 |

### Product Types + Seed

| Method | Path | Mô tả |
|--------|------|--------|
| `POST` | `/api/v2/product-types/bulk` | Bulk create product types — SEED-002 |

### Engagements (v1 compat)

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v1/engagements` | Danh sách |
| `POST` | `/api/v1/engagements` | Tạo |
| `GET` | `/api/v1/engagements/{id}` | Chi tiết |
| `POST` | `/api/v1/engagements/{id}/close` | Đóng |
| `POST` | `/api/v1/engagements/{id}/reopen` | Mở lại |
| `GET` | `/api/v1/engagements/{id}/tests` | Tests của engagement |

### Engagements (v2)

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v2/engagements` | Danh sách |
| `POST` | `/api/v2/engagements` | Tạo |
| `GET` | `/api/v2/engagements/{id}` | Chi tiết |
| `POST` | `/api/v2/engagements/{id}/close` | Đóng |
| `POST` | `/api/v2/engagements/{id}/reopen` | Mở lại |

### Tests (v2)

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v2/tests` | Danh sách |
| `POST` | `/api/v2/tests` | Tạo |
| `GET` | `/api/v2/tests/{id}` | Chi tiết |
| `DELETE` | `/api/v2/tests/{id}` | Xóa |

### Members (v2)

| Method | Path | Mô tả |
|--------|------|--------|
| `POST` | `/api/v2/members` | Thêm member |
| `DELETE` | `/api/v2/members/{id}` | Xóa member |

### Tool Configurations (v2)

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v2/tool-configurations` | Danh sách |
| `POST` | `/api/v2/tool-configurations` | Tạo |
| `GET` | `/api/v2/tool-configurations/{id}` | Chi tiết |
| `PUT` | `/api/v2/tool-configurations/{id}` | Cập nhật |
| `DELETE` | `/api/v2/tool-configurations/{id}` | Xóa |

### Reports (v2)

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v2/reports/templates` | Report templates — CR-010 |
| `POST` | `/api/v2/reports/generate` | Tạo report |
| `GET` | `/api/v2/reports` | Danh sách |
| `GET` | `/api/v2/reports/{id}` | Chi tiết |
| `GET` | `/api/v2/reports/{id}/download` | Tải report |
| `DELETE` | `/api/v2/reports/{id}` | Xóa |

### Risk Acceptances (v2)

| Method | Path | Mô tả |
|--------|------|--------|
| `POST` | `/api/v2/risk-acceptances` | Tạo |

### Finding Groups (v2)

| Method | Path | Mô tả |
|--------|------|--------|
| `POST` | `/api/v2/finding-groups` | Tạo nhóm |

### Internal Endpoints

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/internal/stats` | Thống kê tổng |
| `GET` | `/internal/risk-trend` | Xu hướng rủi ro |
| `GET` | `/internal/product-grades` | Grades theo product |
| `GET` | `/internal/sla-breaches` | Vi phạm SLA |
| `POST` | `/internal/findings/count-by-cve-ids` | Đếm findings theo CVE IDs |
| `GET` | `/internal/sla-dashboard` | SLA dashboard (nội bộ) |

---

## services/sla-service (port :8086)

**Source:** `services/sla-service/cmd/server/main.go`, `services/sla-service/internal/delivery/http/`

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/health` | Health check |
| `GET` | `/api/v1/sla/config` | Cấu hình SLA toàn cục `{global, product_overrides}` |
| `PUT` | `/api/v1/sla/config` | Cập nhật cấu hình SLA toàn cục |
| `GET` | `/api/v2/sla-configurations` | Danh sách cấu hình SLA |
| `POST` | `/api/v2/sla-configurations` | Tạo cấu hình SLA |
| `POST` | `/api/v2/sla-configurations/bulk` | Bulk create SLA configs — SEED-006-A |
| `POST` | `/api/v2/sla-configurations/assign-bulk` | Bulk assign SLA configs — SEED-006-A |
| `GET` | `/api/v2/sla-configurations/{id}` | Chi tiết |
| `PUT` | `/api/v2/sla-configurations/{id}` | Cập nhật |
| `DELETE` | `/api/v2/sla-configurations/{id}` | Xóa |
| `POST` | `/api/v2/sla-configurations/{id}/assign/{product_id}` | Gán SLA cho product |
| `GET` | `/api/v2/sla-dashboard` | SLA dashboard (filter by `?product_id=`) |
| `GET` | `/api/v2/sla-violations` | Danh sách vi phạm SLA (filter: severity, days_overdue_min, limit, offset) |
| `GET` | `/api/v2/sla-violations/{product_id}` | Vi phạm SLA theo product |

---

## services/notification-service (port :8087)

**Source:** `services/notification-service/internal/delivery/http/router.go`

### Webhooks (v1)

| Method | Path | Mô tả |
|--------|------|--------|
| `POST` | `/api/v1/webhooks/bulk` | Bulk create webhooks — SEED-006-B |
| `GET` | `/api/v1/webhooks` | Danh sách webhooks |
| `POST` | `/api/v1/webhooks` | Tạo webhook |
| `GET` | `/api/v1/webhooks/deliveries` | Danh sách tất cả deliveries (flat) — CR-009 |
| `GET` | `/api/v1/webhooks/stats/hourly` | Webhook stats 24h theo giờ — CR-009 |
| `POST` | `/api/v1/webhooks/deliveries/{id}/retry` | Retry failed delivery — CR-009 |
| `DELETE` | `/api/v1/webhooks/{id}` | Xóa webhook |
| `GET` | `/api/v1/webhooks/{id}/deliveries` | Lịch sử gửi của webhook |
| `POST` | `/api/v1/webhooks/{id}/test` | Test webhook |

> **Chú ý:** `GET /api/v1/webhook_deliveries` là alias legacy của `GET /api/v1/webhooks/deliveries`

### Subscriptions (v2)

| Method | Path | Mô tả |
|--------|------|--------|
| `POST` | `/api/v2/subscriptions/bulk` | Bulk create subscriptions — SEED-006-B |
| `GET` | `/api/v2/subscriptions` | Danh sách subscriptions |
| `POST` | `/api/v2/subscriptions` | Tạo subscription |
| `DELETE` | `/api/v2/subscriptions/{id}` | Xóa subscription |

### In-app Notifications (v2)

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v2/notifications` | Danh sách thông báo |
| `GET` | `/api/v2/notifications/stream` | SSE stream |
| `PATCH` | `/api/v2/notifications/{id}/read` | Đánh dấu đã đọc |
| `POST` | `/api/v2/notifications/mark-all-read` | Đánh dấu tất cả đã đọc |
| `GET` | `/api/v2/notifications/unread-count` | Số chưa đọc |

### Rules (v2)

| Method | Path | Mô tả |
|--------|------|--------|
| `POST` | `/api/v2/notification-rules/bulk` | Bulk create rules — SEED-006-B |
| `GET` | `/api/v2/notification-rules` | Danh sách rules |
| `POST` | `/api/v2/notification-rules` | Tạo rule |
| `PUT` | `/api/v2/notification-rules/{id}` | Cập nhật rule |
| `DELETE` | `/api/v2/notification-rules/{id}` | Xóa rule |

### Health & Internal

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/health` | Health check |
| `POST` | `/internal/events/cve` | Nhận CVE events (internal) |

---

## services/jira-service (port :8088)

**Source:** `services/jira-service/internal/delivery/http/router.go`

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/health` | Health check |
| `POST` | `/webhooks/jira/{config_id}` | Jira webhook (HMAC-verified, no JWT auth) |
| `POST` | `/api/v2/jira-configurations/bulk` | Bulk create Jira configs — SEED-006-C |
| `GET` | `/api/v2/jira-configurations/{product_id}` | Lấy cấu hình Jira |
| `POST` | `/api/v2/jira-configurations` | Tạo/cập nhật cấu hình |
| `POST` | `/api/v2/jira-configurations/test` | Test kết nối Jira |
| `GET` | `/jira-configs` | Legacy path compat |

---

## services/audit-service (port :8090)

**Source:** `services/audit-service/internal/delivery/http/router.go`

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/health` | Health check |
| `GET` | `/audit-log` | Danh sách audit log (admin auth enforced by gateway) |

**Query Parameters cho `GET /audit-log`:**

| Param | Type | Mô tả |
|-------|------|--------|
| `action` | string | Lọc theo action |
| `entity_type` | string | Lọc theo loại entity |
| `entity_id` | string | Lọc theo entity ID |
| `user_id` | UUID | Lọc theo user |
| `date_from` | RFC3339 | Từ ngày |
| `date_to` | RFC3339 | Đến ngày |
| `page` | int | Số trang |
| `page_size` | int | Kích thước trang |

---

## services/scan-service (port :8084)

**Source:** `services/scan-service/internal/adapters/handler/http/scan_handler.go`, `services/scan-service/internal/delivery/http/router.go`, `services/scan-service/internal/delivery/http/schedule/schedule_handler.go`

### Scan Handler (adapters — legacy server binary)

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/health/live` | Liveness |
| `GET` | `/health/ready` | Readiness |
| `POST` | `/api/v1/scans` | Tạo scan mới (→ 202 Accepted) |
| `GET` | `/api/v1/scans` | Danh sách scans |
| `GET` | `/api/v1/scans/stats/weekly` | Stats theo tuần — CR-008 |
| `GET` | `/api/v1/scans/stats` | Stats tổng — CR-008 |
| `GET` | `/api/v1/scans/{id}` | Chi tiết scan |
| `DELETE` | `/api/v1/scans/{id}` | Hủy/Cancel scan |
| `GET` | `/api/v1/scans/{id}/findings` | Findings của scan |
| `POST` | `/api/v2/reimport-scan` | Reimport scan |
| `GET` | `/api/v2/test-imports/{id}` | Chi tiết test import |
| `GET` | `/api/v2/parsers` | Danh sách parsers |

### Scan Delivery Router (main delivery layer)

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/health` | Health check |
| `GET` | `/api/v1/scans` | Danh sách scans |
| `POST` | `/api/v1/scans` | Tạo scan |
| `GET` | `/api/v1/scans/stats` | Stats tổng |
| `GET` | `/api/v1/scans/{id}` | Chi tiết scan |
| `POST` | `/api/v1/scans/{id}/cancel` | Hủy scan |
| `GET` | `/api/v1/scans/scheduled` | Danh sách scheduled scans (PostgreSQL `ScheduleRepo` — **TASK-HC-011**) |
| `POST` | `/api/v1/scans/scheduled` | Tạo scheduled scan — SEED-005-C |
| `GET` | `/api/v1/scans/scheduled/{id}` | Chi tiết — SEED-005-C |
| `PUT` | `/api/v1/scans/scheduled/{id}` | Cập nhật — SEED-005-C |
| `DELETE` | `/api/v1/scans/scheduled/{id}` | Xóa — SEED-005-C |
| `POST` | `/api/v1/scans/import` | **501 Not Implemented** — feature planned (**TASK-HC-011**) |
| `POST` | `/api/v1/agents` | Đăng ký agent — SEED-005-C |
| `GET` | `/api/v1/agents` | Danh sách agents — SEED-005-C |
| `GET` | `/api/v1/agents/{id}` | Chi tiết agent — SEED-005-C |
| `POST` | `/api/v1/agents/{id}/reports` | Submit báo cáo từ agent — SEED-005-C |
| `POST` | `/api/v2/reimport-scan` | Reimport scan |
| `GET` | `/api/v2/test-imports/{id}` | Chi tiết test import |
| `GET` | `/api/v2/parsers` | Danh sách parsers |

> **TASK-HC-011 Changes:**
> - `scheduleHandler` wired với real PostgreSQL `ScheduleRepo` (không còn nil)
> - `importHandler`/`parserHandler` nil trong embedded mode — router nil-safe, không panic
> - `POST /api/v1/scans/import` trả **501 Not Implemented** (feature planned)


**Query Parameters cho `GET /api/v1/scans`:**

| Param | Type | Mô tả |
|-------|------|--------|
| `page` | int | Số trang (default 1) |
| `page_size` | int | Kích thước trang (default 20, max 100) |
| `status` | string | Lọc theo status |

### Scheduled Scans (Schedule Handler — standalone router)

**Source:** `services/scan-service/internal/delivery/http/schedule/schedule_handler.go`

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/health/live` | Liveness |
| `GET` | `/health/ready` | Readiness |
| `POST` | `/api/v1/schedules` | Tạo scheduled scan |
| `GET` | `/api/v1/schedules` | Danh sách scheduled scans |
| `GET` | `/api/v1/schedules/{id}` | Chi tiết |
| `PUT` | `/api/v1/schedules/{id}` | Cập nhật |
| `DELETE` | `/api/v1/schedules/{id}` | Xóa |

---

## services/search-service (port :8083)

**Source:** `services/search-service/internal/delivery/http/search_handler.go`

### CVE Search

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/health` | Health check |
| `GET` | `/api/v2/cves` | Tìm kiếm CVE |
| `GET` | `/api/v2/cves/{id}` | Chi tiết CVE |
| `POST` | `/api/v2/cves/search` | Full-text search CVE |
| `POST` | `/api/v2/cves/search/semantic` | Semantic search CVE (503 khi AI unavailable — **TASK-HC-005**) |
| `GET` | `/api/v2/cves/search/semantic/suggestions` | Semantic search suggestions |
| `GET` | `/api/v2/cves/aggregations` | Aggregations |
| `GET` | `/api/v2/cves/export` | Export CVEs |
| `GET` | `/api/v1/search/recent` | Lịch sử tìm kiếm gần đây — **TASK-HC-007** |
| `GET` | `/api/v1/search/suggested` | Gợi ý tìm kiếm — **TASK-HC-007** |
| `GET` | `/internal/cves/count` | Đếm tổng CVEs (internal) |


**Query Parameters cho `GET /api/v2/cves`:**

| Param | Type | Mô tả |
|-------|------|--------|
| `query` | string | Từ khóa tìm kiếm |
| `severity` | string | CRITICAL/HIGH/MEDIUM/LOW |
| `source` | string | Nguồn dữ liệu |
| `sort` | string | Trường sắp xếp |
| `page` | int | Số trang |
| `limit` | int | Kích thước trang (mặc định: 50) |
| `cwe` | string | Lọc theo CWE |
| `vendor` | string | Lọc theo vendor |
| `product` | string | Lọc theo product |
| `min_epss` | float | EPSS tối thiểu |
| `max_epss` | float | EPSS tối đa |
| `is_kev` | bool | Chỉ KEV |
| `is_exploit` | bool | Có exploit |

### EPSS Endpoints

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v2/epss/stats` | EPSS statistics |
| `GET` | `/api/v2/epss/top` | Top EPSS scores |

### Taxonomy

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v2/cwe` | Danh sách CWE |
| `GET` | `/api/v2/cwe/{id}` | Chi tiết CWE |
| `GET` | `/api/v2/capec` | Danh sách CAPEC |
| `GET` | `/api/v2/capec/{id}` | Chi tiết CAPEC |

### Vendors & Products

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v2/vendors` | Danh sách vendors |
| `GET` | `/api/v2/vendors/{vendor}/products` | Products của vendor |
| `GET` | `/api/v2/products` | Tất cả products |

### Browse (Compat)

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/browse/` | Danh sách vendors |
| `GET` | `/browse/vendors` | Danh sách vendors |
| `GET` | `/browse/vendors/{vendor}/products` | Products của vendor |
| `GET` | `/browse/{vendor}` | Legacy compat |

### Dashboard Stats

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/api/v2/stats/dashboard` | Dashboard statistics |

### Internal (OpenSearch)

| Method | Path | Mô tả |
|--------|------|--------|
| `POST` | `/internal/opensearch/index` | Index CVE |
| `POST` | `/internal/opensearch/bulk` | Bulk index CVEs |

### OSV v1 API (Proxy)

| Method | Path | Mô tả |
|--------|------|--------|
| `POST` | `/v1/query` | OSV query |
| `POST` | `/v1/querybatch` | OSV batch query |
| `GET` | `/v1/vulns/list` | Danh sách vulnerabilities |
| `GET` | `/v1/vulns/{id}` | Chi tiết vulnerability |

---

## services/ranking-service

**Source:** `services/ranking-service/internal/delivery/http/router.go`

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/health` | Health check |
| `GET` | `/ranking/lookup` | Tra cứu ranking theo CPE |
| `GET` | `/ranking` | Danh sách rankings |
| `POST` | `/ranking` | Tạo ranking |
| `DELETE` | `/ranking/{id}` | Xóa ranking |

**Query Parameters cho `GET /ranking`:**

| Param | Type | Mô tả |
|-------|------|--------|
| `cpe` | string | CPE string |
| `limit` | int | Giới hạn kết quả |
| `skip` | int | Bỏ qua N kết quả |

---

## services/ai-service (port :9103)

**Source:** `services/ai-service/internal/delivery/http/ai_handler.go`

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/health` | Health check |
| `POST` | `/api/v1/ai/triage` | AI triage một finding (legacy, body-param) |
| `POST` | `/api/v1/ai/enrich` | AI enrich một CVE (legacy, body-param) |
| `GET` | `/api/v1/ai/triage/queue` | Xem hàng đợi triage với stats — CR-014 |
| `POST` | `/api/v1/ai/triage/{findingId}` | AI triage theo finding ID (async 202) — CR-002 |
| `POST` | `/api/v1/ai/triage/{findingId}/review` | Submit human review decision — CR-014 |
| `GET` | `/api/v1/ai/enrichment` | Trạng thái enrichment pipeline — CR-002 |
| `POST` | `/api/v1/ai/enrichment/trigger` | Trigger manual enrichment cho list CVEs — CR-002 |
| `GET` | `/api/v1/ai/enrichment/{cveId}` | Chi tiết enrichment của CVE cụ thể — CR-002 |
| `POST` | `/api/v1/ai/enrichment/batch` | **Batch enrich** danh sách CVEs song song (202 Accepted + job_id async) — **TASK-HC-012** |
| `GET` | `/api/v1/ai/insights` | AI insights tổng quan |


**Query Parameters cho `GET /api/v1/ai/triage/queue`:**

| Param | Type | Mô tả |
|-------|------|--------|
| `status` | string | pending/accepted/overridden/rejected/all (default: pending) |
| `severity` | string | Lọc theo severity |
| `remarks` | string | Lọc theo remarks |

**Query Parameters cho `POST /api/v1/ai/triage/{findingId}/review`:**

Request body: `{ "decision": "accepted|overridden|rejected", "note": "..." }`  
Query: `?force=true` để ghi đè review đã có (tránh 409 Conflict)

---

## services/asset-service (port :8091)

**Source:** `services/asset-service/internal/delivery/http/handlers.go`

| Method | Path | Mô tả |
|--------|------|--------|
| `GET` | `/assets` | Danh sách assets |
| `POST` | `/assets` | Tạo asset |
| `GET` | `/assets/{id}` | Chi tiết asset |
| `PUT` | `/assets/{id}` | Cập nhật asset |
| `PUT` | `/assets/{id}/tags` | Cập nhật tags |
| `GET` | `/assets/{id}/risk` | Risk score |
| `GET` | `/assets/{id}/history` | Lịch sử |
| `GET` | `/assets/{id}/findings` | Findings liên quan |

**Query Parameters cho `GET /assets`:**

| Param | Type | Mô tả |
|-------|------|--------|
| `q` | string | Tìm kiếm |
| `status` | string | Lọc theo trạng thái |
| `os` | string | Lọc theo OS |
| `port` | int | Lọc theo port |
| `tag` | string | Lọc theo tag |
| `page` | int | Số trang |
| `limit` | int | Kích thước trang |

---

## services/product-service

**Source:** `services/product-service/internal/delivery/http/handlers.go`

| Method | Path | Mô tả |
|--------|------|--------|
| `POST` | `/orchestrate` | Orchestrate product workflow |
| `POST` | `/products` | Tạo product |
| `GET` | `/products` | Danh sách products |
| `GET` | `/products/{id}` | Chi tiết product |
| `POST` | `/products/{id}/engagements` | Tạo engagement cho product |
| `GET` | `/products/{id}/engagements` | Danh sách engagements |

---

## services/report-service

**Source:** `services/report-service/internal/delivery/http/handlers.go`

| Method | Path | Mô tả |
|--------|------|--------|
| `POST` | `/reports/generate` | Tạo report |
| `GET` | `/reports/{runID}` | Trạng thái report |
| `GET` | `/reports/{runID}/download/{format}` | Tải report (format: pdf/html/json) |
| `GET` | `/reports/{runID}/exit-code` | Exit code của report job |
