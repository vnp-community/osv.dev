# UI API v3 — Bug Tracking Index

**Test run**: 2026-06-23  
**Server**: `https://c12.openledger.vn`  
**Kết quả**: 80/120 passed (**40 failures**)

## Danh Sách Bug Files

| File | BUG ID | Domain | Severity | Endpoints fail |
|---|---|---|---|---|
| [report.md](./report.md) | — | All | — | Tổng hợp toàn bộ |
| [BUG-014-profile-management.md](./BUG-014-profile-management.md) | BUG-014 | Profile | 🔴 CRITICAL | 6 |
| [BUG-006-findings-methods.md](./BUG-006-findings-methods.md) | BUG-006 | Findings | 🔴 HIGH | 5 |
| [BUG-011-ai-center.md](./BUG-011-ai-center.md) | BUG-011 | AI Center | 🔴 HIGH | 4 |
| [BUG-013-jira-integration.md](./BUG-013-jira-integration.md) | BUG-013 | Integrations | 🔴 HIGH | 4 |
| [BUG-002-notifications.md](./BUG-002-notifications.md) | BUG-002 | Notifications | 🔴 HIGH | 3 |
| [BUG-007-risk-acceptances.md](./BUG-007-risk-acceptances.md) | BUG-007 | Risk | 🔴 HIGH | 2 |
| [BUG-005-scans-history-import.md](./BUG-005-scans-history-import.md) | BUG-005 | Scanning | 🔴 HIGH | 2 |
| [BUG-008-017-remaining.md](./BUG-008-017-remaining.md) | BUG-008~017 | Multiple | 🟡 MEDIUM | 14 |
| [BUG-001-auth-mfa.md](./BUG-001-auth-mfa.md) | BUG-001 | Auth | 🟡 HIGH | 2 |

## Phân Loại Lỗi

```
404 Not Found (route chưa implement): 29 endpoints
405 Method Not Allowed (sai HTTP method): 11 endpoints
```

## Quick Fix Priority

**Sprint 1 (Critical)**:
1. `/api/v1/profile/*` — 6 endpoints (BUG-014)
2. `/api/v1/findings/{id}` PATCH, bulk ops — 5 endpoints (BUG-006)
3. `/api/v1/risk-acceptances` GET/POST — 2 endpoints (BUG-007)
4. `/api/v1/audit-log` — 1 endpoint (BUG-016)
5. `/api/v1/notifications` list, unread, mark-all — 3 endpoints (BUG-002)

**Sprint 2 (High)**:
6. AI Center queue, enrichment, insights — 4 endpoints (BUG-011)
7. JIRA integration — 4 endpoints (BUG-013)
8. Assets/Products PATCH — 2 endpoints (BUG-009)
9. Scans history — 1 endpoint (BUG-005)

**Sprint 3 (Medium)**:
10. MFA setup/confirm — 2 endpoints (BUG-001)
11. Search recent/suggested — 2 endpoints (BUG-017)
12. SLA config PUT — 1 endpoint (BUG-008)
13. Products grades — 1 endpoint (BUG-010)
14. Webhook stats — 1 endpoint (BUG-012)
15. Browse root, DBInfo — 2 endpoints (BUG-004)
16. Semantic suggestions — 1 endpoint (BUG-003)
17. Admin user detail — 1 endpoint (BUG-015)
