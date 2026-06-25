/**
 * Notification Handlers — MSW mock for notification endpoints.
 * Updated: TASK-P3-04 — returns NotificationsListResponse format for useNotificationList()
 */
import { http, HttpResponse, delay } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';
import { notificationsFixture } from '../fixtures/notifications.fixture';

const BASE = '';

// ─── Mutable store so mark-read mutations persist ─────────────────────────────
let notifStore = notificationsFixture.map((n) => ({ ...n }));

export const notificationHandlers = [
  // GET /api/v1/notifications
  http.get(`${BASE}${ENDPOINTS.notifications.list}`, async ({ request }) => {
    await delay(300);
    const url = new URL(request.url);
    const typeFilter = url.searchParams.get('type');
    const unreadOnly = url.searchParams.get('unread_only') === 'true';

    let items = [...notifStore];
    if (typeFilter) items = items.filter((n) => n.type === typeFilter);
    if (unreadOnly) items = items.filter((n) => !n.read);

    const unreadCount = notifStore.filter((n) => !n.read).length;

    return HttpResponse.json({
      items,
      total: items.length,
      unread_count: unreadCount,
    });
  }),

  // POST /api/v1/notifications/:id/read — mark single notification read
  http.post(`${BASE}/api/v1/notifications/:id/read`, async ({ params }) => {
    await delay(200);
    const notif = notifStore.find((n) => n.id === params.id);
    if (notif) notif.read = true;
    return HttpResponse.json({ id: params.id, read: true });
  }),

  // POST /api/v1/notifications/mark-all-read — mark all read
  http.post(`${BASE}${ENDPOINTS.notifications.markAllRead}`, async () => {
    await delay(300);
    const markedCount = notifStore.filter((n) => !n.read).length;
    notifStore.forEach((n) => { n.read = true; });
    return HttpResponse.json({ marked_count: markedCount });
  }),

  // GET /api/v1/notifications/unread-count
  http.get(`${BASE}${ENDPOINTS.notifications.unreadCount}`, async () => {
    const unread_count = notifStore.filter((n) => !n.read).length;
    return HttpResponse.json({ unread_count });
  }),
];
