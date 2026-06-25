# TASK-01 — Thêm điều hướng cho 9 mục cha

**Priority:** 🟡 Medium  
**Effort:** ~15 phút  
**Loại thay đổi:** Config-only (không tạo file mới)  
**Trạng thái:** ✅ DONE — 2026-06-22

---

## Mục tiêu

9 mục cha của left menu hiện tại không có path điều hướng trong `SECTION_TO_PATH`. Khi click vào, sidebar chỉ toggle expand/collapse mà không navigate đến trang nào. Task này thêm các entry còn thiếu để click vào mục cha sẽ navigate về trang mặc định của nhóm đó.

---

## File cần sửa

- [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx)

---

## Thay đổi chi tiết

### Sửa object `SECTION_TO_PATH` (dòng 12–59)

Thêm 9 entries sau vào cuối object `SECTION_TO_PATH`, trước dấu `}` đóng:

```typescript
// Thêm vào SECTION_TO_PATH — Parent node navigation defaults
"vuln-intel":       "/cve/search",
"scanning":         "/scans",
"findings":         "/findings",
"assets":           "/assets",
"product-security": "/products",
"ai-center":        "/ai/triage",
"reports":          "/reports",
"integrations":     "/integrations/api-keys",
"admin":            "/admin/users",
```

---

## Acceptance Criteria

- [x] Click vào "Vulnerability Intel" → navigate tới `/cve/search`
- [x] Click vào "Active Scanning" → navigate tới `/scans`
- [x] Click vào "Findings" → navigate tới `/findings`
- [x] Click vào "Assets" → navigate tới `/assets`
- [x] Click vào "Product Security" → navigate tới `/products`
- [x] Click vào "AI Center" → navigate tới `/ai/triage`
- [x] Click vào "Reports" → navigate tới `/reports`
- [x] Click vào "Integrations" → navigate tới `/integrations/api-keys`
- [x] Click vào "Administration" → navigate tới `/admin/users`
- [x] Đồng thời, click vào mục cha vẫn toggle expand/collapse (behavior hiện tại giữ nguyên)
- [x] Active highlight trên mục cha vẫn hoạt động đúng khi ở trang con

---

## Ghi chú kỹ thuật

- Theo `architecture.md` Section 15: `Route definitions` là static config, được phép dùng literal. Không vi phạm API-First rule.
- Logic `handleNavigate` hiện tại (dòng 183–186) đã hỗ trợ: `if (item.children) toggleExpand(item.id); handleNavigate(item.id)` — chỉ cần thêm path vào mapping là đủ.
- Không cần tạo route mới trong `router.tsx` vì tất cả các path này đã tồn tại.
