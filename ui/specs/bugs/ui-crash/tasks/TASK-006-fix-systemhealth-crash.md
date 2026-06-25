# TASK-006 — Fix `getServiceIcon` crash khi `svc.name` là undefined

**Bug**: [BUG-admin-health.md](../BUG-admin-health.md)  
**Solution**: [SOL-015-system-health.md](../solutions/SOL-015-system-health.md)  
**Priority**: 🟠 P1  
**Effort**: ~5 phút  
**Status**: `[x] DONE`

---

## Mô tả

Function `getServiceIcon(name: string)` gọi `name.includes(k)` nhưng `svc.name` có thể là `undefined` khi backend trả service object thiếu field `name`. Fix gồm 2 phần: (1) guard trong `getServiceIcon`, (2) normalize trong `useSystemHealth` queryFn.

---

## File cần sửa

**File**: [`ui/src/features/admin/components/SystemHealth.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/admin/components/SystemHealth.tsx)

---

## Thay đổi 1 — Guard trong `getServiceIcon`

**Tìm** (line 43–45):

```typescript
function getServiceIcon(name: string): React.ElementType {
  const key = Object.keys(SERVICE_ICONS).find(k => name.includes(k)) ?? "API";
  return SERVICE_ICONS[key] ?? Server;
}
```

**Thay bằng**:

```typescript
function getServiceIcon(name: string | undefined): React.ElementType {
  if (!name) return Server;
  const key = Object.keys(SERVICE_ICONS).find(k => name.includes(k)) ?? "API";
  return SERVICE_ICONS[key] ?? Server;
}
```

## Thay đổi 2 — Normalize trong `useSystemHealth` queryFn

**Tìm** trong `useSystemHealth`:

```typescript
    queryFn: async () => {
      const { data } = await apiClient.get<HealthData>(ENDPOINTS.admin.health);
      return data;
    },
```

**Thay bằng**:

```typescript
    queryFn: async () => {
      const { data } = await apiClient.get<HealthData>(ENDPOINTS.admin.health);
      // Normalize services — đảm bảo name luôn là string
      return {
        ...data,
        services: Array.isArray(data?.services)
          ? data.services.map((svc) => ({
              ...svc,
              name:   String(svc?.name ?? 'unknown'),
              status: String(svc?.status ?? 'unknown'),
            }))
          : [],
      };
    },
```

---

## Acceptance Criteria

- [ ] `getServiceIcon(undefined)` trả về `Server` icon thay vì crash
- [ ] Trang `/admin/health` không còn `TypeError: Cannot read properties of undefined (reading 'includes')`
- [ ] TypeScript không có lỗi mới (đổi param type thành `string | undefined`)

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/ui
npx tsc --noEmit 2>&1 | grep SystemHealth
```
