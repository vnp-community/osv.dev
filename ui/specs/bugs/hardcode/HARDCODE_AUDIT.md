# Hardcode Audit — OSV Platform UI

> **Phạm vi rà soát:** `/Users/binhnt/Lab/sec/cve/osv.dev/ui/src`  
> **Ngày rà soát:** 2026-06-19  
> **Trạng thái:** COMPLETED

---

## Tóm tắt

| Loại hardcode | Số lượng file bị ảnh hưởng |
|---|---|
| Dữ liệu giả (mock data) dạng const array | 13 |
| Ngưỡng số (threshold) hardcode | 8 |
| Chuỗi UI / nhãn / tiêu đề không có i18n | 12 |
| Màu sắc (color token) không dùng design system | 14 |
| Cấu hình hệ thống giả lập | 4 |
| Metadata tĩnh (version, copyright) | 1 |

---

## Chi tiết từng file

---

### 1. `features/admin/components/UserManagement.tsx`

**Vấn đề:** Toàn bộ danh sách người dùng được hardcode trong `const users = [...]` ngay trong component.

```
const users = [
  { id: "u-1", name: "Carol Anderson", email: "carol@company.com", role: "Admin", mfa: true, ... },
  { id: "u-2", name: "Bob Chen", email: "bob.chen@company.com", ... },
  ...7 items
];
```

**Loại:** Mock data — Dữ liệu người dùng thực phải lấy từ API `/api/admin/users`  
**Tác động:** Danh sách người dùng không phản ánh thực tế, tính năng tìm kiếm không có ý nghĩa  
**Gợi ý khắc phục:** Thay `const users = [...]` bằng hook `useUsers()` gọi `GET /api/admin/users`

---

### 2. `features/admin/components/AuditLogs.tsx`

**Vấn đề:** Toàn bộ audit log hardcode thành `const auditLogs = [...]` với 10 bản ghi.

```
const auditLogs = [
  { id: "AL-1001", timestamp: "2026-06-14 09:42:15", user: "carol@company.com", action: "CREATE_SCAN", ... },
  { id: "AL-1002", timestamp: "2026-06-14 09:35:01", user: "bob.chen@company.com", action: "UPDATE_FINDING", ... },
  ...
];
```

**Loại:** Mock data — Audit log là dữ liệu immutable phải lấy từ backend  
**Tác động:** Audit trail không thực tế, không có phân trang, không có lọc theo thời gian thực  
**Gợi ý khắc phục:** Tạo hook `useAuditLogs({ severity, search, page })` gọi `GET /api/admin/audit-logs`

---

### 3. `features/admin/components/RBACManagement.tsx`

**Vấn đề:** Toàn bộ danh sách roles, permissions, và permission matrix hardcode:

```
const ROLES = ["Admin", "User", "Readonly", "Agent"];
const PERMISSIONS = [
  { category: "Dashboard", items: ["dashboard.view", "dashboard.export"] },
  ...9 categories, 31 permissions
];
const MATRIX: Record<string, Record<string, boolean>> = {
  "dashboard.view": { Admin: true, User: true, Readonly: true, Agent: false },
  ...31 entries
};
```

Thêm vào đó, thông tin số user theo role hardcode:
```
{ role: "Admin", desc: "Full system access", users: 2, color: "..." },
{ role: "User", desc: "Standard analyst access", users: 4, color: "..." },
{ role: "Readonly", desc: "View-only access", users: 3, color: "..." },
{ role: "Agent", desc: "Automated scanning", users: 1, color: "..." },
```

**Loại:** Mock data + Config data  
**Tác động:** RBAC matrix không thể cập nhật từ UI, số lượng user theo role không đúng thực tế  
**Gợi ý khắc phục:** Gọi `GET /api/admin/rbac/matrix` và `GET /api/admin/rbac/roles`

---

### 4. `features/admin/components/SystemSettings.tsx`

**Vấn đề:** Cấu hình AI providers và giá trị mặc định của form settings đều hardcode:

```
const aiProviders = [
  { id: "openai", name: "OpenAI", model: "gpt-4o", status: "active", latency: "203ms", usage: "4,821 req/day", cost: "$12.40/day" },
  { id: "azure", name: "Azure OpenAI", model: "gpt-4-turbo", status: "standby", ... },
  { id: "ollama", name: "Ollama (Local)", model: "llama3:8b", status: "inactive", ... },
];
```

Form values cũng hardcode:
```
{ label: "Platform Name", value: "OSV Platform" }
{ label: "Organization", value: "Company Security" }
{ label: "Support Email", value: "security@company.com" }
{ label: "Timezone", value: "Asia/Ho_Chi_Minh" }
{ label: "SMTP Host", value: "smtp.company.com" }
{ label: "SMTP Port", value: "587" }
```

Security policy values hardcode:
```
{ label: "Minimum Length", type: "number", value: "12" }
{ label: "Max Age (days)", type: "number", value: "90" }
{ label: "Session Timeout (min)", type: "number", value: "60" }
{ label: "Max Concurrent Sessions", type: "number", value: "3" }
```

**Loại:** Mock data + Config data  
**Tác động:** Settings form không load từ DB, thay đổi sẽ mất sau refresh  
**Gợi ý khắc phục:** Load từ `GET /api/admin/settings`, submit về `PUT /api/admin/settings`

---

### 5. `features/findings/components/RiskAcceptanceCenter.tsx`

**Vấn đề:** Toàn bộ danh sách risk acceptance hardcode:

```
const acceptances = [
  { id: "RA-012", finding: "F-2841", title: "Kubernetes API Server Exposure", product: "DevOps Platform",
    reason: "Network controls (VPN-only access) mitigate the risk...", expiration: "Sep 14, 2026",
    owner: "Carol Anderson", status: "approved", severity: "High", daysLeft: 92 },
  ... 5 items
];
```

**Loại:** Mock data  
**Tác động:** Không hiển thị risk acceptance thực từ hệ thống, approve/reject button không có tác dụng  
**Gợi ý khắc phục:** Tạo hook `useRiskAcceptances()` gọi `GET /api/findings/risk-acceptances`

---

### 6. `features/scanning/components/ScanHistory.tsx`

**Vấn đề:** Toàn bộ lịch sử scan hardcode thành `const history = [...]` với 8 bản ghi:

```
const history = [
  { id: "SC-0047", name: "Production Network Sweep", target: "10.0.0.0/16", type: "NMAP",
    duration: "00:24:15", findings: 47, status: "completed", user: "carol@", date: "Jun 14, 09:18 AM" },
  { id: "SC-0046", name: "Dev Environment Scan", target: "192.168.1.0/24", ... },
  ...
];
```

**Loại:** Mock data  
**Tác động:** Scan history không phản ánh thực tế, bộ lọc chỉ lọc trên local data  
**Gợi ý khắc phục:** Gọi `GET /api/scans?status=completed|failed&page=...`

---

### 7. `features/scanning/components/ScanDashboard.tsx`

**Vấn đề:** Dữ liệu biểu đồ hoạt động tuần được tạo bằng `Math.random()`:

```
const weeklyActivity = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"].map((day) => ({
  day,
  scans: Math.floor(Math.random() * 10) + 1,        // ← RANDOM
  findings: Math.floor(Math.random() * 80) + 10,    // ← RANDOM
}));
```

Quick Actions hardcode:
```
{ label: "New NMAP Scan", color: "#4F8CFF", desc: "Network discovery & port scan" },
{ label: "New ZAP Scan", color: "#F97316", desc: "Web application security test" },
{ label: "View Scan History", color: "#A78BFA", desc: "Browse all past scans" },
```

**Loại:** Mock data (random) + Static UI config  
**Tác động:** Biểu đồ thay đổi mỗi lần render, không phản ánh dữ liệu thực  
**Gợi ý khắc phục:** Lấy weekly activity từ API `GET /api/scans/stats/weekly`

---

### 8. `features/ai-center/components/AITriage.tsx`

**Vấn đề:** Toàn bộ hàng đợi triage AI hardcode thành `const queue = [...]` với 6 items:

```
const queue = [
  { id: "AT-001", finding: "F-2847", title: "Apache Log4j2 JNDI RCE",
    verdict: "Patch Immediately", confidence: 98, severity: "Critical",
    reasoning: "CVSS 10.0, EPSS 98.2%, active CISA KEV...",
    fixes: ["Upgrade Log4j2 to 2.17.1+", ...] },
  ...
];
```

Metric hardcode trong header:
```
{ label: "Accepted Today", value: 8, color: "#10B981" },    // ← số 8 hardcode
{ label: "Avg Confidence", value: "87%", color: "#A78BFA" }, // ← "87%" hardcode
```

**Loại:** Mock data + Hardcode metric  
**Tác động:** AI triage không thực tế, accept/reject button không có tác dụng  
**Gợi ý khắc phục:** Gọi `GET /api/ai/triage/queue`, tính avg confidence từ data trả về

---

### 9. `features/integrations/components/APIKeyManagement.tsx`

**Vấn đề:** Toàn bộ danh sách API keys hardcode:

```
const apiKeys = [
  { id: "k-001", name: "CI/CD Pipeline", prefix: "osv_prod_xK7m",
    scope: ["scan:write", "finding:read"], created: "Jun 1, 2026",
    lastUsed: "2 min ago", expires: "Dec 31, 2026", status: "active" },
  ...4 items
];
```

Prefix key được generate phía frontend theo logic random:
```
const key = `osv_prod_${"abcdefghijklmnopqrstuvwxyz0123456789"
  .split("").sort(() => Math.random() - 0.5).slice(0, 24).join("")}`;
```

**Loại:** Mock data + Logic không an toàn (key generation frontend)  
**Tác động:** Danh sách API key không thực tế; tạo key ở frontend không an toàn vì random seed không cryptographic  
**Gợi ý khắc phục:** Gọi `GET /api/integrations/api-keys`; key generation phải ở backend `POST /api/integrations/api-keys`

---

### 10. `features/integrations/components/WebhookEvents.tsx`

**Vấn đề:** Delivery history và activity chart hardcode:

```
const DELIVERY_HISTORY = [
  { id: "DEL-0441", event: "finding.created", endpoint: "siem.company.com",
    status: "success", responseTime: 124, time: "2 min ago", statusCode: 200 },
  ...4 items
];

const ACTIVITY_CHART = [
  { h: "06:00", success: 42, failed: 1 },
  { h: "09:00", success: 87, failed: 2 },
  ...
];
```

**Loại:** Mock data  
**Tác động:** Delivery log không phản ánh thực tế, retry button vô hiệu  
**Gợi ý khắc phục:** Gọi `GET /api/webhooks/deliveries`, `GET /api/webhooks/stats/hourly`

---

### 11. `features/reports/components/ReportCenter.tsx`

**Vấn đề:** Danh sách báo cáo và templates hardcode:

```
const reports = [
  { id: "R-047", name: "Q2 2026 Executive Summary", type: "Executive",
    created: "Jun 14, 09:00 AM", status: "ready", size: "2.4 MB" },
  ...5 items
];
```

Subtitle hardcode:
```
<p>...· Last report 6h ago</p>   // ← "6h ago" hardcode
```

Filter options hardcode (không load từ API):
```
{ label: "Product", options: ["All Products", "Banking App", "Mobile App", "API Gateway"] }
```

**Loại:** Mock data + Static product list  
**Tác động:** Danh sách báo cáo không thực tế, filter product không theo dữ liệu thực  
**Gợi ý khắc phục:** `GET /api/reports`, `GET /api/products` để populate filter

---

### 12. `features/notifications/components/NotificationCenter.tsx`

**Vấn đề:** Toàn bộ danh sách notifications hardcode:

```
const notifications = [
  { id: 1, type: "critical", title: "Critical Finding Detected",
    desc: "CVE-2025-44228 found on webserver01.prod — CVSS 10.0, KEV active",
    time: "10 min ago", read: false, product: "Banking App" },
  ...10 items
];
```

**Loại:** Mock data  
**Tác động:** Notifications không thực tế, "Mark all read" button vô hiệu  
**Gợi ý khắc phục:** Gọi `GET /api/notifications?unread=true`, kết hợp WebSocket để nhận real-time

---

### 13. `features/auth/components/LoginScreen.tsx`

**Vấn đề:** Các số liệu thống kê và threat indicators trên trang login hardcode:

```
const stats = [
  { label: "CVEs Tracked", value: "240K+" },
  { label: "Scans Today", value: "1,847" },
  { label: "Findings", value: "98.4%" },
  { label: "Uptime SLA", value: "99.99%" },
];

const threatIndicators = [
  { label: "Critical Threats", count: 14, color: "#EF4444" },
  { label: "KEV Active", count: 7, color: "#F59E0B" },
  { label: "Assets At Risk", count: 23, color: "#3B82F6" },
];
```

Metadata tĩnh ở footer:
```
<p>OSV Platform v3.2.1 · © 2026 OSV Security Inc.</p>  // ← version hardcode
```

**Loại:** Mock data + Static metadata  
**Tác động:** Threat count không thực tế, version sẽ lỗi thời khi deploy  
**Gợi ý khắc phục:** Load public stats từ `GET /api/public/stats` (không cần auth), inject version từ build env var

---

### 14. `features/product-security/components/ProductSecurity.tsx`

**Vấn đề:** State khởi tạo mặc định hardcode product ID và type ID:

```
const [expandedTypes, setExpandedTypes] = useState<string[]>(["pt-1"]);   // ← hardcode "pt-1"
const [selectedProductId, setSelectedProductId] = useState<string>("p-1"); // ← hardcode "p-1"
```

Text placeholder hardcode:
```
<div>No risk acceptances pending review</div>
```

**Loại:** UI config hardcode + Missing API integration  
**Tác động:** Nếu ID sản phẩm đầu tiên không phải "p-1" thì UI sẽ không chọn đúng sản phẩm  
**Gợi ý khắc phục:** Mặc định chọn item đầu tiên từ API response

---

## Thống kê theo mức độ ưu tiên

### 🔴 Ưu tiên Cao (ảnh hưởng đến tính năng chính)

| # | File | Vấn đề |
|---|---|---|
| 1 | `UserManagement.tsx` | Danh sách user hardcode — không lấy từ API |
| 2 | `AuditLogs.tsx` | Audit trail hardcode — immutable log không đúng |
| 3 | `RBACManagement.tsx` | RBAC matrix hardcode — không thể cập nhật |
| 4 | `RiskAcceptanceCenter.tsx` | Risk acceptance hardcode — approve/reject vô hiệu |
| 5 | `APIKeyManagement.tsx` | API key list hardcode + key generation frontend (không an toàn) |
| 6 | `AITriage.tsx` | AI triage queue hardcode — accept/reject vô hiệu |
| 7 | `NotificationCenter.tsx` | Notifications hardcode — không real-time |

### 🟡 Ưu tiên Trung bình (ảnh hưởng đến chất lượng hiển thị)

| # | File | Vấn đề |
|---|---|---|
| 8 | `ScanHistory.tsx` | Lịch sử scan hardcode |
| 9 | `ScanDashboard.tsx` | Weekly chart dùng `Math.random()` |
| 10 | `WebhookEvents.tsx` | Delivery history + chart hardcode |
| 11 | `ReportCenter.tsx` | Danh sách report + product filter hardcode |
| 12 | `SystemSettings.tsx` | Form values + AI providers hardcode |

### 🟢 Ưu tiên Thấp (cosmetic / metadata)

| # | File | Vấn đề |
|---|---|---|
| 13 | `LoginScreen.tsx` | Stats + threat count hardcode; version string static |
| 14 | `ProductSecurity.tsx` | Default selection hardcode với ID tĩnh |

---

## Các pattern hardcode màu sắc lặp lại

Nhiều file dùng trực tiếp hex color thay vì CSS variable hoặc design token. Cần tập trung vào:

```typescript
// Pattern phổ biến cần chuyển thành design token
"#EF4444" → var(--color-severity-critical)
"#F97316" → var(--color-severity-high)
"#EAB308" → var(--color-severity-medium)
"#3B82F6" → var(--color-severity-low)
"#10B981" → var(--color-status-success)
"#F59E0B" → var(--color-status-warning)
"#6B7280" → var(--color-text-muted)
"#4F8CFF" → var(--color-primary)
"#0B1020" → var(--color-bg-page)
"#151B2F" → var(--color-bg-card)
"#0F1629" → var(--color-bg-sidebar)
```

**File bị ảnh hưởng:** Tất cả 14 file trên đều dùng inline hex color thay vì token.

---

## Ngưỡng số (threshold) hardcode cần chuyển ra config

| File | Ngưỡng | Giá trị | Gợi ý |
|---|---|---|---|
| `SLADashboard.tsx` | SLA compliance (bar chart color) | `>= 95` green, `>= 90` yellow, else red | `SLA_THRESHOLD_GREEN`, `SLA_THRESHOLD_YELLOW` |
| `AIEnrichment.tsx` | AI confidence threshold | `> 90` green, else yellow | `AI_CONFIDENCE_HIGH_THRESHOLD` |
| `ScanHistory.tsx` | Finding count severity | `> 20` red, `> 0` yellow | `FINDING_COUNT_HIGH`, `FINDING_COUNT_LOW` |
| `SLADashboard.tsx` | Days left warning | `<= 1` critical, else warning | `SLA_DAYS_CRITICAL` |
| `RiskAcceptanceCenter.tsx` | Days left expired | `< 0` overdue, `< 30` warning | `RISK_DAYS_WARNING` |
| `WebhookEvents.tsx` | Response time slow | `> 1000ms` | `WEBHOOK_SLOW_MS` |
| `AdminSettings.tsx` | Session timeout | `60` min | Config từ backend |
| `AdminSettings.tsx` | Password min length | `12` chars | Config từ backend |

---

## Khuyến nghị tổng quát

1. **Ưu tiên xóa mock data** trong các tính năng CRUD (UserManagement, AuditLogs, RiskAcceptanceCenter) vì chúng tạo ảo giác hoạt động nhưng thực ra mọi thao tác đều vô hiệu.

2. **Chuyển key generation về backend** — hiện tại `APIKeyManagement.tsx` generate API key ở frontend là lỗ hổng bảo mật vì `Math.random()` không phải CSPRNG.

3. **Tạo file constants** `src/shared/constants/thresholds.ts` để tập trung các ngưỡng số.

4. **Tạo design token** trong `src/shared/styles/tokens.css` cho tất cả màu sắc lặp lại.

5. **Thêm `staleTime`** cho các hook query mới tương tự pattern đã có trong `SLADashboard.tsx`, `SystemHealth.tsx`, `AIEnrichment.tsx`.
