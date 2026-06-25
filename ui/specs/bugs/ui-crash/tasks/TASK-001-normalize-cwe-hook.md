# TASK-001 — Normalize `useCWELibrary` response để fix `undefined.map`

**Bug**: [BUG-cve-cwe.md](../BUG-cve-cwe.md)  
**Solution**: [SOL-001-cwe-library.md](../solutions/SOL-001-cwe-library.md)  
**Priority**: 🟠 P1  
**Effort**: ~5 phút  
**Status**: `[x] DONE`

---

## Mô tả

Hook `useCWELibrary` không normalize response từ backend. Khi backend trả về response thiếu field `cweList` hoặc trả `null`, component crash với `TypeError: Cannot read properties of undefined (reading 'map')`.

---

## File cần sửa

**File**: [`ui/src/features/cve-intel/hooks/useTaxonomy.ts`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/cve-intel/hooks/useTaxonomy.ts)

---

## Thay đổi chính xác

**Tìm đoạn code** (lines 58–68):

```typescript
export function useCWELibrary(params?: { q?: string }) {
  return useQuery<CWEListResponse>({
    queryKey: cveKeys.cwe('list'),
    queryFn: async () => {
      const { data } = await apiClient.get<CWEListResponse>(ENDPOINTS.cwe.list, { params });
      return data;
    },
    staleTime: 10 * 60_000,
    placeholderData: (prev) => prev,
  });
}
```

**Thay bằng**:

```typescript
export function useCWELibrary(params?: { q?: string }) {
  return useQuery<CWEListResponse>({
    queryKey: cveKeys.cwe('list'),
    queryFn: async () => {
      const { data } = await apiClient.get<CWEListResponse>(ENDPOINTS.cwe.list, { params });
      // Normalize — đảm bảo cweList luôn là array dù backend trả shape khác
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

---

## Acceptance Criteria

- [ ] `useCWELibrary` trả về `{ cweList: [], total: 0 }` khi backend trả `{}` hoặc `null`
- [ ] `useCWELibrary` trả về đúng data khi backend trả đúng shape
- [ ] Trang `/cve/cwe` không còn console error
- [ ] TypeScript không có lỗi mới

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/ui
npx tsc --noEmit 2>&1 | grep useTaxonomy
```
