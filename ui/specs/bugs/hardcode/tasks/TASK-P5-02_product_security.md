# TASK-P5-02 — Fix `ProductSecurity.tsx` → Dynamic Default Selection

**Phase:** 5 — Minor Fixes  
**Nguồn giải pháp:** [`solutions/13_14_15_16_misc_and_setup.md` — Solution 14](../solutions/13_14_15_16_misc_and_setup.md)  
**Ưu tiên:** 🟢 Minor  
**Phụ thuộc:** Không có (chỉ sửa state logic)

---

## Vấn đề hiện tại

```typescript
// ❌ HIỆN TẠI — features/product-security/components/ProductSecurity.tsx
const [expandedTypes, setExpandedTypes] = useState<string[]>(["pt-1"]);
//                                                             ^^^^ hardcode ID
const [selectedProductId, setSelectedProductId] = useState<string>("p-1");
//                                                                   ^^^^ hardcode ID
```

> **Vấn đề:** Nếu product type đầu tiên không có ID `"pt-1"` hoặc product đầu tiên không có ID `"p-1"` (ví dụ khi backend seed data khác), component sẽ không hiển thị gì hoặc hiển thị sai.

---

## Fix — Dynamic default selection

**Đây là fix đơn giản — chỉ thay đổi state initialization logic:**

```typescript
// ✅ SAU KHI FIX
export function ProductSecurity() {
  const productsQuery = useProducts();
  
  // Bắt đầu với state rỗng — sẽ được tính từ server data
  const [expandedTypes, setExpandedTypes] = useState<string[]>([]);
  const [selectedProductId, setSelectedProductId] = useState<string | null>(null);

  return (
    <QueryBoundary query={productsQuery} skeleton={<ProductSkeleton />}>
      {({ productTypes, riskTrend }) => {
        const allProducts = productTypes.flatMap(pt => pt.products);

        // ✅ Dynamic default — lấy item đầu tiên từ server data
        const effectiveExpandedTypes = expandedTypes.length > 0
          ? expandedTypes
          : productTypes.length > 0 ? [productTypes[0].id] : [];

        const effectiveSelectedId = selectedProductId ?? allProducts[0]?.id ?? null;
        const selectedProduct = allProducts.find(p => p.id === effectiveSelectedId) ?? allProducts[0];

        // Dùng effectiveExpandedTypes và effectiveSelectedId thay vì state trực tiếp
        // ...
      }}
    </QueryBoundary>
  );
}
```

---

## Danh sách files cần sửa

### [MODIFY] `src/features/product-security/components/ProductSecurity.tsx`

Chỉ thay đổi:
1. `useState<string[]>(["pt-1"])` → `useState<string[]>([])`
2. `useState<string>("p-1")` → `useState<string | null>(null)`
3. Thêm `effectiveExpandedTypes` và `effectiveSelectedId` computed variables
4. Thay thế mọi references đến `expandedTypes` → `effectiveExpandedTypes` (trong render)
5. Thay thế mọi references đến `selectedProductId` → `effectiveSelectedId` (trong render)

---

## Tiêu chí hoàn thành

- [x] `ProductSecurity.tsx` không còn `useState(["pt-1"])` hoặc `useState("p-1")`
- [x] Default selection tính từ `productTypes[0].id` và `allProducts[0]?.id`
- [x] Component vẫn hoạt động đúng khi user click expand/select
- [x] TypeScript không có lỗi (`string | null` type handling)
- [x] effectiveExpandedTypes và effectiveSelectedId được dùng trong render (chevron icon + active highlight)

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã sửa:**
- [`features/product-security/components/ProductSecurity.tsx`](../../../../ui/src/features/product-security/components/ProductSecurity.tsx) — [MODIFY] Dynamic default selection

---

## Kiểm tra

```bash
# Mở http://localhost:3000 → Product Security
# 1. Default: product type đầu tiên được expand, product đầu tiên được select
# 2. Nếu thay đổi seed data (ID khác "pt-1") → vẫn select đúng item đầu tiên
# 3. Click product khác → select thay đổi bình thường
# 4. Click type header → collapse/expand bình thường
```
