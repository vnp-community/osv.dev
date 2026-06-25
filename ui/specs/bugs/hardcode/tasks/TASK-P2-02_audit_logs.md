# TASK-P2-02 — Fix `AuditLogs.tsx` → `useAuditLogs()`

**Phase:** 2 — Admin Module  
**Nguồn giải pháp:** [`solutions/02_admin_audit_logs.md`](../solutions/02_admin_audit_logs.md)  
**Ưu tiên:** 🟠 Admin — ưu tiên cao  
**Phụ thuộc:** TASK-P1-03, TASK-P1-04, TASK-P2-01 (dùng chung `features/admin/types.ts`)

---

## Vấn đề hiện tại

```typescript
// ❌ HIỆN TẠI — features/admin/components/AuditLogs.tsx
const auditLogs = [
  { id: 'AL-1001', timestamp: '...', user: '...', action: 'CREATE_SCAN', ... },
  // 10 audit events hardcode
];
// Filter chỉ hoạt động local, không phân trang
```

---

## API Endpoint

```
GET /api/v1/audit-log?severity=Critical&search=&page=1&pageSize=50
```
> ENDPOINTS.audit.log đã có sẵn trong endpoints.ts

---

## Danh sách files cần tạo/sửa

### [MODIFY] `src/features/admin/types.ts` — thêm AuditEvent types

```typescript
export interface AuditEvent {
  id: string;
  userId: string;
  userName: string;
  action: string;
  entityType: string;
  entityId: string;
  ipAddress: string;
  userAgent?: string;
  result: 'success' | 'failure';
  metadata?: Record<string, unknown>;
  timestamp: string;
  // UI fields
  resource?: string;
  severity?: 'Info' | 'Warning' | 'Critical';
  before?: string;
  after?: string;
}

export interface AuditLogsResponse {
  events: AuditEvent[];
  total: number;
  page: number;
  pageSize: number;
}
```

### [NEW] `src/features/admin/hooks/useAuditLogs.ts`

```typescript
export interface AuditLogsParams {
  search?: string;
  severity?: 'Info' | 'Warning' | 'Critical';
  action?: string;
  userId?: string;
  dateFrom?: string;
  dateTo?: string;
  page?: number;
  pageSize?: number;
}

export function useAuditLogs(params?: AuditLogsParams) {
  return useQuery<AuditLogsResponse>({
    queryKey: ['audit', 'list', params],
    queryFn: async () => {
      const { data } = await apiClient.get<AuditLogsResponse>(ENDPOINTS.audit.log, { params });
      return data;
    },
    staleTime: 30_000,
  });
}
```

### [MODIFY] `src/features/admin/components/AuditLogs.tsx`

Xem code đầy đủ tại: [`solutions/02_admin_audit_logs.md`](../solutions/02_admin_audit_logs.md) — mục "Component sau khi fix"

**Thay đổi chính:**
- Xóa `const auditLogs = [...]`
- Dùng `useQuery` trực tiếp với server params (search, severity, page)
- Pagination: `page` state, nút Prev/Next
- Expand row → hiển thị `before`/`after` JSON từ server

### [NEW] `src/mocks/handlers/audit.handlers.ts`

Xem code đầy đủ tại: [`solutions/02_admin_audit_logs.md`](../solutions/02_admin_audit_logs.md) — mục "MSW Handler"

Export: `auditHandlers`  
Import từ: `src/mocks/fixtures/audit.fixture.ts`

### [MODIFY] `src/mocks/handlers/index.ts`

```typescript
import { auditHandlers } from './audit.handlers';
// ...
export const handlers = [...auditHandlers, ...];
```

---

## Tiêu chí hoàn thành

- [x] `features/admin/types.ts` có AuditEvent, AuditLogsResponse (trong types.ts chung)
- [x] `features/admin/hooks/useAuditLogs.ts` tạo xong
- [x] `AuditLogs.tsx` không còn `const auditLogs = [...]`
- [x] Search và severity filter gửi params lên API (server-side filter trong MSW)
- [x] Click vào row mở rộng → hiển thị before/after JSON với tryParseJson safe
- [x] MSW handler filter đúng theo severity và search (import từ audit-logs.fixture.ts)
- [x] Loading spinner khi fetch, error state với Retry
- [x] TypeScript 0 lỗi mới

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo/sửa:**
- [`features/admin/types.ts`](../../../../ui/src/features/admin/types.ts) — AuditEvent, AuditLogsResponse, AuditLogsParams
- [`features/admin/hooks/useAuditLogs.ts`](../../../../ui/src/features/admin/hooks/useAuditLogs.ts) — [NEW]
- [`features/admin/components/AuditLogs.tsx`](../../../../ui/src/features/admin/components/AuditLogs.tsx) — [MODIFY] Refactored
- [`mocks/handlers/admin.handlers.ts`](../../../../ui/src/mocks/handlers/admin.handlers.ts) — import audit-logs.fixture, server-side filter

---

## Kiểm tra

```bash
# Mở http://localhost:3000 → Admin → Audit Logs
# 1. Verify: events hiển thị từ MSW
# 2. Filter "Critical" → chỉ hiện critical events
# 3. Search "carol" → chỉ hiện events của carol
# 4. Click vào row UPDATE_FINDING → expand → thấy before/after JSON
```
