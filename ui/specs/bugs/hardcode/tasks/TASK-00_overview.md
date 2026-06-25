# Task Overview — Hardcode Fix OSV Platform UI

> **Nguồn giải pháp:** [`solutions/`](../solutions/)  
> **Ngày tạo:** 2026-06-19  
> **Tổng số tác vụ:** 19 tasks + 4 tasks nền tảng  
> **Quy tắc:** No-Hardcode Data Rule (architecture.md Section 5.5)

---

## Danh sách tác vụ theo Phase

| Task File | Phase | Component | Độ ưu tiên |
|---|---|---|---|
| [TASK-P1-01](./TASK-P1-01_thresholds.md) | Phase 1 | `shared/constants/thresholds.ts` | 🔴 Foundation |
| [TASK-P1-02](./TASK-P1-02_design_tokens.md) | Phase 1 | `shared/styles/tokens.css` | 🔴 Foundation |
| [TASK-P1-03](./TASK-P1-03_msw_setup.md) | Phase 1 | `src/mocks/` structure | 🔴 Foundation |
| [TASK-P1-04](./TASK-P1-04_msw_fixtures.md) | Phase 1 | MSW fixtures tất cả endpoints | 🔴 Foundation |
| [TASK-P2-01](./TASK-P2-01_user_management.md) | Phase 2 | `UserManagement.tsx` | 🟠 Admin |
| [TASK-P2-02](./TASK-P2-02_audit_logs.md) | Phase 2 | `AuditLogs.tsx` | 🟠 Admin |
| [TASK-P2-03](./TASK-P2-03_rbac.md) | Phase 2 | `RBACManagement.tsx` | 🟠 Admin |
| [TASK-P2-04](./TASK-P2-04_system_settings.md) | Phase 2 | `SystemSettings.tsx` | 🟠 Admin |
| [TASK-P3-01](./TASK-P3-01_risk_acceptance.md) | Phase 3 | `RiskAcceptanceCenter.tsx` | 🟡 Core |
| [TASK-P3-02](./TASK-P3-02_ai_triage.md) | Phase 3 | `AITriage.tsx` | 🟡 Core |
| [TASK-P3-03](./TASK-P3-03_api_key_management.md) | Phase 3 | `APIKeyManagement.tsx` | 🟡 Core |
| [TASK-P3-04](./TASK-P3-04_notification_center.md) | Phase 3 | `NotificationCenter.tsx` | 🟡 Core |
| [TASK-P4-01](./TASK-P4-01_scan_history.md) | Phase 4 | `ScanHistory.tsx` | 🔵 Scan |
| [TASK-P4-02](./TASK-P4-02_scan_dashboard.md) | Phase 4 | `ScanDashboard.tsx` | 🔵 Scan |
| [TASK-P4-03](./TASK-P4-03_webhook_events.md) | Phase 4 | `WebhookEvents.tsx` | 🔵 Scan |
| [TASK-P4-04](./TASK-P4-04_report_center.md) | Phase 4 | `ReportCenter.tsx` | 🔵 Scan |
| [TASK-P5-01](./TASK-P5-01_login_screen.md) | Phase 5 | `LoginScreen.tsx` | 🟢 Minor |
| [TASK-P5-02](./TASK-P5-02_product_security.md) | Phase 5 | `ProductSecurity.tsx` | 🟢 Minor |
| [TASK-P5-03](./TASK-P5-03_apply_design_tokens.md) | Phase 5 | Apply tokens toàn codebase | 🟢 Minor |

---

## Thứ tự thực hiện bắt buộc

```
Phase 1 (Foundation) → Phase 2 (Admin) → Phase 3 (Core) → Phase 4 (Scan) → Phase 5 (Minor)
```

Phase 1 PHẢI hoàn thành trước tất cả phase còn lại vì MSW handlers là dependency.

---

## Pattern chuẩn cho mọi task

```typescript
// ✅ PATTERN theo architecture.md Section 5.5.3
export function FeatureComponent() {
  const query = useFeatureData();  // React Query hook
  return (
    <QueryBoundary query={query} skeleton={<FeatureSkeleton />}>
      {(data) => <FeatureContent data={data} />}
    </QueryBoundary>
  );
}
```

---

## Trạng thái tổng quan

- [x] Phase 1 — Foundation ✅ (2026-06-19)
- [x] Phase 2 — Admin Module ✅ (2026-06-19)
- [x] Phase 3 — Core Features ✅ (2026-06-19)
- [x] Phase 4 — Scan & Report ✅ (2026-06-19)
- [x] Phase 5 — Minor Fixes ✅ (2026-06-19)
