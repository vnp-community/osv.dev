import { http, HttpResponse } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';
import { getUserByEmail, userFixtures } from '../fixtures/auth.fixture';
import type { LoginRequest } from '@/features/auth/types';

// Session state (in-memory — reset mỗi khi MSW worker restart)
let currentUser = userFixtures.bob;
let fakeToken = 'mock-access-token-init';

export const authHandlers = [
  // POST /api/v1/auth/login
  http.post(ENDPOINTS.auth.login, async ({ request }) => {
    const body = await request.json() as LoginRequest;

    const user = getUserByEmail(body.email);
    if (!user) {
      return HttpResponse.json(
        {
          error: 'INVALID_CREDENTIALS',
          message: 'Invalid email or password',
          details: {},
          trace_id: 'mock-auth-001',
        },
        { status: 401 }
      );
    }

    currentUser = user;
    fakeToken = `mock-token-${user.role}-${Date.now()}`;

    return HttpResponse.json(
      {
        access_token: fakeToken,
        expires_in: 900,
        user: currentUser,
        mfa_required: false,
      },
      {
        headers: {
          'Set-Cookie': `refresh_token=mock-refresh-${user.role}; HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth/refresh; Max-Age=604800`,
        },
      }
    );
  }),

  // POST /api/v1/auth/refresh
  http.post(ENDPOINTS.auth.refresh, () => {
    fakeToken = `mock-refreshed-${Date.now()}`;
    return HttpResponse.json({
      access_token: fakeToken,
      expires_in: 900,
    });
  }),

  // GET /api/v1/auth/me
  http.get(ENDPOINTS.auth.me, () => {
    return HttpResponse.json({ user: currentUser });
  }),

  // POST /api/v1/auth/logout
  http.post(ENDPOINTS.auth.logout, () => {
    return HttpResponse.json(
      { success: true },
      {
        headers: {
          'Set-Cookie': 'refresh_token=; HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth/refresh; Max-Age=0',
        },
      }
    );
  }),

  // GET /api/v1/auth/mfa/setup
  http.get(ENDPOINTS.auth.mfaSetup, () => {
    return HttpResponse.json({
      secret: 'JBSWY3DPEHPK3PXP',
      qr_url: `otpauth://totp/OSV:${currentUser.email}?secret=JBSWY3DPEHPK3PXP&issuer=OSV%20Platform`,
      backup_codes: ['a1b2-c3d4', 'e5f6-g7h8', 'i9j0-k1l2', 'm3n4-o5p6'],
    });
  }),

  // POST /api/v1/auth/mfa/confirm
  http.post(ENDPOINTS.auth.mfaConfirm, async ({ request }) => {
    const body = await request.json() as { code: string };
    if (body.code === '123456' || /^\d{6}$/.test(body.code)) {
      return HttpResponse.json({ success: true, mfa_enabled: true });
    }
    return HttpResponse.json(
      { error: 'INVALID_MFA_CODE', message: 'TOTP code is invalid', details: {}, trace_id: 'mock-mfa-001' },
      { status: 400 }
    );
  }),

  // GET /api/v1/auth/oauth/google
  http.get(ENDPOINTS.auth.oauthGoogle, () => {
    return HttpResponse.json({
      redirect_url: 'https://accounts.google.com/o/oauth2/auth?client_id=mock&redirect_uri=mock&response_type=code&scope=openid+email+profile&state=mock-state',
    });
  }),

  // GET /api/v1/auth/oauth/github
  http.get(ENDPOINTS.auth.oauthGitHub, () => {
    return HttpResponse.json({
      redirect_url: 'https://github.com/login/oauth/authorize?client_id=mock&redirect_uri=mock&scope=read:user+user:email&state=mock-state',
    });
  }),

  // GET /api/v1/auth/callback
  http.get(ENDPOINTS.auth.oauthCallback, ({ request }) => {
    const url = new URL(request.url);
    const code = url.searchParams.get('code');
    if (!code) {
      return HttpResponse.json(
        { error: 'INVALID_CALLBACK', message: 'Missing code parameter', details: {}, trace_id: 'mock-oauth-001' },
        { status: 400 }
      );
    }
    fakeToken = `mock-oauth-token-${Date.now()}`;
    return HttpResponse.json({
      access_token: fakeToken,
      expires_in: 900,
      user: { ...currentUser, email: 'oauth-user@gmail.com', name: 'OAuth User' },
    });
  }),

  // GET /api/v2/public/stats — TASK-P5-01: no auth, login page stats
  http.get('/api/v2/public/stats', () => {
    return HttpResponse.json({
      totalCVEs: '240K+',
      scansToday: 1847,
      findingAccuracy: '98.4%',
      uptimeSLA: '99.99%',
      threatIndicators: {
        criticalThreats: 14,
        kevActive: 7,
        assetsAtRisk: 23,
      },
    });
  }),
];

