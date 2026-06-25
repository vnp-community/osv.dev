# SOL-012 — Webhook Events: `TypeError: Cannot read properties of undefined (reading 'find')`

**Bug file**: [BUG-integrations-webhooks.md](../BUG-integrations-webhooks.md)  
**Route**: `/integrations/webhooks`  
**Component**: [`WebhookEvents.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/integrations/components/WebhookEvents.tsx)  
**Priority**: 🟠 P1

---

## Root Cause

```typescript
// WebhookEvents.tsx:84 — crash point
const selected = webhooks.find((w) => w.id === selectedId) ?? webhooks[0];
```

Khi `webhooks` là `undefined` (API trả về shape không đúng), `.find()` sẽ crash.  
Pattern này thường xảy ra khi response wrapper là `{ data: [...] }` nhưng component expect `[...]` trực tiếp.

---

## Fix

### Fix 1 — Normalize trong useWebhooks hook

```typescript
// features/integrations/hooks/ (tạo mới hoặc inline)
export function useWebhooks() {
  return useQuery<{ webhooks: Webhook[]; total: number }>({
    queryKey: ['integrations', 'webhooks'],
    queryFn: async () => {
      const { data } = await apiClient.get(ENDPOINTS.integrations.webhooks);
      return {
        webhooks: Array.isArray(data?.webhooks) ? data.webhooks
                : Array.isArray(data)            ? data        // handle array response
                : [],
        total: data?.total ?? 0,
      };
    },
    staleTime: 30_000,
    placeholderData: (prev) => prev,
  });
}
```

### Fix 2 — Guard trước `.find()`

```typescript
// WebhookEvents.tsx
{(data) => {
  const webhooks = Array.isArray(data?.webhooks) ? data.webhooks : [];
  const selected = webhooks.find((w) => w.id === selectedId) ?? webhooks[0] ?? null;

  if (!selected) return <EmptyState title="No webhooks configured" />;
  // ...
}}
```

---

## Files cần sửa

| File | Thay đổi |
|------|---------|
| [`WebhookEvents.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/integrations/components/WebhookEvents.tsx) | Normalize array + guard trước `.find()` |
