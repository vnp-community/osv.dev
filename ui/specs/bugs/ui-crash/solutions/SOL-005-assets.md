# SOL-005 — Asset Inventory: `TypeError: Cannot read properties of null (reading 'filter')`

**Bug file**: [BUG-assets.md](../BUG-assets.md)  
**Route**: `/assets`  
**Component**: [`AssetInventory.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/assets/components/AssetInventory.tsx)  
**Priority**: 🔴 P0

---

## Root Cause

Khi backend trả về `{ assets: null }` (thay vì `[]` cho empty collection),  
`QueryBoundary` pass `null` vào children, component gọi `data.assets.filter(...)` → crash.

Stack trace từ browser:
```
TypeError: Cannot read properties of null (reading 'filter')
  at children (AssetInventory-*.js:1:...)
  at f (QueryBoundary-D1KgYtbP.js:1:2415)
```

Vấn đề nằm ở `QueryBoundary` — nó không check `null` array fields bên trong `data`, chỉ check `data` bản thân.

---

## Fix

### Fix 1 — Normalize trong hook `useAssets` (Recommended)

```typescript
// ui/src/features/assets/hooks/useAssets.ts (hoặc tương đương)
export function useAssets(params?: AssetsParams) {
  return useQuery<AssetsResponse>({
    queryKey: ['assets', params],
    queryFn: async () => {
      const { data } = await apiClient.get<AssetsResponse>(ENDPOINTS.assets.list, { params });
      return {
        assets: Array.isArray(data?.assets) ? data.assets : [],
        total:  typeof data?.total === 'number' ? data.total : 0,
        page:   data?.page ?? 1,
        page_size: data?.page_size ?? 20,
      };
    },
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });
}
```

### Fix 2 — Guard trong component

```typescript
// AssetInventory.tsx — bên trong QueryBoundary children
{(data) => {
  const assets = Array.isArray(data?.assets) ? data.assets : [];
  
  const filtered = assets.filter((a) => {
    // ...filter logic
  });
  // ...
}}
```

---

## Pattern tham chiếu (Architecture Section 5.5.3)

```typescript
// Chuẩn theo architecture.md — component template
export function AssetInventory() {
  const { data, isLoading, isError, refetch } = useAssets(params);

  if (isLoading) return <AssetInventorySkeleton />;
  if (isError)   return <ErrorState message={error.message} onRetry={refetch} />;
  if (!data?.assets?.length) return <EmptyState title="No assets found" />;

  return <AssetTable assets={data.assets} />;
}
```

---

## Files cần sửa

| File | Thay đổi |
|------|---------|
| `features/assets/hooks/useAssets.ts` | Normalize `assets: null` → `[]` trong queryFn |
| [`AssetInventory.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/assets/components/AssetInventory.tsx) | Thêm `Array.isArray` guard trước `.filter()` |
