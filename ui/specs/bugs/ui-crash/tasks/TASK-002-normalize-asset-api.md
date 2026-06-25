# TASK-002 — Normalize `assetApi.list` response để fix `null.filter`

**Bug**: [BUG-assets.md](../BUG-assets.md)  
**Solution**: [SOL-005-assets.md](../solutions/SOL-005-assets.md)  
**Priority**: 🔴 P0  
**Effort**: ~5 phút  
**Status**: `[x] DONE`

---

## Mô tả

`assetApi.list()` trả thẳng `data` từ server mà không normalize. Khi backend trả `{ assets: null }`, component `AssetInventory` gọi `data.assets.filter(...)` → crash.  
Fix tại tầng API (`assetApi.ts`) để normalize một lần, không cần sửa component.

---

## File cần sửa

**File**: [`ui/src/features/assets/api/assetApi.ts`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/assets/api/assetApi.ts)

---

## Thay đổi chính xác

**Tìm đoạn code** trong `assetApi.list`:

```typescript
  list: async (params?: AssetsListParams): Promise<AssetsListResponse> => {
    const { data } = await apiClient.get<AssetsListResponse>(
      ENDPOINTS.assets.list,
      { params }
    );
    return data;
  },
```

**Thay bằng**:

```typescript
  list: async (params?: AssetsListParams): Promise<AssetsListResponse> => {
    const { data } = await apiClient.get<AssetsListResponse>(
      ENDPOINTS.assets.list,
      { params }
    );
    // Normalize — đảm bảo assets luôn là array dù backend trả null/undefined
    return {
      assets: Array.isArray(data?.assets) ? data.assets : [],
      total:  typeof data?.total === 'number' ? data.total : 0,
    };
  },
```

---

## Acceptance Criteria

- [ ] `assetApi.list()` không bao giờ trả `assets: null` hoặc `assets: undefined`
- [ ] Trang `/assets` không còn `TypeError: Cannot read properties of null (reading 'filter')`
- [ ] Hiển thị empty state khi không có assets thay vì crash
- [ ] TypeScript không có lỗi mới

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/ui
npx tsc --noEmit 2>&1 | grep assetApi
```
