# SOL-015 — System Health: `TypeError: Cannot read properties of undefined (reading 'includes')`

**Bug file**: [BUG-admin-health.md](../BUG-admin-health.md)  
**Route**: `/admin/health`  
**Component**: [`SystemHealth.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/admin/components/SystemHealth.tsx)  
**Priority**: 🟠 P1

---

## Root Cause

Stack trace chỉ rõ vị trí:
```
TypeError: Cannot read properties of undefined (reading 'includes')
  at https://c12.openledger.vn/assets/SystemHealth-BF--TRmg.js:1:994
  at Array.find (<anonymous>)
  at A (SystemHealth-BF--TRmg.js:1:984)
  at children (...:1:4634)
  at Array.map (...:1:4618)
  at children (...:1:4618)
```

Có một hàm helper `A` gọi `.find()` và trong predicate gọi `.includes()` trên `undefined`.

Đây là function tại dòng ~994 trong minified code — từ architecture context, đây là:
```typescript
// Likely code:
const key = Object.keys(SERVICE_ICONS).find(k => name.includes(k)) ?? "API";
//                                                   ^^^^ name là undefined
```

Hoặc:
```typescript
// Hoặc check trạng thái service:
services.map(svc => ({
  ...svc,
  icon: SERVICE_ICONS_MAP.find(([k]) => svc.name.includes(k))?.[1] ?? DefaultIcon
  //                                          ^^^^ svc.name undefined khi backend trả thiếu field
}))
```

---

## Fix

### Fix 1 — Guard trong helper function xử lý service name

```typescript
// SystemHealth.tsx — hàm getServiceIcon hoặc tương đương
function getServiceKey(name: string | undefined): string {
  if (!name) return "API";  // ← guard undefined
  return Object.keys(SERVICE_ICONS).find(k => name.includes(k)) ?? "API";
}
```

### Fix 2 — Normalize service data trong hook

```typescript
// useSystemHealth hook (hoặc queryFn)
queryFn: async () => {
  const { data } = await apiClient.get(ENDPOINTS.admin.health);
  return {
    ...data,
    services: Array.isArray(data?.services)
      ? data.services.map((svc: unknown) => {
          const s = svc as Record<string, unknown>;
          return {
            name:    String(s?.name ?? 'unknown'),   // ← normalize string
            status:  String(s?.status ?? 'unknown'),
            latency: s?.latency ?? null,
            // ...other fields
          };
        })
      : [],
  };
},
```

### Fix 3 — Optional chaining pattern

```typescript
// Bất kỳ chỗ nào dùng .includes() trên field từ server
const key = Object.keys(SERVICE_ICONS).find(k => svc?.name?.includes(k)) ?? "API";
//                                                    ^^^ optional chain
```

---

## Files cần sửa

| File | Thay đổi |
|------|---------|
| [`SystemHealth.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/admin/components/SystemHealth.tsx) | Thêm null guard trước `.includes()`, normalize service name |
| Hook tương ứng | Normalize `services[].name` → đảm bảo là string |
