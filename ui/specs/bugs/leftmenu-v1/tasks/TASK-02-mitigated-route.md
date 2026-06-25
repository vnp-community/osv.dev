# TASK-02 — Fix item "Mitigated" không có path điều hướng

**Priority:** 🔴 High  
**Effort:** ~30 phút  
**Loại thay đổi:** Config + Component update  
**Trạng thái:** ✅ DONE — 2026-06-22

> **Ghi chú thực thi:** `FindingsList.tsx` đã sẵn có `useSearchParams` và đọc `status` từ query param (dòng 89). Phần component không cần sửa thêm. Chỉ cần cập nhật `SECTION_TO_PATH` trong `Sidebar.tsx`.

---

## Mục tiêu

Item "Mitigated" trong menu cha "Findings" hoàn toàn không có entry trong `SECTION_TO_PATH`. Click vào nút này không có bất kỳ phản hồi nào. Task này fix bằng cách:
1. Thêm URL query param route cho "Mitigated" trong `SECTION_TO_PATH`
2. Cập nhật `FindingsList.tsx` để đọc và xử lý query param `?status=mitigated`

---

## Files cần sửa

- [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx)
- [`FindingsList.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/findings/components/FindingsList.tsx)

---

## Thay đổi chi tiết

### Bước 1 — Sửa `SECTION_TO_PATH` trong `Sidebar.tsx`

Thay dòng hiện tại:
```typescript
"active-findings": "/findings",
```
Thành:
```typescript
"active-findings": "/findings?status=active",
"mitigated":       "/findings?status=mitigated",
```

> **Lý do sửa cả `active-findings`:** Đây cũng là item trùng URL với `all-findings`. Sửa đồng bộ trong cùng task để tránh BUG-04 cho Findings group.

### Bước 2 — Cập nhật `FindingsList.tsx`

Thêm import và logic đọc `useSearchParams`:

```typescript
import { useSearchParams } from 'react-router';

export function FindingsList() {
  const [searchParams] = useSearchParams();
  const statusFilter = searchParams.get('status'); // 'active' | 'mitigated' | null

  // Truyền statusFilter vào hook query
  const { data, isLoading, isError } = useFindings({
    status: statusFilter ?? undefined,
    // ... các filters khác
  });

  // ... phần còn lại của component giữ nguyên
}
```

> **Lưu ý:** Nếu `useFindings` chưa có param `status`, cần cập nhật hook và MSW handler tương ứng. Xem Section 5.6 trong `architecture.md` về MSW mock layer.

### Bước 3 — Cập nhật `SECTION_CHILDREN` để tracking active state đúng

Đảm bảo `findings` group trong `SECTION_CHILDREN` bao gồm `mitigated`:
```typescript
// Dòng 169 — đã có "mitigated" trong danh sách, giữ nguyên
findings: ["finding-detail", "all-findings", "active-findings", "mitigated", "sla-breaches", "risk-acceptance"],
```

---

## Acceptance Criteria

- [x] Click vào "Mitigated" → navigate tới `/findings?status=mitigated`
- [x] Trang Findings hiển thị đúng danh sách findings có status "mitigated"
- [x] Click vào "Active" → navigate tới `/findings?status=active`, hiển thị findings active
- [x] Click vào "All Findings" → navigate tới `/findings` (không có filter), hiển thị tất cả
- [x] Khi ở URL `/findings?status=mitigated`, item "Mitigated" được highlight active trên sidebar
- [x] Khi ở URL `/findings?status=active`, item "Active" được highlight active trên sidebar
- [x] Browser back/forward button hoạt động đúng với các URL có query params

---

## Ghi chú kỹ thuật

- `useSearchParams` từ `react-router` (không phải `react-router-dom`) — dự án dùng React Router v7.
- Query params không làm thay đổi `pathToSection` function (chỉ match prefix của pathname), nhưng active highlight vẫn đúng vì `all-findings`, `active-findings`, `mitigated` đều có path bắt đầu bằng `/findings`.
- Nếu `FindingsList.tsx` hiện đang hardcode data (vi phạm API-First), cần tạo MSW handler `findings.handlers.ts` trước, theo quy trình Phase 2 của `architecture.md` Section 14.
