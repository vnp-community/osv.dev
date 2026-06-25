# User Requirements Document (URD) — OSV Platform

**Version:** 3.0  
**Ngày cập nhật:** 2026-06-16  
**Trạng thái:** v2.2 Active — v3.0 Target  

---

## 1. Introduction

Tài liệu này định nghĩa **User Requirements** của OSV Platform, tập trung vào nhu cầu của các nhóm người dùng được xác định từ các Change Requests (v1 + v2). Mỗi user requirement (UR) có thể được trace về một hoặc nhiều CR cụ thể.

Ký hiệu:
- ✅ Implemented trong v2.x
- 🔵 Planned trong v3.0 (OpenVulnScan CRs)
- 🔶 Planned trong v3.1 (UI-API CRs — API-first frontend contracts)

---

## 2. User Personas

| Persona | Tên | Vai trò |
|---------|-----|---------| 
| **Alice** | DevSecOps Engineer | Tích hợp CVE scanning vào CI/CD pipeline |
| **Bob** | Security Analyst | Tra cứu CVE, quản lý findings, import scan results |
| **Carol** | Security Manager / CISO | Báo cáo, SLA, tổng quan bảo mật |
| **Dave** | Tool Builder / Platform Integrator | Tích hợp OSV API vào sản phẩm khác |
| **Eve** | Security Researcher / CERT | Nghiên cứu CVE, taxonomy, bulk export |
| **Frank** | Frontend/Web User | Sử dụng React SPA — cần UI/UX mượt mà, dashboard, real-time notifications |

---

## 3. User Requirements

### 3.1 Alice — DevSecOps Engineer

#### CVE Search & API
- **UR-01** ✅: Tôi muốn query CVE theo package name + version để biết dependencies có lỗ hổng không.
- **UR-02** ✅: Tôi muốn query CVE theo CPE (vendor/product/version) với strict và lax matching mode.  
  *(CR-001: CVE Search by CPE)*
- **UR-03** ✅: Tôi muốn API trả về trong < 100ms để không làm chậm CI pipeline.
- **UR-04** ✅: Tôi cần exit code trong report (A–F grade) để tự động fail/pass build dựa trên finding severity.  
  *(CR-DD-009: Product Grading)*

#### CI/CD Integration
- **UR-05** ✅: Tôi muốn tích hợp scan import pipeline: push scan result → platform tự tạo Test và Findings.  
  *(CR-DD-001: Product hierarchy, CR-DD-002: Scan import)*
- **UR-06** ✅: Tôi muốn nhận thông báo khi scan mới tìm thấy Critical vulnerability.  
  *(CR-DD-007: Notification Service, CR-GCV-006)*
- **UR-07** ✅: Tôi muốn filter báo cáo theo severity để chỉ fail build với High+ CVEs.  
  *(CR-DD-009: Report filtering)*

#### Authentication
- **UR-08** ✅: Tôi muốn tạo API key với scoped permissions để dùng trong CI pipeline không cần username/password.  
  *(CR-007: API Key Management)*

---

### 3.2 Bob — Security Analyst

#### CVE Research
- **UR-09** ✅: Tôi muốn search CVE bằng keyword và filter theo severity, vendor, CWE.  
  *(CR-GCV-004: OpenSearch FTS)*
- **UR-10** ✅: Tôi muốn browse danh sách vendors → products → CVEs theo hierarchical structure.  
  *(CR-002: Browse)*
- **UR-11** ✅: Tôi muốn xem EPSS score (exploit probability) của từng CVE và sort theo EPSS.  
  *(CR-GCV-002: EPSS)*
- **UR-12** ✅: Tôi muốn xem CISA KEV và biết CVE nào đang bị khai thác trong thực tế.  
  *(CR-GCV-007: KEV)*
- **UR-13** ✅: Tôi muốn xem CVE có exploit ngoài thực tế (ExploitDB) hay không (`is_exploit` flag).  
  *(CR-GCV-001: ExploitDB fetcher)*

#### Finding Management
- **UR-14** ✅: Tôi muốn import scan results từ tool bên ngoài (Nmap XML, Trivy, Bandit, Snyk...) qua parser factory.  
  *(CR-DD-002: Scan import 21+ parsers)*
- **UR-15** ✅: Tôi muốn xem danh sách findings sau scan, filter theo severity/CVE/status.  
  *(CR-DD-001: Finding list)*
- **UR-16** ✅: Tôi muốn đánh dấu một finding là "False Positive" hoặc "Accept Risk" với comment lý do.  
  *(CR-DD-004: State machine, CR-DD-005: Risk acceptance)*
- **UR-17** ✅: Tôi muốn close (mitigate) một finding sau khi đã fix, với timestamp và user tracking.  
  *(CR-DD-004: Close/reopen)*
- **UR-18** ✅: Tôi muốn reopen finding nếu vulnerability tái xuất hiện.  
  *(CR-DD-004: Reopen)*
- **UR-19** ✅: Tôi muốn xem audit trail đầy đủ của một finding — ai thay đổi gì, khi nào.  
  *(CR-DD-010: Audit Service)*
- **UR-20** ✅: Tôi muốn hệ thống tự động detect duplicate findings (cùng CVE + component) và link về original.  
  *(CR-DD-003: Deduplication)*

#### [Planned v3.0] Active Scanning
- **UR-21** 🔵: Tôi muốn khởi chạy Nmap full scan trên một subnet và nhận danh sách CVEs trên từng host.  
  *(CR-OVS-001: Scan Service Nmap)*
- **UR-22** 🔵: Tôi muốn chạy OWASP ZAP active scan trên một web application URL và nhận web alerts.  
  *(CR-OVS-001: Scan Service ZAP)*
- **UR-23** 🔵: Tôi muốn xem real-time progress của scan đang chạy qua SSE stream.  
  *(CR-OVS-001: SSE progress)*
- **UR-24** 🔵: Tôi muốn lên lịch quét định kỳ (daily/weekly) cho danh sách targets.  
  *(CR-OVS-007: Scheduled Scans)*
- **UR-25** 🔵: Tôi muốn nhận AI-suggested triage recommendation (Confirmed/FalsePositive) cho finding.  
  *(CR-OVS-005: AI Triage)*

---

### 3.3 Carol — Security Manager / CISO

#### Báo cáo & Dashboard
- **UR-26** ✅: Tôi muốn báo cáo PDF executive summary với severity distribution chart và danh sách CVE nghiêm trọng.  
  *(CR-DD-009: PDF Report)*
- **UR-27** ✅: Tôi muốn báo cáo HTML với light/dark theme, có thể chia sẻ với team.  
  *(CR-DD-009: HTML Report)*
- **UR-28** ✅: Tôi muốn export findings ra Excel/XLSX theo format DefectDojo.  
  *(CR-DD-009: Excel Formatter)*
- **UR-29** ✅: Tôi muốn xem "Product Grade" (A–F) của từng product dựa trên tình trạng findings.  
  *(CR-DD-009: Product Grading)*
- **UR-30** ✅: Tôi muốn xem CVE aggregations: severity distribution, top vendors, CVE by year.  
  *(CR-GCV-004: Aggregations, CR-GCV-007: KEV stats)*

#### SLA Management
- **UR-31** ✅: Tôi muốn hệ thống tự động tính SLA deadline cho mỗi finding (Critical: 7 ngày, High: 30 ngày...).  
  *(CR-DD-006: SLA Service)*
- **UR-32** ✅: Tôi muốn nhận cảnh báo khi một finding đã breach SLA.  
  *(CR-DD-006: SLA breach notification)*
- **UR-33** ✅: Tôi muốn configure SLA policy riêng cho từng product.  
  *(CR-DD-006: Per-product SLA)*

#### Notifications
- **UR-34** ✅: Tôi muốn nhận thông báo qua Email/Slack/Teams khi có Critical CVE mới, SLA breach.  
  *(CR-DD-007: Notification channels)*
- **UR-35** ✅: Tôi muốn đăng ký webhook để nhận events khi CISA KEV catalog có CVE mới.  
  *(CR-GCV-006: Webhook, CR-GCV-007)*

---

### 3.4 Dave — Tool Builder / Platform Integrator

#### API & Data Access
- **UR-36** ✅: Tôi cần REST API ổn định để query CVE theo CPE, package/version, keyword.  
  *(Core CVE API, CR-001)*
- **UR-37** ✅: Tôi cần API response time < 100ms average.  
  *(NFR-01)*
- **UR-38** ✅: Tôi cần bulk data dumps (JSON/CSV) để mirror database hoặc analyze offline.  
  *(CR-GCV-010: Export)*
- **UR-39** ✅: Tôi cần OpenAPI spec tổng hợp từ tất cả services qua Gateway.  
  *(CR-GCV-008: Gateway)*

#### Webhook & Events
- **UR-40** ✅: Tôi muốn đăng ký webhook với HMAC-SHA256 verification để nhận events an toàn.  
  *(CR-GCV-006: Webhook HMAC)*
- **UR-41** ✅: Tôi muốn subscribe events theo vendor/product/severity để chỉ nhận relevant alerts.  
  *(CR-GCV-006: Subscription filters)*
- **UR-42** ✅: Tôi muốn webhook delivery có retry logic (3 attempts, exponential backoff).  
  *(CR-GCV-006: Retry/backoff)*

#### Integration
- **UR-43** ✅: Tôi muốn tích hợp JIRA: finding được tạo tự động tạo JIRA ticket, và khi ticket closed → finding closed.  
  *(CR-DD-008: JIRA bidirectional)*
- **UR-44** ✅: Tôi muốn import scan results từ tool bên ngoài (Bandit, Trivy, Snyk...) qua parser factory.  
  *(CR-DD-002: Scan import 21+ parsers)*

---

### 3.5 Eve — Security Researcher / CERT

#### CVE Research
- **UR-45** ✅: Tôi muốn search CVE bằng natural language query ("buffer overflow in web server").  
  *(CR-GCV-004: Semantic search)*
- **UR-46** ✅: Tôi muốn filter CVE theo CWE (e.g., `?cwe=CWE-89` cho SQL Injection) hoặc CAPEC attack pattern.  
  *(CR-GCV-003: CWE/CAPEC filter, CR-003: Taxonomy)*
- **UR-47** ✅: Tôi muốn xem danh sách vendor, chọn vendor → xem products, chọn product → xem CVEs.  
  *(CR-002: Browse, CR-GCV-005: CPE)*
- **UR-48** ✅: Tôi muốn xem EPSS score và sort CVEs theo exploit probability.  
  *(CR-GCV-002: EPSS)*
- **UR-49** ✅: Tôi muốn xem CISA KEV và filter CVEs đang bị khai thác ngoài thực tế.  
  *(CR-GCV-007: KEV)*
- **UR-50** ✅: Tôi muốn xem CVE có exploit ngoài thực tế (`is_exploit` flag từ ExploitDB).  
  *(CR-GCV-001: ExploitDB fetcher)*
- **UR-51** ✅: Tôi muốn export kết quả search thành JSON hoặc CSV với source attribution đầy đủ.  
  *(CR-GCV-010: Export + attribution)*
- **UR-52** ✅: Tôi muốn xem Atom/RSS feed của CVEs mới nhất để subscribe theo RSS reader.  
  *(CR-008: CVE feeds)*
- **UR-53** ✅: Tôi muốn xem thống kê database: số CVE theo source, ngày sync gần nhất.  
  *(CR-005: DB statistics)*

#### Taxonomy
- **UR-54** ✅: Tôi muốn lookup CWE theo ID, xem description và CAPEC patterns liên quan.  
  *(CR-003: CWE lookup, CR-GCV-003)*
- **UR-55** ✅: Tôi muốn lookup CAPEC attack pattern theo ID, xem mitigations và liên kết CVEs.  
  *(CR-003: CAPEC lookup, CR-GCV-003)*
- **UR-56** ✅: Tôi muốn xem aggregations: top vulnerable vendors, severity distribution, CVE by month.  
  *(CR-GCV-004: Aggregations, CR-GCV-007: KEV stats)*

---

## 4. Non-Functional Requirements (User-facing)

| ID | Requirement | Status | CR |
|----|------------|--------|-----|
| NFR-U-01 | API response < 100ms cho CVE lookup (P95) | ✅ | Core |
| NFR-U-02 | Search results trả về trong < 500ms | ✅ | CR-GCV-004 |
| NFR-U-03 | Report generation < 30 giây cho 1000 findings | ✅ | CR-DD-009 |
| NFR-U-04 | Webhook delivery retry tối đa 3 lần | ✅ | CR-GCV-006 |
| NFR-U-05 | SLA breach notification trong vòng 5 phút | ✅ | CR-DD-006 |
| NFR-U-06 | API Key validation < 5ms | ✅ | CR-007 |
| NFR-U-07 | Platform available 99.9% | Target | Core |
| NFR-U-08 | NATS events delivered at-least-once | ✅ | Core |
| NFR-U-09 | [v3.0] Scan progress SSE độ trễ < 2 giây | 🔵 | CR-OVS-001 |
| NFR-U-10 | [v3.0] Nmap /24 subnet scan < 5 phút | 🔵 | CR-OVS-001 |
| NFR-U-11 | [v3.1] Dashboard BFF response < 500ms | 🔶 | CR-UI-002 |
| NFR-U-12 | [v3.1] Auth middleware < 5ms per request | 🔶 | CR-UI-001 |
| NFR-U-13 | [v3.1] SSE notification latency < 2 giây | 🔶 | CR-UI-002 |
| NFR-U-14 | [v3.1] Login rate limit: 5 req/min per IP | 🔶 | CR-UI-001 |
| NFR-U-15 | [v3.1] Account lockout sau 5 failed attempts | 🔶 | CR-UI-001 |

---

## 5. User Requirements Traceability Matrix

| UR | Persona | CR tham chiếu | Status |
|----|---------|--------------|--------|
| UR-01 ~ UR-03 | Alice | Core CVE API | ✅ |
| UR-04 | Alice | CR-DD-009 | ✅ |
| UR-05 | Alice | CR-DD-001, CR-DD-002 | ✅ |
| UR-06 | Alice | CR-DD-007, CR-GCV-006 | ✅ |
| UR-07 | Alice | CR-DD-009 | ✅ |
| UR-08 | Alice | CR-007 | ✅ |
| UR-09 ~ UR-13 | Bob | CR-GCV-004, CR-002, CR-GCV-002, CR-GCV-007, CR-GCV-001 | ✅ |
| UR-14 ~ UR-20 | Bob | CR-DD-002, CR-DD-001, CR-DD-004, CR-DD-005, CR-DD-010, CR-DD-003 | ✅ |
| UR-21 ~ UR-25 | Bob | CR-OVS-001, CR-OVS-007, CR-OVS-005 | 🔵 |
| UR-26 ~ UR-30 | Carol | CR-DD-009, CR-GCV-004, CR-GCV-007 | ✅ |
| UR-31 ~ UR-33 | Carol | CR-DD-006 | ✅ |
| UR-34 ~ UR-35 | Carol | CR-DD-007, CR-GCV-006 | ✅ |
| UR-36 ~ UR-39 | Dave | Core API, CR-GCV-010, CR-GCV-008 | ✅ |
| UR-40 ~ UR-42 | Dave | CR-GCV-006 | ✅ |
| UR-43 ~ UR-44 | Dave | CR-DD-008, CR-DD-002 | ✅ |
| UR-45 | Eve | CR-GCV-004 | ✅ |
| UR-46 | Eve | CR-GCV-003, CR-003 | ✅ |
| UR-47 | Eve | CR-002, CR-GCV-005 | ✅ |
| UR-48 | Eve | CR-GCV-002 | ✅ |
| UR-49 | Eve | CR-GCV-007 | ✅ |
| UR-50 | Eve | CR-GCV-001 | ✅ |
| UR-51 | Eve | CR-GCV-010 | ✅ |
| UR-52 | Eve | CR-008 | ✅ |
| UR-53 | Eve | CR-005 | ✅ |
| UR-54 ~ UR-56 | Eve | CR-003, CR-GCV-003, CR-GCV-004 | ✅ |
| UR-57 ~ UR-62 | Frank | CR-UI-001, CR-UI-002 | 🔶 |
| UR-63 ~ UR-66 | Alice+Bob | CR-UI-003, CR-UI-005 | 🔶 |
| UR-67 ~ UR-72 | All | CR-UI-007, CR-UI-009, CR-UI-010 | 🔶 |

---

## 6. Frank — Frontend / Web UI User

### 6.1 Authentication & Session

- **UR-57** 🔶: Tôi muốn đăng nhập bằng email + password và nhận JWT access token trong < 500ms.  
  *(CR-UI-001 §2.1: POST /api/v1/auth/login)*
- **UR-58** 🔶: Tôi muốn token tự động refresh khi hết hạn (15 min) mà không cần đăng nhập lại.  
  *(CR-UI-001 §2.2: POST /api/v1/auth/refresh — httpOnly cookie)*
- **UR-59** 🔶: Tôi muốn xem thông tin user hiện tại (role, permissions) khi app load.  
  *(CR-UI-001 §2.3: GET /api/v1/auth/me)*
- **UR-60** 🔶: Tôi muốn logout an toàn — revoke token + xóa refresh cookie.  
  *(CR-UI-001 §2.4: POST /api/v1/auth/logout)*
- **UR-61** 🔶: Tôi muốn đăng nhập qua Google/GitHub OAuth2 không cần đặt password.  
  *(CR-UI-001 §2.7-2.9: OAuth2 flow — v3.0 feature, phụ thuộc CR-OVS-003)*
- **UR-62** 🔶: Tôi muốn cài đặt MFA (TOTP) để tăng bảo mật tài khoản.  
  *(CR-UI-001 §2.5-2.6: MFA setup/confirm — v3.0 feature)*

### 6.2 Dashboard & Real-Time

- **UR-63** 🔶: Tôi muốn xem Executive Dashboard với KPIs (Critical findings, Security Grade, SLA compliance) trong một API call.  
  *(CR-UI-002 §2.1: GET /api/v1/dashboard — BFF aggregate < 500ms)*
- **UR-64** 🔶: Tôi muốn nhận notifications real-time (SLA breach, KEV mới, scan completed) qua SSE stream.  
  *(CR-UI-002 §2.3: GET /api/v1/notifications/stream)*
- **UR-65** 🔶: Tôi muốn xem chi tiết SLA compliance per product với trend chart.  
  *(CR-UI-002 §2.2: GET /api/v1/dashboard/sla)*

### 6.3 CVE Intelligence UI

- **UR-66** 🔶: Khi search CVE, tôi muốn thấy `is_kev`, `has_exploit`, `epss_score`, và `sources` trong kết quả.  
  *(CR-UI-003: CVE search schema update)*
- **UR-67** 🔶: Tôi muốn autocomplete vendor name khi gõ vào search box.  
  *(CR-UI-003: GET /api/v2/vendors)*
- **UR-68** 🔶: Tôi muốn xem EPSS distribution chart và top 20 CVEs by EPSS score.  
  *(CR-UI-003: GET /api/v2/epss/top, /api/v2/epss/distribution)*

### 6.4 Finding Management UI

- **UR-69** 🔶: Khi đổi trạng thái finding, tôi muốn nhận 409 rõ ràng nếu transition không hợp lệ.  
  *(CR-UI-005: `INVALID_TRANSITION` error code)*
- **UR-70** 🔶: Tôi muốn thấy `sla_days_left`, `is_kev`, và `epss_score` trong finding list.  
  *(CR-UI-005: Finding schema update)*
- **UR-71** 🔶: Tôi muốn bulk assign/reopen nhiều findings cùng lúc.  
  *(CR-UI-005: POST /api/v1/findings/bulk/reopen, bulk/assign)*

### 6.5 Admin & Reports UI

- **UR-72** 🔶: Tôi muốn quản lý users (invite, lock, reset password) qua Admin UI.  
  *(CR-UI-010: Admin User Management)*
- **UR-73** 🔶: Tôi muốn download report file (PDF/Excel/CSV) trực tiếp từ UI.  
  *(CR-UI-009: GET /api/v1/reports/{id}/download)*
- **UR-74** 🔶: Tôi muốn xem System Health dashboard cho tất cả microservices.  
  *(CR-UI-010: GET /api/v1/admin/health — fan-out đến tất cả services)*

