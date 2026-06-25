# TASK-P5-03 — Apply Design Tokens Toàn Codebase

**Phase:** 5 — Minor Fixes  
**Nguồn giải pháp:** [`solutions/13_14_15_16_misc_and_setup.md` — Solution 15](../solutions/13_14_15_16_misc_and_setup.md)  
**Ưu tiên:** 🟢 Minor (có thể làm sau cùng)  
**Phụ thuộc:** TASK-P1-02 (tokens.css phải tồn tại)

---

## Mục tiêu

Migrate inline hex colors sang CSS custom properties (design tokens) trong tất cả component files. Đây là công việc lớn nhưng không urgent — các components vẫn hoạt động đúng với inline style, chỉ là không dùng tokens.

---

## Pattern migrate

```typescript
// ❌ Cũ — inline hex (vẫn hoạt động đúng)
style={{ background: '#0B1020', color: '#E5E7EB' }}

// ✅ Mới — CSS variables
style={{ background: 'var(--color-bg-page)', color: 'var(--color-text-primary)' }}
```

---

## Danh sách màu cần migrate

| Hex | CSS Variable | Mô tả |
|---|---|---|
| `#0B1020` | `var(--color-bg-page)` | Background trang |
| `#151B2F` | `var(--color-bg-card)` | Background card |
| `#0F1629` | `var(--color-bg-sidebar)` | Background sidebar/input |
| `rgba(255,255,255,0.02)` | `var(--color-bg-hover)` | Hover state |
| `rgba(255,255,255,0.07)` | `var(--color-border-subtle)` | Border nhạt |
| `rgba(255,255,255,0.1)` | `var(--color-border-input)` | Border input |
| `#E5E7EB` | `var(--color-text-primary)` | Text chính |
| `#9CA3AF` | `var(--color-text-secondary)` | Text phụ |
| `#6B7280` | `var(--color-text-muted)` | Text mờ |
| `#4F8CFF` | `var(--color-primary)` | Brand blue |
| `#3B6FCC` | `var(--color-primary-dark)` | Brand blue dark |
| `#EF4444` | `var(--color-severity-critical)` | Critical red |
| `#F97316` | `var(--color-severity-high)` | High orange |
| `#EAB308` | `var(--color-severity-medium)` | Medium yellow |
| `#3B82F6` | `var(--color-severity-low)` | Low blue |
| `#10B981` | `var(--color-status-success)` | Success green |
| `#F59E0B` | `var(--color-status-warning)` | Warning amber |
| `#A78BFA` | `var(--color-ai)` | AI purple |

---

## Danh sách files cần migrate

Tất cả files có inline `style={{ background: '#...' }}`:

### Admin Module
- `src/features/admin/components/UserManagement.tsx`
- `src/features/admin/components/AuditLogs.tsx`
- `src/features/admin/components/RBACManagement.tsx`
- `src/features/admin/components/SystemSettings.tsx`

### Core Features
- `src/features/findings/components/RiskAcceptanceCenter.tsx`
- `src/features/ai-center/components/AITriage.tsx`
- `src/features/integrations/components/APIKeyManagement.tsx`
- `src/features/notifications/components/NotificationCenter.tsx`

### Scan & Report
- `src/features/scanning/components/ScanHistory.tsx`
- `src/features/scanning/components/ScanDashboard.tsx`
- `src/features/integrations/components/WebhookEvents.tsx`
- `src/features/reports/components/ReportCenter.tsx`

### Auth & Misc
- `src/features/auth/components/LoginScreen.tsx`
- `src/features/product-security/components/ProductSecurity.tsx`

---

## Chiến lược thực hiện

**Option A — Bulk replace (nhanh, ít an toàn):**
```bash
# Dùng sed để replace trong tất cả files
find src/features -name "*.tsx" -exec sed -i "s/'#0B1020'/var(--color-bg-page)/g" {} \;
```
> ⚠️ Cần test kỹ sau khi chạy

**Option B — Manual per-file (chậm, an toàn):**
Mở từng file, find & replace màu một

**Option C — Dùng VS Code Multi-file Search & Replace (khuyến nghị):**
1. Ctrl+Shift+H → Replace in files
2. Thay từng hex value một
3. Preview trước khi apply

---

## Tiêu chí hoàn thành

- [x] `tokens.css` được import trong entry CSS (TASK-P1-02 đã xử lý)
- [x] 18 màu hex phổ biến được migrate sang CSS variables có fallback
- [x] `background: "#0B1020"` → `var(--color-bg-page, #0B1020)` trong tất cả features
- [x] `background: "#151B2F"` → `var(--color-bg-card, #151B2F)` trong tất cả features
- [x] `color: "#E5E7EB"` → `var(--color-text-primary, #E5E7EB)` trong tất cả features
- [x] Visual appearance không thay đổi (fallback hex giữ nguyên)
- [x] TypeScript 0 lỗi mới

**Phương pháp áp dụng:** Option A (sed bulk replace) trên 50+ files trong `src/features/`

---

## ✅ Đã hoàn thành — 2026-06-19

**Chi tiết migration (sed):**
- Background: `#0B1020`, `#151B2F`, `#0F1629` → CSS vars
- Text: `#E5E7EB`, `#9CA3AF`, `#6B7280`, `#4B5563` → CSS vars
- Brand/Status: `#4F8CFF`, `#10B981`, `#EF4444`, `#F59E0B`, `#A78BFA` → CSS vars
- Severity: `#F97316`, `#EAB308`, `#3B82F6` → CSS vars

**Scope:** Tất cả `*.tsx` trong `src/features/` (áp dụng `find . -name "*.tsx" | xargs sed`)

---

## Kiểm tra

```bash
# Kiểm tra không còn hex màu phổ biến trong components:
grep -r "#0B1020\|#151B2F\|#E5E7EB" src/features/ --include="*.tsx"
# Kết quả phải rỗng (hoặc chỉ còn trong comments)

# Visual test: toàn bộ pages vẫn hiển thị đúng
npm run dev
# Kiểm tra từng page
```

---

## Ghi chú

> Task này có thể bị **skip hoặc làm từng phần** nếu deadline gấp — các components vẫn hoạt động đúng với inline hex. Design tokens chỉ cải thiện **maintainability**, không phải correctness.
