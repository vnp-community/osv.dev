# OSV Platform — Danh sách các trang UI và đường dẫn truy vấn

> **Nguồn**: Phân tích từ `ui/src/app/router.tsx` và `ui/src/app/components/Sidebar.tsx`  
> **Cập nhật lần cuối**: 2026-06-19  
> **Framework**: React Router v7 (createBrowserRouter)

---

## Tổng quan kiến trúc routing

```
/                        → redirect đến /dashboard (AuthGuard)
/login                   → Public (không cần đăng nhập)
/auth/callback           → Public — OAuth2 callback
/* (còn lại)             → Protected (yêu cầu JWT, qua AppLayout với Sidebar+Topbar)
```

---

## 1. Auth — Xác thực

| Trang | Đường dẫn | Component | Mô tả |
|-------|-----------|-----------|-------|
| Đăng nhập | `/login` | `LoginScreen` | Form đăng nhập (username/password + OAuth) — **Public** |
| OAuth Callback | `/auth/callback` | `OAuthCallback` | Xử lý redirect từ OAuth provider — **Public** |
| Onboarding | `/onboarding` | `OnboardingExperience` | Hướng dẫn khởi động lần đầu cho người dùng mới |
| Hồ sơ cá nhân | `/profile` | `UserProfile` | Xem và chỉnh sửa thông tin tài khoản cá nhân |

---

## 2. Dashboard — Bảng điều khiển

| Trang | Đường dẫn | Component | Mô tả |
|-------|-----------|-----------|-------|
| Dashboard chính | `/dashboard` | `Dashboard` | Tổng quan toàn hệ thống |
| Executive Overview | `/dashboard/executive` | `Dashboard` | Báo cáo cấp lãnh đạo (CISO/CTO view) |
| Risk Overview | `/dashboard/risk` | `RiskOverview` | Tổng quan rủi ro bảo mật |
| SLA Dashboard | `/dashboard/sla` | `SLADashboard` | Theo dõi tuân thủ SLA của các finding |

---

## 3. Vulnerability Intel — Thông tin lỗ hổng bảo mật

| Trang | Đường dẫn | Component | Mô tả |
|-------|-----------|-----------|-------|
| CVE Search | `/cve/search` | `CVESearch` | Tìm kiếm CVE theo ID, keyword, bộ lọc |
| Semantic Search | `/cve/semantic` | `SemanticSearch` | Tìm kiếm lỗ hổng theo ngữ nghĩa (AI-powered) |
| Vendor Catalog | `/cve/vendors` | `VendorCatalog` | Danh mục nhà cung cấp và CVE liên quan |
| KEV Catalog | `/cve/kev` | `KEVCatalog` | Known Exploited Vulnerabilities (CISA KEV) |
| EPSS Analytics | `/cve/epss` | `EPSSAnalytics` | Phân tích xác suất khai thác (Exploit Prediction Scoring System) |
| CWE Library | `/cve/cwe` | `CWELibrary` | Thư viện Common Weakness Enumeration |
| CAPEC Library | `/cve/capec` | `CAPECLibrary` | Thư viện Common Attack Pattern Enumeration & Classification |

---

## 4. Active Scanning — Quét bảo mật tích cực

| Trang | Đường dẫn | Component | Mô tả |
|-------|-----------|-----------|-------|
| Scan Dashboard | `/scans` | `ScanDashboard` | Tổng quan tất cả các lần quét |
| New Scan | `/scans/new` | `ScanWizard` | Wizard tạo lần quét mới |
| Running Scans | `/scans/running` | `RunningScans` | Danh sách các scan đang chạy |
| Scan History | `/scans/history` | `ScanHistory` | Lịch sử quét |
| Scan Detail | `/scans/:id` | `RunningScan` | Chi tiết theo dõi một lần quét đang chạy |
| Nmap Results | `/scans/:id/results/nmap` | `NmapResults` | Kết quả quét mạng Nmap |
| ZAP Results | `/scans/:id/results/zap` | `ZAPResults` | Kết quả quét web OWASP ZAP |

### Lưu ý về tham số động (`:id`)

- `:id` = UUID của scan job
- Sidebar shortcut sử dụng `/scans/latest/results/nmap` và `/scans/latest/results/zap` (alias)

---

## 5. Findings — Kết quả phát hiện lỗ hổng

| Trang | Đường dẫn | Component | Mô tả |
|-------|-----------|-----------|-------|
| All Findings | `/findings` | `FindingsList` | Danh sách tất cả findings với bộ lọc |
| Finding Detail | `/findings/:id` | `FindingDetail` | Chi tiết một finding cụ thể |
| Risk Acceptance | `/findings/risk-acceptance` | `RiskAcceptanceCenter` | Trung tâm quản lý chấp nhận rủi ro |

> **Lưu ý**: `/findings/risk-acceptance` phải được khai báo **trước** `/findings/:id` trong router để tránh xung đột với dynamic route.

---

## 6. Assets — Quản lý tài sản

| Trang | Đường dẫn | Component | Mô tả |
|-------|-----------|-----------|-------|
| Asset Inventory | `/assets` | `AssetInventory` | Kho tài sản IT (servers, services, apps) |
| Asset Detail | `/assets/:id` | `AssetDetail` | Chi tiết một tài sản cụ thể |

---

## 7. Product Security — Bảo mật sản phẩm

| Trang | Đường dẫn | Component | Mô tả |
|-------|-----------|-----------|-------|
| Product Security | `/products` | `ProductSecurity` | Danh sách sản phẩm, engagements và security scorecards |

> Sidebar liệt kê các mục con (Products, Engagements, Security Scorecards) nhưng cùng map về `/products`.

---

## 8. AI Center — Trung tâm AI

| Trang | Đường dẫn | Component | Mô tả |
|-------|-----------|-----------|-------|
| AI Triage Queue | `/ai/triage` | `AITriage` | Hàng đợi phân loại lỗ hổng bằng AI |
| AI Enrichment | `/ai/enrichment` | `AIEnrichment` | Làm giàu thông tin CVE bằng AI |

---

## 9. Reports — Báo cáo

| Trang | Đường dẫn | Component | Mô tả |
|-------|-----------|-----------|-------|
| Report Center | `/reports` | `ReportCenter` | Tạo và xuất báo cáo (executive, technical, compliance) |

---

## 10. Notifications — Thông báo

| Trang | Đường dẫn | Component | Mô tả |
|-------|-----------|-----------|-------|
| Notification Center | `/notifications` | `NotificationCenter` | Trung tâm thông báo và cảnh báo hệ thống |

---

## 11. Integrations — Tích hợp

| Trang | Đường dẫn | Component | Mô tả |
|-------|-----------|-----------|-------|
| API Key Management | `/integrations/api-keys` | `APIKeyManagement` | Quản lý API keys |
| Webhook Events | `/integrations/webhooks` | `WebhookEvents` | Cấu hình và xem lịch sử webhook events |

> Sidebar có mục "Jira" nhưng cũng map về `/integrations/webhooks` (chưa có route riêng).

---

## 12. Administration — Quản trị hệ thống

| Trang | Đường dẫn | Component | Mô tả |
|-------|-----------|-----------|-------|
| User Management | `/admin/users` | `UserManagement` | Quản lý người dùng |
| RBAC Management | `/admin/roles` | `RBACManagement` | Quản lý vai trò và phân quyền |
| Audit Logs | `/admin/audit` | `AuditLogs` | Nhật ký kiểm tra hệ thống |
| System Health | `/admin/health` | `SystemHealth` | Trạng thái sức khoẻ hệ thống |
| System Settings | `/admin/settings` | `SystemSettings` | Cài đặt hệ thống (cũng link từ user footer) |

---

## Tổng hợp tất cả route theo thứ tự khai báo trong router

```
PUBLIC (không cần auth):
  /login
  /auth/callback

PROTECTED (cần JWT, trong AppLayout):
  /                          → redirect /dashboard
  /dashboard
  /dashboard/executive
  /dashboard/risk
  /dashboard/sla
  /cve/search
  /cve/kev
  /cve/semantic
  /cve/epss
  /cve/vendors
  /cve/cwe
  /cve/capec
  /scans
  /scans/new
  /scans/running
  /scans/history
  /scans/:id
  /scans/:id/results/nmap
  /scans/:id/results/zap
  /findings
  /findings/risk-acceptance
  /findings/:id
  /assets
  /assets/:id
  /products
  /ai/triage
  /ai/enrichment
  /reports
  /notifications
  /integrations/api-keys
  /integrations/webhooks
  /admin/users
  /admin/roles
  /admin/audit
  /admin/health
  /admin/settings
  /profile
  /onboarding

CATCH-ALL:
  *                          → redirect /dashboard
```

---

## Sidebar navigation map (phân nhóm theo menu)

```
Dashboard
  ├── Executive Overview       /dashboard/executive
  ├── Risk Overview            /dashboard/risk
  └── SLA Dashboard            /dashboard/sla

Vulnerability Intel
  ├── CVE Search               /cve/search
  ├── Semantic Search          /cve/semantic
  ├── Vendor Catalog           /cve/vendors
  ├── KEV Catalog              /cve/kev
  ├── EPSS Analytics           /cve/epss
  ├── CWE Library              /cve/cwe
  └── CAPEC Library            /cve/capec

Active Scanning               [badge: 3 running]
  ├── Scan Dashboard           /scans
  ├── New Scan                 /scans/new
  ├── Running Scans            /scans/running  [badge: 3]
  ├── Scan History             /scans/history
  ├── Nmap Results             /scans/latest/results/nmap
  └── ZAP Results              /scans/latest/results/zap

Findings                      [badge: 245 critical]
  ├── All Findings             /findings       [badge: 245]
  ├── Active                   /findings
  ├── Mitigated                /findings
  ├── SLA Dashboard            /dashboard/sla
  └── Risk Acceptance          /findings/risk-acceptance

Assets
  ├── Asset Inventory          /assets
  └── Asset Detail             /assets

Product Security
  ├── Products                 /products
  ├── Engagements              /products
  └── Security Scorecards      /products

AI Center
  ├── AI Triage Queue          /ai/triage
  ├── AI Enrichment            /ai/enrichment
  └── AI Insights              /ai/triage

Reports
  ├── Executive Reports        /reports
  ├── Technical Reports        /reports
  └── Compliance Reports       /reports

Notifications                  /notifications  [badge: 12]

Integrations
  ├── API Keys                 /integrations/api-keys
  ├── Webhooks                 /integrations/webhooks
  └── Jira                     /integrations/webhooks  ← chưa có route riêng

Administration
  ├── Users                    /admin/users
  ├── Roles & Permissions      /admin/roles
  ├── Audit Logs               /admin/audit
  ├── System Health            /admin/health
  └── System Settings          /admin/settings

── Quick Links ──
  Getting Started              /onboarding
  My Profile                   /profile
```

---

## Các vấn đề cần chú ý / TODO

| # | Vấn đề | Mô tả |
|---|--------|-------|
| 1 | **Jira chưa có route** | Sidebar item "Jira" (`id: jira`) map về `/integrations/webhooks` — cần trang riêng `/integrations/jira` |
| 2 | **Sub-reports chưa phân trang** | "Executive Reports", "Technical Reports", "Compliance Reports" cùng map về `/reports` |
| 3 | **Sub-products chưa phân trang** | "Products", "Engagements", "Security Scorecards" cùng map về `/products` |
| 4 | **AI Insights** | Map về `/ai/triage` — cần route `/ai/insights` riêng |
| 5 | **Findings Active/Mitigated** | Các mục "Active" và "Mitigated" trong sidebar cùng map về `/findings` thay vì có query-param hoặc route riêng |
| 6 | **Sidebar Latest Scan** | Shortcut `/scans/latest/results/nmap` và `/scans/latest/results/zap` dùng alias "latest" nhưng router chỉ định nghĩa `/scans/:id/results/...` |
