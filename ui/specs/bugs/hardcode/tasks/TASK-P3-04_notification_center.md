# TASK-P3-04 — Fix `NotificationCenter.tsx` → `useNotifications()` + SSE

**Phase:** 3 — Core Features  
**Nguồn giải pháp:** [`solutions/10_11_12_webhook_report_notification.md` — Solution 12](../solutions/10_11_12_webhook_report_notification.md)  
**Ưu tiên:** 🟡 Core — ưu tiên cao  
**Phụ thuộc:** TASK-P1-03, TASK-P1-04

---

## Vấn đề hiện tại

```typescript
// ❌ HIỆN TẠI — features/notifications/components/NotificationCenter.tsx
const notifications = [
  { id: 'n-1', type: 'critical', title: 'Critical Finding Detected', read: false, ... },
  // 10 notifications hardcode
];
// "Mark All Read" button không có tác dụng
// Không có real-time (mọi thứ tĩnh)
```

---

## API Endpoints

```
GET  /api/v1/notifications            → Danh sách (ENDPOINTS.notifications.list)
POST /api/v1/notifications/mark-all-read → Mark all read
POST /api/v1/notifications/:id/read   → Mark 1 notification đã đọc
GET  /api/v1/notifications/stream     → SSE stream (ENDPOINTS.notifications.stream)
```

---

## Danh sách files cần tạo/sửa

### [NEW] `src/features/notifications/types.ts` (hoặc shared/types)

```typescript
export interface Notification {
  id: string;
  type: 'critical' | 'sla' | 'kev' | 'scan';
  title: string;
  description: string;
  product?: string;
  read: boolean;
  createdAt: string;
  timeAgo: string;
  metadata?: Record<string, unknown>;
}

export interface NotificationsResponse {
  notifications: Notification[];
  total: number;
  unreadCount: number;
}
```

### [NEW] `src/features/notifications/hooks/useNotifications.ts`

```typescript
export function useNotifications(params?: { type?: string; unreadOnly?: boolean }) {
  return useQuery<NotificationsResponse>({
    queryKey: ['notifications', 'list', params],
    queryFn: ...,
    staleTime: 30_000,
    refetchInterval: 60_000,  // Poll mỗi 1 phút khi không có SSE
  });
}

export function useMarkAllRead() { ... }
export function useMarkRead() { ... }

// SSE real-time hook
export function useNotificationSSE() {
  const queryClient = useQueryClient();
  useEffect(() => {
    const source = new EventSource(ENDPOINTS.notifications.stream, { withCredentials: true });
    source.onmessage = () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
    };
    source.onerror = () => source.close();
    return () => source.close();
  }, [queryClient]);
}
```

### [MODIFY] `src/features/notifications/components/NotificationCenter.tsx`

Xem code đầy đủ tại: [`solutions/10_11_12_webhook_report_notification.md`](../solutions/10_11_12_webhook_report_notification.md) — Solution 12

**Thay đổi chính:**
- Xóa `const notifications = [...]`
- Import `useNotifications`, `useMarkAllRead`, `useMarkRead`, `useNotificationSSE`
- `unreadCount` lấy từ `notifQuery.data?.unreadCount` (không hardcode badge)
- "Mark All Read" gọi `markAllRead.mutate()`
- Gọi `useNotificationSSE()` để kết nối real-time
- Filter type gửi `type` param lên API

### [MODIFY] `src/shared/api/endpoints.ts` — thêm notifications

```typescript
notifications: {
  list:        '/api/v1/notifications',
  markAllRead: '/api/v1/notifications/mark-all-read',
  markRead:    (id: string) => `/api/v1/notifications/${id}/read`,
  stream:      '/api/v1/notifications/stream',
},
```

### [NEW] `src/mocks/handlers/notifications.handlers.ts`

```typescript
export const notificationHandlers = [
  http.get('/api/v1/notifications', ...),
  http.post('/api/v1/notifications/mark-all-read', ...),
  http.post('/api/v1/notifications/:id/read', ...),
  // SSE endpoint: MSW không cần implement (fallback về polling)
];
```

Import từ `src/mocks/fixtures/notifications.fixture.ts`

---

## Tiêu chí hoàn thành

- [x] `features/notifications/hooks/useNotificationList.ts` tạo xong (GET + mark-read + mark-all-read)
- [x] `NotificationCenter.tsx` không còn `const notifications = [...]`
- [x] unreadCount badge tính từ `data.unread_count` (server)
- [x] "Mark All Read" gọi POST mutation + optimistic update
- [x] Filter theo type gửi `type` param lên API
- [x] TYPE_CONFIG map: icon + color từ CSS variables
- [x] formatTimeAgo() từ ISO string
- [x] MSW handler: import từ notifications.fixture.ts, mutable read state
- [x] SSE hook cũ (useNotifications) giữ nguyên, không bị phá
- [x] TypeScript 0 lỗi mới

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo/sửa:**
- [`features/notifications/hooks/useNotificationList.ts`](../../../../ui/src/features/notifications/hooks/useNotificationList.ts) — [NEW] 3 hooks + optimistic update
- [`features/notifications/components/NotificationCenter.tsx`](../../../../ui/src/features/notifications/components/NotificationCenter.tsx) — [MODIFY] Refactored
- [`mocks/handlers/notification.handlers.ts`](../../../../ui/src/mocks/handlers/notification.handlers.ts) — [MODIFY] Import fixture, {items, total, unread_count} format

---

## Kiểm tra

```bash
# Mở http://localhost:3000 → Notifications
# 1. Verify: 5 notifications từ MSW, badge hiển thị "3" (3 unread)
# 2. Filter "critical" → chỉ thấy n-1, n-5
# 3. "Mark All Read" → badge về 0, tất cả notifications không còn bold
# 4. Click vào 1 notification → mark as read (1 item cụ thể)
```
