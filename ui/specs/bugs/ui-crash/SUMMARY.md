# OSV Platform — UI Scan Report

**Scanned**: 2026-06-20 00:06:10  
**Base URL**: https://c12.openledger.vn  
**Method**: React Router client-side navigation (pushState — không reload)  
**Total pages**: 32 | ✅ **16 OK** | ❌ **16 ERROR**

---

## Tổng hợp lỗi theo loại

| Loại lỗi | Số trang | Trang bị ảnh hưởng |
|----------|----------|---------------------|
| 🔴 **JS Crash** (TypeError / React ErrorBoundary) | 7 | CWE Library, Assets, Products, Webhooks, RBAC, System Health |
| 🟠 **API 404 Not Found** | 8 | Scans, Findings Risk, Reports, Notifications, API Keys, Audit Logs, Settings, Risk Acceptance |
| 🟡 **API 500 Server Error** | 1 | Findings — All Findings |
| 🔵 **API 503 Service Unavailable** | 2 | AI Triage, AI Enrichment |
| ⚫ **React Error #31** (render object as child) | 1 | Admin — RBAC Management (26 errors!) |

---

## Kết quả chi tiết

| # | Page | Route | Status | Real Err |
|---|------|-------|--------|----------|
| 1 | Dashboard — Main | `/dashboard` | ✅ OK | 0 |
| 2 | Dashboard — Executive Overview | `/dashboard/executive` | ✅ OK | 0 |
| 3 | Dashboard — Risk Overview | `/dashboard/risk` | ✅ OK | 0 |
| 4 | Dashboard — SLA Dashboard | `/dashboard/sla` | ✅ OK | 0 |
| 5 | CVE Intel — CVE Search | `/cve/search` | ✅ OK | 0 |
| 6 | CVE Intel — KEV Catalog | `/cve/kev` | ✅ OK | 0 |
| 7 | CVE Intel — Semantic Search | `/cve/semantic` | ✅ OK | 0 |
| 8 | CVE Intel — EPSS Analytics | `/cve/epss` | ✅ OK | 0 |
| 9 | CVE Intel — Vendor Catalog | `/cve/vendors` | ✅ OK | 0 |
| 10 | CVE Intel — **CWE Library** | `/cve/cwe` | ❌ ERROR | 4 |
| 11 | CVE Intel — CAPEC Library | `/cve/capec` | ✅ OK | 0 |
| 12 | Scanning — **Scan Dashboard** | `/scans` | ❌ ERROR | 3 |
| 13 | Scanning — New Scan Wizard | `/scans/new` | ✅ OK | 0 |
| 14 | Scanning — Running Scans | `/scans/running` | ✅ OK | 0 |
| 15 | Scanning — Scan History | `/scans/history` | ✅ OK | 0 |
| 16 | Findings — **All Findings** | `/findings` | ❌ ERROR | 3 |
| 17 | Findings — **Risk Acceptance Center** | `/findings/risk-acceptance` | ❌ ERROR | 3 |
| 18 | Assets — **Asset Inventory** | `/assets` | ❌ ERROR | 4 |
| 19 | **Product Security** | `/products` | ❌ ERROR | 4 |
| 20 | AI Center — **AI Triage Queue** | `/ai/triage` | ❌ ERROR | 3 |
| 21 | AI Center — **AI Enrichment** | `/ai/enrichment` | ❌ ERROR | 3 |
| 22 | Reports — **Report Center** | `/reports` | ❌ ERROR | 6 |
| 23 | Notifications — **Notification Center** | `/notifications` | ❌ ERROR | 3 |
| 24 | Integrations — **API Key Management** | `/integrations/api-keys` | ❌ ERROR | 3 |
| 25 | Integrations — **Webhook Events** | `/integrations/webhooks` | ❌ ERROR | 4 |
| 26 | Admin — User Management | `/admin/users` | ✅ OK | 0 |
| 27 | Admin — **RBAC Management** | `/admin/roles` | ❌ ERROR | 26 |
| 28 | Admin — **Audit Logs** | `/admin/audit` | ❌ ERROR | 3 |
| 29 | Admin — **System Health** | `/admin/health` | ❌ ERROR | 4 |
| 30 | Admin — **System Settings** | `/admin/settings` | ❌ ERROR | 3 |
| 31 | User — Profile | `/profile` | ✅ OK | 0 |
| 32 | User — Onboarding | `/onboarding` | ✅ OK | 0 |

---

## ❌ Chi tiết bugs

### BUG-001 — CWE Library: JS Crash (TypeError undefined.map)
- **Route**: `/cve/cwe` | **File**: [BUG-cve-cwe.md](BUG-cve-cwe.md)
- **Lỗi**: `TypeError: Cannot read properties of undefined (reading 'map')`
- **Nguyên nhân**: API trả về `undefined` thay vì array, component không kiểm tra null trước khi `.map()`

### BUG-002 — Scan Dashboard: API 404
- **Route**: `/scans` | **File**: [BUG-scans.md](BUG-scans.md)
- **Lỗi**: `Failed to load resource: 404`
- **Nguyên nhân**: Endpoint scan dashboard chưa được deploy hoặc route backend sai

### BUG-003 — Findings All: API 500 Server Error
- **Route**: `/findings` | **File**: [BUG-findings.md](BUG-findings.md)
- **Lỗi**: `Failed to load resource: 500`
- **Nguyên nhân**: Backend lỗi khi truy vấn findings — cần kiểm tra server logs

### BUG-004 — Risk Acceptance Center: API 404
- **Route**: `/findings/risk-acceptance` | **File**: [BUG-findings-risk-acceptance.md](BUG-findings-risk-acceptance.md)
- **Lỗi**: `Failed to load resource: 404`
- **Nguyên nhân**: Endpoint `/risk-acceptance` chưa được implement

### BUG-005 — Asset Inventory: JS Crash (TypeError null.filter)
- **Route**: `/assets` | **File**: [BUG-assets.md](BUG-assets.md)
- **Lỗi**: `TypeError: Cannot read properties of null (reading 'filter')`
- **Nguyên nhân**: API trả về `null` thay vì array, component không guard null

### BUG-006 — Product Security: JS Crash (TypeError undefined.flatMap)
- **Route**: `/products` | **File**: [BUG-products.md](BUG-products.md)
- **Lỗi**: `TypeError: Cannot read properties of undefined (reading 'flatMap')`
- **Nguyên nhân**: Data từ API không match expected shape

### BUG-007 — AI Triage Queue: API 503 Service Unavailable
- **Route**: `/ai/triage` | **File**: [BUG-ai-triage.md](BUG-ai-triage.md)
- **Lỗi**: `Failed to load resource: 503`
- **Nguyên nhân**: AI service chưa chạy hoặc đang down

### BUG-008 — AI Enrichment: API 503 Service Unavailable
- **Route**: `/ai/enrichment` | **File**: [BUG-ai-enrichment.md](BUG-ai-enrichment.md)
- **Lỗi**: `Failed to load resource: 503`
- **Nguyên nhân**: AI service chưa chạy hoặc đang down

### BUG-009 — Report Center: API 404
- **Route**: `/reports` | **File**: [BUG-reports.md](BUG-reports.md)
- **Lỗi**: `Failed to load resource: 404` (6 lần)
- **Nguyên nhân**: Report API endpoints chưa được deploy

### BUG-010 — Notification Center: API 404
- **Route**: `/notifications` | **File**: [BUG-notifications.md](BUG-notifications.md)
- **Lỗi**: `Failed to load resource: 404`
- **Nguyên nhân**: Notification API endpoint chưa được deploy

### BUG-011 — API Key Management: API 404
- **Route**: `/integrations/api-keys` | **File**: [BUG-integrations-api-keys.md](BUG-integrations-api-keys.md)
- **Lỗi**: `Failed to load resource: 404`
- **Nguyên nhân**: Integration API endpoints chưa được deploy

### BUG-012 — Webhook Events: JS Crash (TypeError undefined.find)
- **Route**: `/integrations/webhooks` | **File**: [BUG-integrations-webhooks.md](BUG-integrations-webhooks.md)
- **Lỗi**: `TypeError: Cannot read properties of undefined (reading 'find')`
- **Nguyên nhân**: Component gọi `.find()` trên data chưa load xong

### BUG-013 — RBAC Management: React Error #31 (26 errors — CRITICAL)
- **Route**: `/admin/roles` | **File**: [BUG-admin-roles.md](BUG-admin-roles.md)
- **Lỗi**: `Minified React error #31` — render object as React child
- **Nguyên nhân**: Một object được render trực tiếp vào JSX thay vì string/number. Cần decode: `invariant=31&args[]=object with keys {description, label, value}`

### BUG-014 — Audit Logs: API 404
- **Route**: `/admin/audit` | **File**: [BUG-admin-audit.md](BUG-admin-audit.md)
- **Lỗi**: `Failed to load resource: 404`
- **Nguyên nhân**: Audit log API endpoint chưa được deploy

### BUG-015 — System Health: JS Crash (TypeError undefined.includes)
- **Route**: `/admin/health` | **File**: [BUG-admin-health.md](BUG-admin-health.md)
- **Lỗi**: `TypeError: Cannot read properties of undefined (reading 'includes')` tại `SystemHealth.js:1:994`
- **Nguyên nhân**: Health check data shape không match — field bị `undefined` khi gọi `.includes()`

### BUG-016 — System Settings: API 404
- **Route**: `/admin/settings` | **File**: [BUG-admin-settings.md](BUG-admin-settings.md)
- **Lỗi**: `Failed to load resource: 404`
- **Nguyên nhân**: Settings API endpoint chưa được deploy

---

## Phân loại ưu tiên sửa

| Priority | Bug | Lý do |
|----------|-----|-------|
| 🔴 P0 | BUG-013 RBAC Management | 26 errors — trang crash hoàn toàn |
| 🔴 P0 | BUG-003 Findings (500) | Backend error — mất dữ liệu |
| 🔴 P0 | BUG-005 Assets (null.filter) | JS crash — trang không dùng được |
| 🟠 P1 | BUG-001 CWE Library | JS crash — undefined.map |
| 🟠 P1 | BUG-006 Product Security | JS crash — undefined.flatMap |
| 🟠 P1 | BUG-012 Webhooks | JS crash — undefined.find |
| 🟠 P1 | BUG-015 System Health | JS crash — undefined.includes |
| 🟡 P2 | BUG-007,008 AI Services | 503 — service down |
| 🟡 P2 | BUG-002,004,009–011,014,016 | API 404 — endpoints missing |