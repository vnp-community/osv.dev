/**
 * profile.handlers.ts — MSW handlers cho /api/v1/profile/*
 * Phục vụ UserProfile.tsx (sessions tab, notifications settings tab)
 */
import { http, HttpResponse } from 'msw';
import { sessionsFixture, notifSettingsFixture } from '../fixtures/profile.fixture';

const BASE = '';

// In-memory mutable copy for PUT support
let notifSettingsState = [...notifSettingsFixture];

export const profileHandlers = [
  // GET /api/v1/profile/sessions
  http.get(`${BASE}/api/v1/profile/sessions`, () => {
    return HttpResponse.json({
      items: sessionsFixture,
      total: sessionsFixture.length,
    });
  }),

  // GET /api/v1/profile/notifications/settings
  http.get(`${BASE}/api/v1/profile/notifications/settings`, () => {
    return HttpResponse.json({
      items: notifSettingsState,
    });
  }),

  // PUT /api/v1/profile/notifications/settings
  http.put(`${BASE}/api/v1/profile/notifications/settings`, async ({ request }) => {
    const body = await request.json() as { items: typeof notifSettingsState };
    if (Array.isArray(body?.items)) {
      notifSettingsState = body.items;
    }
    return HttpResponse.json({ items: notifSettingsState });
  }),

  // DELETE /api/v1/profile/sessions/:id — revoke a session
  http.delete(`${BASE}/api/v1/profile/sessions/:id`, ({ params }) => {
    const session = sessionsFixture.find(s => s.id === params.id);
    if (!session) return new HttpResponse(null, { status: 404 });
    if (session.current) {
      return HttpResponse.json(
        { error: 'CANNOT_REVOKE_CURRENT', message: 'Cannot revoke your current session' },
        { status: 400 }
      );
    }
    return new HttpResponse(null, { status: 204 });
  }),
];
