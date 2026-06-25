# TASK-004 — Normalize `useWebhooks` queryFn để fix `undefined.find`

**Bug**: [BUG-integrations-webhooks.md](../BUG-integrations-webhooks.md)  
**Solution**: [SOL-012-webhooks.md](../solutions/SOL-012-webhooks.md)  
**Priority**: 🟠 P1  
**Effort**: ~5 phút  
**Status**: `[x] DONE`

---

## Mô tả

`useWebhooks` trong `WebhookEvents.tsx` không normalize response. `QueryBoundary` destructure `{ webhooks }` — nếu `webhooks` là `undefined`, `.find()` crash.

---

## File cần sửa

**File**: [`ui/src/features/integrations/components/WebhookEvents.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/integrations/components/WebhookEvents.tsx)

---

## Thay đổi chính xác

**Tìm đoạn code** `useWebhooks` (khoảng line 30–35):

```typescript
function useWebhooks() {
  return useQuery<WebhooksResponse>({
    queryKey: ['webhooks'],
    queryFn: async () => {
      const { data } = await apiClient.get<WebhooksResponse>(ENDPOINTS.webhooks.list);
      return data;
    },
```

**Thay bằng**:

```typescript
function useWebhooks() {
  return useQuery<WebhooksResponse>({
    queryKey: ['webhooks'],
    queryFn: async () => {
      const { data } = await apiClient.get<WebhooksResponse>(ENDPOINTS.webhooks.list);
      // Normalize — đảm bảo webhooks luôn là array
      return {
        webhooks: Array.isArray(data?.webhooks) ? data.webhooks : [],
        total:    typeof data?.total === 'number' ? data.total : 0,
      };
    },
```

---

## Acceptance Criteria

- [ ] `useWebhooks` trả về `{ webhooks: [], total: 0 }` khi backend thiếu field
- [ ] Trang `/integrations/webhooks` không còn `TypeError: Cannot read properties of undefined (reading 'find')`
- [ ] TypeScript không có lỗi mới

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/ui
npx tsc --noEmit 2>&1 | grep WebhookEvents
```
