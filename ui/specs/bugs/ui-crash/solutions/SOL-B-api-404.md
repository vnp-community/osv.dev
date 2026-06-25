# SOL-B — API 404 Errors: Backend Endpoints Missing

**Bugs**: BUG-002, BUG-004, BUG-009, BUG-010, BUG-011, BUG-014, BUG-016  
**Priority**: 🟡 P3 — UI render OK khi có error handling, chỉ cần deploy endpoint

---

## Tổng hợp endpoints bị 404

| Bug | Route UI | Endpoint bị thiếu | HTTP Method |
|-----|----------|-------------------|-------------|
| BUG-002 | `/scans` | `/api/v1/scans/stats/weekly` | GET |
| BUG-004 | `/findings/risk-acceptance` | `/api/v1/findings/risk-acceptances` | GET |
| BUG-009 | `/reports` | `/api/v1/reports` | GET |
| BUG-009 | `/reports` | `/api/v1/reports/templates` | GET |
| BUG-010 | `/notifications` | `/api/v1/notifications` | GET |
| BUG-011 | `/integrations/api-keys` | `/api/v1/api-keys` | GET |
| BUG-014 | `/admin/audit` | `/api/v1/admin/audit-logs` | GET |
| BUG-016 | `/admin/settings` | `/api/v1/admin/settings` | GET |

---

## Giải pháp theo Architecture

Theo `architecture.md Section 5.6`, khi backend chưa implement endpoint,  
sử dụng **MSW Mock Layer** để trả về stub response — trang render bình thường.

### Option A — MSW Mock (Ngắn hạn, theo architecture)

Tạo/cập nhật MSW handlers để intercept các endpoint 404:

```typescript
// src/mocks/handlers/scan.handlers.ts — thêm handler cho stats/weekly
http.get('/api/v1/scans/stats/weekly', () => {
  return HttpResponse.json({
    weeks: [
      { week: '2026-W24', completed: 12, failed: 1, avgDuration: 340 },
      { week: '2026-W23', completed: 8,  failed: 0, avgDuration: 290 },
      // ...
    ],
    total: 20,
  });
}),
```

```typescript
// src/mocks/handlers/notification.handlers.ts
http.get('/api/v1/notifications', ({ request }) => {
  const url = new URL(request.url);
  const page = Number(url.searchParams.get('page') ?? '1');
  return HttpResponse.json({
    notifications: notificationsFixture.slice((page-1)*20, page*20),
    total: notificationsFixture.length,
    unread_count: 12,
  });
}),
```

```typescript
// src/mocks/handlers/report.handlers.ts
http.get('/api/v1/reports', () =>
  HttpResponse.json({ reports: [], total: 0 })
),
http.get('/api/v1/reports/templates', () =>
  HttpResponse.json({ templates: reportTemplatesFixture })
),
```

```typescript
// src/mocks/handlers/admin.handlers.ts
http.get('/api/v1/admin/audit-logs', ({ request }) => {
  const url = new URL(request.url);
  const page = Number(url.searchParams.get('page') ?? '1');
  return HttpResponse.json({
    entries: auditLogsFixture.slice((page-1)*50, page*50),
    total: auditLogsFixture.length,
    page, page_size: 50,
  });
}),
http.get('/api/v1/admin/settings', () =>
  HttpResponse.json(systemSettingsFixture)
),
http.get('/api/v1/api-keys', () =>
  HttpResponse.json({ api_keys: apiKeysFixture, total: apiKeysFixture.length })
),
```

### Option B — Backend Implementation (Dài hạn)

Triển khai các endpoints theo spec trong `specs/openapi.yaml`:

```bash
# Các endpoint cần implement trong Go backend:
GET  /api/v1/scans/stats/weekly              # ScanService.GetWeeklyStats()
GET  /api/v1/findings/risk-acceptances       # FindingService.ListRiskAcceptances()
GET  /api/v1/reports                         # ReportService.ListReports()
GET  /api/v1/reports/templates               # ReportService.ListTemplates()
GET  /api/v1/notifications                   # NotificationService.List()
GET  /api/v1/api-keys                        # AuthService.ListAPIKeys()
GET  /api/v1/admin/audit-logs                # AdminService.GetAuditLogs()
GET  /api/v1/admin/settings                  # AdminService.GetSettings()
PUT  /api/v1/admin/settings                  # AdminService.UpdateSettings()
```

---

## UI-Side: Error Handling Pattern

Ngay cả khi endpoint chưa có, UI KHÔNG nên crash — phải hiển thị EmptyState thay vì error:

```typescript
// Pattern đúng theo architecture.md:5.5.3
export function NotificationCenter() {
  const { data, isLoading, isError } = useNotifications();

  if (isLoading) return <NotificationSkeleton />;
  
  // isError = 404 từ backend → hiển thị friendly message, KHÔNG crash
  if (isError) return (
    <EmptyState
      icon={<Bell />}
      title="Notifications unavailable"
      description="This feature is being set up. Check back soon."
    />
  );
  
  if (!data?.notifications?.length) return (
    <EmptyState title="No notifications" description="You're all caught up!" />
  );

  return <NotificationList items={data.notifications} />;
}
```

> **Lưu ý**: Hiện tại các trang này không crash (không có JS crash), chỉ có 404 errors trong console.  
> Nếu component đã handle `isError` đúng (hiển thị ErrorState), thì không cần gấp — chỉ cần deploy endpoint.

---

## Kiểm tra endpoint availability

```bash
TOKEN=$(curl -s -X POST https://c12.openledger.vn/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@openvulnscan.io","password":"Admin@123!ChangeMe"}' \
  | jq -r '.access_token')

# Test từng endpoint
for ep in \
  "/api/v1/scans/stats/weekly" \
  "/api/v1/findings/risk-acceptances" \
  "/api/v1/reports" \
  "/api/v1/reports/templates" \
  "/api/v1/notifications" \
  "/api/v1/api-keys" \
  "/api/v1/admin/audit-logs" \
  "/api/v1/admin/settings"
do
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer $TOKEN" \
    "https://c12.openledger.vn$ep")
  echo "$STATUS  $ep"
done
```
