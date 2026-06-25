# Hardcode Fix Solutions — OSV Platform UI

> **Dựa trên:** [architecture.md](../../architecture.md) · [TDD.md](../../TDD.md)  
> **Ngày tạo:** 2026-06-19  
> **Phạm vi:** Toàn bộ 14 components có hardcode data

---

## Nguyên tắc chỉ đạo (từ Architecture Doc)

Theo **Section 5.5 — No-Hardcode Data Rule** trong `architecture.md`:

> **NGHIÊM CẤM** `const data = [...]` trong component files.  
> Mọi dữ liệu **PHẢI** lấy từ server qua `React Query` hooks.  
> Khi dev, dùng **MSW (Mock Service Worker)** thay vì hardcode.

---

## Danh sách file giải pháp

| File | Nội dung |
|---|---|
| [01_admin_user_management.md](./01_admin_user_management.md) | UserManagement.tsx — hook + API + MSW |
| [02_admin_audit_logs.md](./02_admin_audit_logs.md) | AuditLogs.tsx — hook + API + MSW |
| [03_admin_rbac.md](./03_admin_rbac.md) | RBACManagement.tsx — hook + API + MSW |
| [04_admin_settings.md](./04_admin_settings.md) | SystemSettings.tsx — hook + API + MSW |
| [05_risk_acceptance.md](./05_risk_acceptance.md) | RiskAcceptanceCenter.tsx — hook + API + MSW |
| [06_scan_history.md](./06_scan_history.md) | ScanHistory.tsx — sử dụng hook `useScans` sẵn có |
| [07_scan_dashboard.md](./07_scan_dashboard.md) | ScanDashboard.tsx — xóa Math.random(), dùng API stats |
| [08_ai_triage.md](./08_ai_triage.md) | AITriage.tsx — hook + API + MSW |
| [09_api_key_management.md](./09_api_key_management.md) | APIKeyManagement.tsx — hook + API + bảo mật key gen |
| [10_webhook_events.md](./10_webhook_events.md) | WebhookEvents.tsx — mở rộng hook sẵn có |
| [11_report_center.md](./11_report_center.md) | ReportCenter.tsx — hook + API + dynamic product filter |
| [12_notification_center.md](./12_notification_center.md) | NotificationCenter.tsx — hook + SSE real-time |
| [13_login_screen.md](./13_login_screen.md) | LoginScreen.tsx — public stats API + env version |
| [14_product_security.md](./14_product_security.md) | ProductSecurity.tsx — dynamic default selection |
| [15_design_tokens.md](./15_design_tokens.md) | Tập trung màu sắc + threshold vào constants |
| [16_msw_fixtures.md](./16_msw_fixtures.md) | MSW handlers cho tất cả endpoints mới |

---

## Chiến lược thực thi

### Phase 1 — Foundation (không thay đổi component)
1. Tạo `shared/constants/thresholds.ts` — tập trung ngưỡng số
2. Tạo `shared/styles/tokens.css` — design tokens màu sắc
3. Tạo MSW handlers cho tất cả endpoints mới
4. Tạo MSW fixtures có dữ liệu realistic

### Phase 2 — Admin Module (ưu tiên cao)
5. Fix `UserManagement.tsx` → `useAdminUsers()`
6. Fix `AuditLogs.tsx` → `useAuditLogs()`
7. Fix `RBACManagement.tsx` → `useRBACMatrix()`
8. Fix `SystemSettings.tsx` → `useSystemSettings()`

### Phase 3 — Core Features (ưu tiên cao)
9. Fix `RiskAcceptanceCenter.tsx` → `useRiskAcceptances()`
10. Fix `AITriage.tsx` → `useAITriageQueue()`
11. Fix `APIKeyManagement.tsx` → `useAPIKeys()` + backend key gen
12. Fix `NotificationCenter.tsx` → `useNotifications()` + SSE

### Phase 4 — Scan & Report
13. Fix `ScanHistory.tsx` → reuse `useScans({status: 'completed'})`
14. Fix `ScanDashboard.tsx` → `useScanStats()` thay `Math.random()`
15. Fix `WebhookEvents.tsx` → mở rộng hook sẵn có
16. Fix `ReportCenter.tsx` → `useReports()` + dynamic products

### Phase 5 — Minor Fixes
17. Fix `LoginScreen.tsx` → public stats API + build version
18. Fix `ProductSecurity.tsx` → dynamic default selection
19. Apply design tokens toàn bộ codebase

---

## Template chuẩn cho mọi fix

```typescript
// ✅ PATTERN CHUẨN theo architecture.md Section 5.5.3
export function FeatureComponent() {
  const query = useFeatureData();  // React Query hook

  return (
    <QueryBoundary query={query} skeleton={<FeatureSkeleton />}>
      {(data) => <FeatureContent data={data} />}
    </QueryBoundary>
  );
}

// Hook pattern
function useFeatureData(params?: Params) {
  return useQuery<ResponseType>({
    queryKey: ['feature', params],
    queryFn: async () => {
      const { data } = await apiClient.get<ResponseType>(ENDPOINTS.feature.list, { params });
      return data;
    },
    staleTime: 5 * 60_000,
  });
}
```
