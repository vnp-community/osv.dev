# TASK-003 — Normalize `useProducts` queryFn để fix `undefined.flatMap`

**Bug**: [BUG-products.md](../BUG-products.md)  
**Solution**: [SOL-006-products.md](../solutions/SOL-006-products.md)  
**Priority**: 🟠 P1  
**Effort**: ~5 phút  
**Status**: `[x] DONE`

---

## Mô tả

`useProducts` trong `ProductSecurity.tsx` không normalize response. Khi `productTypes` là `undefined`, code `productTypes.flatMap(pt => pt.products)` crash.

---

## File cần sửa

**File**: [`ui/src/features/product-security/components/ProductSecurity.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/product-security/components/ProductSecurity.tsx)

---

## Thay đổi chính xác

**Tìm đoạn code** `useProducts` (khoảng line 29–38):

```typescript
function useProducts() {
  return useQuery<ProductsResponse>({
    queryKey: ['products'],
    queryFn: async () => {
      const { data } = await apiClient.get<ProductsResponse>(ENDPOINTS.products.list);
      return data;
    },
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });
}
```

**Thay bằng**:

```typescript
function useProducts() {
  return useQuery<ProductsResponse>({
    queryKey: ['products'],
    queryFn: async () => {
      const { data } = await apiClient.get<ProductsResponse>(ENDPOINTS.products.list);
      // Normalize — đảm bảo productTypes luôn là array
      return {
        productTypes: Array.isArray(data?.productTypes) ? data.productTypes : [],
        riskTrend:    Array.isArray(data?.riskTrend)    ? data.riskTrend    : [],
      };
    },
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });
}
```

---

## Acceptance Criteria

- [ ] `useProducts` trả về `{ productTypes: [], riskTrend: [] }` khi backend thiếu fields
- [ ] Trang `/products` không còn `TypeError: Cannot read properties of undefined (reading 'flatMap')`
- [ ] TypeScript không có lỗi mới

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/ui
npx tsc --noEmit 2>&1 | grep ProductSecurity
```
