# TASK-P4-01 — Fix `ScanHistory.tsx` → Dùng `useScans()` sẵn có

**Phase:** 4 — Scan & Report  
**Nguồn giải pháp:** [`solutions/06_07_scan_history_dashboard.md` — Solution 06](../solutions/06_07_scan_history_dashboard.md)  
**Ưu tiên:** 🔵 Scan  
**Phụ thuộc:** TASK-P1-03

---

## Vấn đề hiện tại

```typescript
// ❌ HIỆN TẠI — features/scanning/components/ScanHistory.tsx
const history = [
  { id: 'SC-0039', name: 'Production Network Sweep', status: 'completed', ... },
  // 8 scan records hardcode
];
```

> **Đặc biệt:** Hook `useScans` đã tồn tại trong `features/scanning/hooks/useScans.ts` nhưng chưa được dùng trong `ScanHistory.tsx`!

---

## API Endpoint

```
GET /api/v1/scans?status=completed,failed&page=1&pageSize=20
```
> ENDPOINTS.scans.list đã có sẵn trong endpoints.ts

---

## Danh sách files cần tạo/sửa

### [MODIFY] `src/features/scanning/components/ScanHistory.tsx`

**Thay đổi chính — rất đơn giản:**
- Xóa `const history = [...]`
- Import `useScans` từ `../hooks/useScans` (đã có sẵn)
- Dùng hook với params đúng:

```typescript
const scansQuery = useScans({
  status: 'completed,failed,cancelled',
  search: search || undefined,
  type: typeFilter !== 'all' ? typeFilter : undefined,
  page,
  pageSize: 20,
});
```

Xem code component đầy đủ tại: [`solutions/06_07_scan_history_dashboard.md`](../solutions/06_07_scan_history_dashboard.md) — mục "Component sau khi fix"

---

## Tiêu chí hoàn thành

- [x] `ScanHistory.tsx` không còn `const history = [...]`
- [x] Dùng `useScans({ status: 'completed,failed,cancelled', ... })`
- [x] Search input gửi `search` param, type filter gửi `type` param
- [x] Pagination (page state, Prev/Next buttons, hiển thị Page X of Y)
- [x] formatDate() từ ISO startedAt/completedAt (không hardcode)
- [x] formatDuration() tính từ startedAt + completedAt ISO
- [x] navigator.clipboard.writeText(s.id) cho Copy button
- [x] Skeleton loading
- [x] TypeScript 0 lỗi mới

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã sửa:**
- [`features/scanning/components/ScanHistory.tsx`](../../../../ui/src/features/scanning/components/ScanHistory.tsx) — [MODIFY] Refactored to `useScans()`

---

## Kiểm tra

```bash
# Mở http://localhost:3000 → Scan History
# 1. Verify: scans hiển thị từ hook (không hardcode)
# 2. Search "production" → filter scans chứa "production"
# 3. Filter "ZAP" → chỉ hiện ZAP scans
# 4. "View →" trên completed scan → navigate đúng URL
```
