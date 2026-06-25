# SOL-006 — Product Security: `TypeError: Cannot read properties of undefined (reading 'flatMap')`

**Bug file**: [BUG-products.md](../BUG-products.md)  
**Route**: `/products`  
**Component**: [`ProductSecurity.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/product-security/components/ProductSecurity.tsx#L95)  
**Priority**: 🟠 P1

---

## Root Cause

```typescript
// ProductSecurity.tsx:95 — crash point
const allProducts = productTypes.flatMap(pt => pt.products);
```

`QueryBoundary` destructure `{ productTypes, riskTrend }` từ `data`.  
Nếu backend trả về `{ riskTrend: [...] }` mà **thiếu field `productTypes`**, thì `productTypes` = `undefined` → `.flatMap()` crash.

Server response hiện tại trả về shape không khớp với `ProductsResponse`:
```typescript
interface ProductsResponse {
  productTypes: ProductType[];  // ← field này bị undefined khi thiếu
  riskTrend: RiskTrendPoint[];
}
```

---

## Fix

### Fix 1 — Normalize trong `useProducts` hook (Recommended)

```typescript
// ProductSecurity.tsx (hoặc tách ra hooks/useProducts.ts)
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

### Fix 2 — Defensive in component children

```typescript
// ProductSecurity.tsx bên trong QueryBoundary
{({ productTypes: rawProductTypes, riskTrend }) => {
  // Guard: đảm bảo array
  const productTypes = Array.isArray(rawProductTypes) ? rawProductTypes : [];
  const allProducts = productTypes.flatMap(pt => pt.products ?? []);
  // ...
}}
```

---

## Files cần sửa

| File | Thay đổi |
|------|---------|
| [`ProductSecurity.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/product-security/components/ProductSecurity.tsx) | Normalize trong `useProducts` queryFn + optional guard |
