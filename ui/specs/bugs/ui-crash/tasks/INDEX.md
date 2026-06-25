# Tasks — OSV Platform UI Bug Fixes

**Nguồn**: Solutions từ scan Playwright headless, 2026-06-20  
**Tổng tasks**: 9  

---

## Nhóm A — JS Crash (Fix code UI, thực thi ngay)

| Task | File | Bug | Priority | Effort |
|------|------|-----|----------|--------|
| [TASK-001](TASK-001-normalize-cwe-hook.md) | `useTaxonomy.ts` | CWE Library `undefined.map` | 🟠 P1 | ~5 min |
| [TASK-002](TASK-002-normalize-asset-api.md) | `assetApi.ts` | Assets `null.filter` | 🔴 P0 | ~5 min |
| [TASK-003](TASK-003-normalize-products-hook.md) | `ProductSecurity.tsx` | Products `undefined.flatMap` | 🟠 P1 | ~5 min |
| [TASK-004](TASK-004-normalize-webhook-hook.md) | `WebhookEvents.tsx` | Webhooks `undefined.find` | 🟠 P1 | ~5 min |
| [TASK-005](TASK-005-normalize-rbac-hook.md) | `useRBACMatrix.ts` | RBAC React Error #31 | 🔴 P0 | ~15 min |
| [TASK-006](TASK-006-fix-systemhealth-crash.md) | `SystemHealth.tsx` | SystemHealth `undefined.includes` | 🟠 P1 | ~5 min |

## Nhóm B — API 404 (Tạo MSW mock handlers)

| Task | File mới | Bug | Priority | Effort |
|------|----------|-----|----------|--------|
| [TASK-007](TASK-007-msw-handlers-api404.md) | `src/mocks/handlers/*.ts` | 7 endpoints 404 | 🟡 P3 | ~30 min |

## Nhóm C — Server Errors (UI defensive + backend note)

| Task | File | Bug | Priority | Effort |
|------|------|-----|----------|--------|
| [TASK-008](TASK-008-findings-500-handling.md) | `findingApi.ts`, `FindingsList.tsx` | Findings 500 | 🔴 P0 | ~10 min |
| [TASK-009](TASK-009-ai-503-graceful.md) | `AITriage.tsx`, `AIEnrichment.tsx` | AI 503 | 🟡 P2 | ~10 min |

---

## Thứ tự thực thi đề xuất

```
1. TASK-005 (RBAC - P0, 26 errors)
2. TASK-002 (Assets - P0, crash)
3. TASK-008 (Findings 500 - P0)
4. TASK-001 (CWE - P1)
5. TASK-003 (Products - P1)
6. TASK-004 (Webhooks - P1)
7. TASK-006 (SystemHealth - P1)
8. TASK-009 (AI 503 - P2)
9. TASK-007 (MSW mocks - P3, sau khi backend sẵn sàng)
```
