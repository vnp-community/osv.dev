# TASK-007 — Tạo MSW mock handlers cho 7 API 404 endpoints

**Bugs**: BUG-002, BUG-004, BUG-009, BUG-010, BUG-011, BUG-014, BUG-016  
**Solution**: [SOL-B-api-404.md](../solutions/SOL-B-api-404.md)  
**Priority**: 🟡 P3  
**Effort**: ~30 phút  
**Status**: `[x] DONE`

> **Ghi chú**: Task này chỉ cần thiết nếu muốn các trang render có dữ liệu giả trong môi trường dev.  
> Các trang hiện tại đã có error handling nên không crash — chỉ hiện trạng thái lỗi/empty.

---

## Mô tả

7 endpoints bị 404 vì backend chưa deploy. Tạo MSW handlers để intercept và trả stub response, các trang sẽ render có dữ liệu trong môi trường dev.

---

## Bước 1 — Kiểm tra cấu trúc MSW hiện tại

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/ui/src/mocks -type f | sort
```

---

## Bước 2 — Tạo/cập nhật handler files

### File: `src/mocks/handlers/scan.handlers.ts` (thêm handler)

Thêm vào cuối array handlers:

```typescript
// GET /api/v1/scans/stats/weekly — BUG-002
http.get('/api/v1/scans/stats/weekly', () => {
  return HttpResponse.json({
    weeks: [
      { week: '2026-W24', completed: 12, failed: 1, avgDuration: 340 },
      { week: '2026-W23', completed: 8,  failed: 0, avgDuration: 290 },
      { week: '2026-W22', completed: 15, failed: 2, avgDuration: 410 },
      { week: '2026-W21', completed: 7,  failed: 0, avgDuration: 280 },
    ],
    total: 42,
    total_failed: 3,
  });
}),
```

### File: `src/mocks/handlers/finding.handlers.ts` (thêm handler)

```typescript
// GET /api/v1/findings/risk-acceptances — BUG-004
http.get('/api/v1/findings/risk-acceptances', () => {
  return HttpResponse.json({
    risk_acceptances: [],
    total: 0,
    page: 1,
    page_size: 20,
  });
}),
```

### File: `src/mocks/handlers/report.handlers.ts` (tạo mới hoặc thêm)

```typescript
import { http, HttpResponse } from 'msw';

export const reportHandlers = [
  // GET /api/v1/reports — BUG-009
  http.get('/api/v1/reports', () =>
    HttpResponse.json({ reports: [], total: 0, page: 1, page_size: 20 })
  ),
  // GET /api/v1/reports/templates — BUG-009
  http.get('/api/v1/reports/templates', () =>
    HttpResponse.json({
      templates: [
        { id: 'executive-summary', name: 'Executive Summary', description: 'High-level risk overview' },
        { id: 'technical-detail',  name: 'Technical Detail',  description: 'Full findings with remediation' },
        { id: 'compliance',        name: 'Compliance Report', description: 'Regulatory compliance status' },
      ],
    })
  ),
];
```

### File: `src/mocks/handlers/notification.handlers.ts` (tạo mới hoặc thêm)

```typescript
import { http, HttpResponse } from 'msw';

export const notificationHandlers = [
  // GET /api/v1/notifications — BUG-010
  http.get('/api/v1/notifications', ({ request }) => {
    const url = new URL(request.url);
    const page = Number(url.searchParams.get('page') ?? '1');
    return HttpResponse.json({
      notifications: [],
      total: 0,
      unread_count: 0,
      page,
      page_size: 20,
    });
  }),
];
```

### File: `src/mocks/handlers/integration.handlers.ts` (tạo mới hoặc thêm)

```typescript
import { http, HttpResponse } from 'msw';

export const integrationHandlers = [
  // GET /api/v1/api-keys — BUG-011
  http.get('/api/v1/api-keys', () =>
    HttpResponse.json({ api_keys: [], total: 0 })
  ),
];
```

### File: `src/mocks/handlers/admin.handlers.ts` (thêm handlers)

```typescript
// GET /api/v1/admin/audit-logs — BUG-014
http.get('/api/v1/admin/audit-logs', ({ request }) => {
  const url = new URL(request.url);
  const page = Number(url.searchParams.get('page') ?? '1');
  return HttpResponse.json({
    entries: [],
    total: 0,
    page,
    page_size: 50,
  });
}),

// GET /api/v1/admin/settings — BUG-016
http.get('/api/v1/admin/settings', () =>
  HttpResponse.json({
    general: {
      platform_name: 'OpenVulnScan',
      organization: 'My Organization',
      support_email: 'support@example.com',
      timezone: 'UTC',
      date_format: 'YYYY-MM-DD',
    },
    smtp: { host: '', port: 587, username: '', from_name: 'OpenVulnScan' },
    security: {
      password_min_length: 12,
      password_max_age_days: 90,
      session_timeout_minutes: 60,
      max_concurrent_sessions: 3,
      mfa_required: false,
      allow_sms_otp: false,
    },
    ai: { providers: [], active_provider_id: '' },
  })
),
```

---

## Bước 3 — Đăng ký handlers vào `src/mocks/handlers/index.ts`

```typescript
// Đảm bảo các handlers mới được export
export * from './report.handlers';
export * from './notification.handlers';
export * from './integration.handlers';
```

---

## Acceptance Criteria

- [ ] `VITE_ENABLE_MSW=true` → tất cả 7 endpoint trả response 200 thay vì 404
- [ ] Các trang `/scans`, `/reports`, `/notifications`, `/integrations/api-keys`, `/admin/audit`, `/admin/settings` render có dữ liệu (hoặc empty state sạch)
- [ ] Không ảnh hưởng production build (`VITE_ENABLE_MSW=false`)
