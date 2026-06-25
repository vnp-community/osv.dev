# SOL-001 — CWE Library: `TypeError: Cannot read properties of undefined (reading 'map')`

**Bug file**: [BUG-cve-cwe.md](../BUG-cve-cwe.md)  
**Route**: `/cve/cwe`  
**Component**: [`CWELibrary.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/cve-intel/components/CWELibrary.tsx)  
**Hook**: [`useTaxonomy.ts → useCWELibrary()`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/cve-intel/hooks/useTaxonomy.ts#L58)  
**Priority**: 🟠 P1

---

## Root Cause

`useCWELibrary()` trả về `data` có type `CWEListResponse` nhưng hook không normalize response.  
Khi backend trả về response shape khác với kỳ vọng (vd: `{ data: [...] }` thay vì `{ cweList: [...] }`),  
`data.cweList` sẽ là `undefined`. Component trong `QueryBoundary` destructure `{ cweList }` và gọi `cweList.map()` → crash.

```typescript
// useTaxonomy.ts:62 — THIẾU normalization
queryFn: async () => {
  const { data } = await apiClient.get<CWEListResponse>(ENDPOINTS.cwe.list, { params });
  return data;  // ← không normalize, nếu backend trả shape sai → cweList undefined
},
```

So sánh với `useVendorCatalog` đã có normalize đúng:
```typescript
// Vendor có normalize:
return {
  vendors: Array.isArray(data?.vendors) ? data.vendors : [],
  total:   typeof data?.total === 'number' ? data.total : 0,
};
```

---

## Fix

### Fix 1 — Normalize response trong hook (Recommended — theo pattern của VendorCatalog)

```typescript
// ui/src/features/cve-intel/hooks/useTaxonomy.ts
export function useCWELibrary(params?: { q?: string }) {
  return useQuery<CWEListResponse>({
    queryKey: cveKeys.cwe('list'),
    queryFn: async () => {
      const { data } = await apiClient.get<CWEListResponse>(ENDPOINTS.cwe.list, { params });
      // Normalize — đảm bảo shape đúng bất kể backend trả gì
      return {
        cweList: Array.isArray(data?.cweList) ? data.cweList : [],
        total:   typeof data?.total === 'number' ? data.total : 0,
      };
    },
    staleTime: 10 * 60_000,
    placeholderData: (prev) => prev,
  });
}
```

### Fix 2 — Defensive guard trong QueryBoundary children (Defense in depth)

```typescript
// CWELibrary.tsx — bên trong QueryBoundary children
{({ cweList }) => {
  // Guard: đảm bảo cweList luôn là array
  const safeList = Array.isArray(cweList) ? cweList : [];
  
  const categories = ["All", ...Array.from(new Set(safeList.map(c => c.category)))];
  const filtered = safeList.filter(c => ...);
  // ...
}}
```

---

## Testing

```typescript
// test: useTaxonomy.test.ts
it('useCWELibrary normalizes undefined cweList to empty array', async () => {
  server.use(
    http.get('/api/v2/cwe', () =>
      HttpResponse.json({ total: 0 })  // ← thiếu cweList field
    )
  );
  const { result } = renderHook(() => useCWELibrary());
  await waitFor(() => expect(result.current.isSuccess).toBe(true));
  expect(result.current.data?.cweList).toEqual([]);  // không undefined
});
```

---

## Files cần sửa

| File | Thay đổi |
|------|---------|
| [`useTaxonomy.ts`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/cve-intel/hooks/useTaxonomy.ts) | Thêm normalize trong `useCWELibrary` queryFn |
| [`CWELibrary.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/cve-intel/components/CWELibrary.tsx) | Optional: thêm `Array.isArray` guard |
