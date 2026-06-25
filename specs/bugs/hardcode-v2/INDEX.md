# Hardcode / Stub Bug Index — v2

> Audit ngày: 2026-06-24
> Scope: `/services/**/*.go` — rà soát tiếp sau hardcode-v1
> Tổng số bugs: **7**
> Loại bug: Stub handlers trả hardcode data thay vì query DB thực

---

## Bảng Tổng Hợp

| ID | Severity | Service | Loại | Mô tả ngắn | File chính | Status |
|----|----------|---------|------|-------------|-----------|--------|
| [BUG-H2-001](./BUG-H2-001-asset-service-get-asset-shell.md) | 🔴 High | asset-service | Stub Handler | `GetAsset` trả `{"id":"..."}` không query DB | `handlers.go:L169` | ✅ Fixed |
| [BUG-H2-002](./BUG-H2-002-asset-service-get-history-stub.md) | 🟡 Medium | asset-service | Stub Handler | `GetHistory` hardcode `[]` | `handlers.go:L224` | ✅ Fixed |
| [BUG-H2-003](./BUG-H2-003-asset-service-get-findings-stub.md) | 🟡 Medium | asset-service | Stub Handler | `GetFindings` hardcode `[]` | `handlers.go:L281` | ✅ Fixed |
| [BUG-H2-004](./BUG-H2-004-sla-service-placeholder-handlers.md) | 🔴 High | sla-service | Placeholder | 5 handler functions trả 0/501 dù DB connected | `main.go:L131-180` | ✅ Fixed |
| [BUG-H2-005](./BUG-H2-005-notification-jira-stubs.md) | 🟠 Medium | notification-service | Orphan Stubs | 6 package-level stubs trả 501 orphan từ `IntegrationHandler` | `integration_handler.go:L245` | ✅ Fixed |
| [BUG-H2-006](./BUG-H2-006-notification-alerts-nil-guard.md) | 🟠 Medium | notification-service | Nil Guard | `AlertsHandler` nil → 503 /notifications | `router.go:L86` | ✅ Verified OK |
| [BUG-H2-007](./BUG-H2-007-scan-service-create-scan-stub.md) | 🔴 High | scan-service | Stub Handler | `CreateScan` → 503 hardcode | `router.go:L120` | ✅ Fixed (prev session) |

---

## Phân Loại Theo Mức Độ Ưu Tiên

### 🔴 High (dữ liệu bị ẩn/mất)

1. **BUG-H2-001**: `GetAsset` không bao giờ query DB → GET /assets/{id} luôn trả stub
2. **BUG-H2-004**: SLA service placeholder → tất cả SLA CRUD trả 0 hoặc 501
3. ~~**BUG-H2-007**~~: `CreateScan` 503 → ✅ **Đã fix 2026-06-24**

### 🟠 Medium

4. **BUG-H2-005**: Jira integration stubs → 6 routes trả 501 (nên dùng IntegrationHandler)
5. **BUG-H2-006**: AlertsHandler nil guard → `/notifications` trả 503

### 🟡 Low

6. **BUG-H2-002**: `GetHistory` hardcode `[]`
7. **BUG-H2-003**: `GetFindings` hardcode `[]`
