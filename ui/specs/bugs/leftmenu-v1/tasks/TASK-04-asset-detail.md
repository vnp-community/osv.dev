# TASK-04 — Fix "Asset Detail" trỏ sai trang

**Priority:** 🔴 High  
**Effort:** ~10 phút  
**Loại thay đổi:** Config + navItems update (xóa 1 item)  
**Trạng thái:** ✅ DONE — 2026-06-22

> **Ghi chú thực thi:** Đã xóa `{ id: "asset-detail" }` khỏi `navItems`. Đã giữ lại `"asset-detail"` trong `SECTION_CHILDREN.assets` để sidebar vẫn highlight "Assets" khi user đang ở `/assets/:id`.

---

## Mục tiêu

Item "Asset Detail" (`id: asset-detail`) trong menu "Assets" đang được cấu hình trỏ về `/assets` — cùng URL với "Asset Inventory". Điều này gây ra 2 vấn đề:
1. Click vào "Asset Detail" chuyển người dùng về trang **danh sách** thay vì trang **chi tiết**
2. Hai nút cùng trỏ về 1 URL → trải nghiệm như nút bị liệt

Trang chi tiết tài sản (`/assets/:id`) yêu cầu một `assetId` cụ thể từ context (click vào một asset trong danh sách). Vì vậy, "Asset Detail" không phải là entry point phù hợp từ sidebar.

**Giải pháp:** Xóa item "Asset Detail" khỏi `navItems`.

---

## File cần sửa

- [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx)

---

## Thay đổi chi tiết

### Sửa `navItems` — xóa item "Asset Detail" (dòng 116–122)

**Trước:**
```typescript
{
  id: "assets", label: "Assets", icon: Server,
  children: [
    { id: "asset-inventory", label: "Asset Inventory" },
    { id: "asset-detail", label: "Asset Detail" },
  ],
},
```

**Sau:**
```typescript
{
  id: "assets", label: "Assets", icon: Server,
  children: [
    { id: "asset-inventory", label: "Asset Inventory" },
  ],
},
```

### Sửa `SECTION_TO_PATH` — xóa entry `asset-detail` (dòng 37)

**Trước:**
```typescript
"asset-inventory": "/assets",
"asset-detail":    "/assets",
```

**Sau:**
```typescript
"asset-inventory": "/assets",
// Xóa: "asset-detail": "/assets",
```

---

## Acceptance Criteria

- [x] Menu "Assets" chỉ còn 1 item con: "Asset Inventory"
- [x] Click "Asset Inventory" → navigate tới `/assets`
- [x] Trang `/assets/:id` (Asset Detail) vẫn hoạt động bình thường khi truy cập từ danh sách (click vào row)
- [x] Sidebar không hiển thị item "Asset Detail" nữa
- [x] Active highlight trên "Asset Inventory" đúng khi đang ở `/assets`
- [x] Active highlight trên "Assets" (mục cha) đúng khi đang ở `/assets/:id`

---

## Ghi chú kỹ thuật

- `SECTION_CHILDREN.assets` (dòng 171) hiện tại là `["asset-inventory", "asset-detail"]`. Sau khi xóa item, cập nhật thành:
  ```typescript
  assets: ["asset-inventory"],
  ```
  Hoặc giữ nguyên `"asset-detail"` trong `SECTION_CHILDREN` nếu muốn mục cha "Assets" vẫn được highlight active khi user đang ở trang `/assets/:id` (trường hợp user navigate trực tiếp bằng URL).

- **Khuyến nghị:** Giữ `"asset-detail"` trong `SECTION_CHILDREN.assets` để đảm bảo "Assets" header được highlight khi đang xem detail page.
