# TASK-07 — Dọn dẹp Orphan IDs dư thừa trong Sidebar

**Priority:** 🟢 Low  
**Effort:** ~15 phút  
**Loại thay đổi:** Config-only cleanup  
**Trạng thái:** ✅ DONE — 2026-06-22

> **Ghi chú thực thi:** Đã xóa `false-positive`, `running-scan-detail` khỏi `SECTION_CHILDREN`. Đã xóa `security-settings` khỏi cả `SECTION_TO_PATH` (session trước) và `SECTION_CHILDREN.admin`. Đã xóa `finding-detail` và `asset-detail` khỏi `SECTION_TO_PATH` (session trước). Giữ lại các IDs trong `SECTION_CHILDREN` để highlight mục cha đúng.

---

## Mục tiêu

Một số IDs tồn tại trong các object config của Sidebar (`SECTION_CHILDREN`, `SECTION_TO_PATH`) nhưng không khớp với bất kỳ item nào trong `navItems`. Các IDs mồ côi này không ảnh hưởng đến UI trực tiếp nhưng gây:
- Logic highlight active state bị sai (ví dụ: `finding-detail` duplicate với nhiều paths)
- Code khó maintain và dễ gây nhầm lẫn cho developer

---

## File cần sửa

- [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx)

---

## Thay đổi chi tiết

### 7a. Xóa `false-positive` khỏi `SECTION_CHILDREN`

**Dòng 169** — Sửa `findings` trong `SECTION_CHILDREN`:

```typescript
// Trước:
findings: ["finding-detail", "all-findings", "active-findings", "mitigated", "false-positive", "sla-breaches", "risk-acceptance"],

// Sau:
findings: ["finding-detail", "all-findings", "active-findings", "mitigated", "sla-breaches", "risk-acceptance"],
```

> `false-positive` không có trong `navItems` và không có route. Nếu feature này được thêm sau, thêm lại vào đây.

---

### 7b. Xóa `running-scan-detail` khỏi `SECTION_CHILDREN`

**Dòng 170** — Sửa `scanning` trong `SECTION_CHILDREN`:

```typescript
// Trước:
scanning: ["scan-dashboard", "new-scan", "running-scans", "running-scan-detail", "scan-history", "nmap-results", "zap-results"],

// Sau:
scanning: ["scan-dashboard", "new-scan", "running-scans", "scan-history", "nmap-results", "zap-results"],
```

> Route `/scans/:id` (chi tiết scan đang chạy) sẽ tự động highlight "Active Scanning" mục cha thông qua `SECTION_CHILDREN` match. Không cần ID riêng.

---

### 7c. Xóa `finding-detail` khỏi `SECTION_TO_PATH`

**Dòng 33** — Xóa entry sau khỏi `SECTION_TO_PATH`:

```typescript
// Xóa dòng này:
"finding-detail": "/findings",
```

> **Lý do:** ID này không có item tương ứng trong `navItems`. Nó gây nhiễu cho hàm `pathToSection` — khi user đang ở `/findings/:id`, hàm sẽ match `finding-detail` (path `/findings`) thay vì `all-findings` (cũng path `/findings`). Kết quả highlight active state bị non-deterministic. Xóa đi để `pathToSection` chỉ match `all-findings` (tức là item con duy nhất map đến `/findings`).

---

### 7d. Xóa `security-settings` khỏi `SECTION_TO_PATH`

**Dòng 56** — Xóa entry sau khỏi `SECTION_TO_PATH`:

```typescript
// Xóa dòng này:
"security-settings": "/admin/settings",
```

> **Lý do:** Đây là duplicate của `system-settings: "/admin/settings"`. Không có menu item `security-settings` nào trong `navItems`. Giữ lại gây nhầm lẫn khi đọc code.

---

### 7e. Dọn dẹp `SECTION_CHILDREN.admin`

Xóa `security-settings` không có trong `navItems`:

```typescript
// Trước:
admin: ["users", "roles", "audit-logs", "system-health", "system-settings", "security-settings"],

// Sau:
admin: ["users", "roles", "audit-logs", "system-health", "system-settings"],
```

---

## Tổng kết thay đổi

Sau TASK-07, `Sidebar.tsx` sẽ sạch hơn như sau:

```typescript
const SECTION_TO_PATH: Record<string, string> = {
  // ... (xóa "finding-detail" và "security-settings")
};

const SECTION_CHILDREN: Record<string, string[]> = {
  findings: ["finding-detail", "all-findings", "active-findings", "mitigated", "sla-breaches", "risk-acceptance"],
  // ^ "false-positive" đã xóa

  scanning: ["scan-dashboard", "new-scan", "running-scans", "scan-history", "nmap-results", "zap-results"],
  // ^ "running-scan-detail" đã xóa

  assets: ["asset-inventory", "asset-detail"],     // giữ "asset-detail" để highlight khi ở /assets/:id
  "ai-center": ["ai-triage", "ai-enrichment", "ai-insights"],
  "vuln-intel": ["cve-search", "semantic-search", "vendor-catalog", "kev-catalog", "epss-analytics", "cwe-catalog", "capec-catalog"],
  admin: ["users", "roles", "audit-logs", "system-health", "system-settings"],
  // ^ "security-settings" đã xóa
};
```

---

## Acceptance Criteria

- [x] Sidebar.tsx không còn ID `false-positive` trong `SECTION_CHILDREN`
- [x] Sidebar.tsx không còn ID `running-scan-detail` trong `SECTION_CHILDREN`
- [x] Sidebar.tsx không còn entry `finding-detail` trong `SECTION_TO_PATH`
- [x] Sidebar.tsx không còn entry `security-settings` trong `SECTION_TO_PATH`
- [x] Sidebar.tsx không còn ID `security-settings` trong `SECTION_CHILDREN.admin`
- [x] Active highlight trên "All Findings" hoạt động đúng khi ở `/findings` và `/findings/:id`
- [x] Active highlight trên "Administration" hoạt động đúng khi ở `/admin/settings`
- [x] Không có regressions trên bất kỳ menu item nào còn lại

---

## Ghi chú kỹ thuật

- Đây là task cleanup thuần túy, không ảnh hưởng UX hiển thị. Có thể thực hiện cuối cùng sau khi các TASK 01-06 đã được verify.
- Nếu sau này feature `false-positive` hoặc `running-scan-detail` được phát triển, cần thêm lại ID tương ứng vào đúng config (cả `navItems` + `SECTION_TO_PATH` + `SECTION_CHILDREN`).
