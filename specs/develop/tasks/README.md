# Tasks Index — CVE Platform Development

> **Dựa trên:** [specs/develop/](../)  
> **Cập nhật:** 2026-06-03  
> **Mô hình:** Sprint-based, mỗi sprint = 2 tuần

---

## Tổng Quan Backlog

| Sprint | File | Chủ đề | Ưu tiên |
|--------|------|--------|---------|
| [SPRINT-01](./SPRINT-01-foundation.md) | Foundation & Cleanup | Dọn dẹp legacy, tổ chức lại codebase | 🔴 P0 |
| [SPRINT-02](./SPRINT-02-pkg-shared.md) | Shared Library Enhancement | Mở rộng `services/pkg/` | 🔴 P0 |
| [SPRINT-03](./SPRINT-03-source-sync.md) | Source Sync Enhancement | Webhook, credential manager, admin API | 🔴 P0 |
| [SPRINT-04](./SPRINT-04-converter-svc.md) | Converter Service | Port `vulnfeeds/` → gRPC microservice | 🟠 P1 |
| [SPRINT-05](./SPRINT-05-ai-enrichment.md) | AI Enrichment Enhancement | KEV/EPSS/CWE integration pipeline | 🟠 P1 |
| [SPRINT-06](./SPRINT-06-admin-svc.md) | Admin Service | Admin API handlers + data quality | 🟠 P1 |
| [SPRINT-07](./SPRINT-07-search.md) | Search Enhancement | Semantic search, faceted, saved alerts | 🟡 P2 |
| [SPRINT-08](./SPRINT-08-api-v2.md) | API v2 & CLI | Extended API, API key mgmt, CLI commands | 🟡 P2 |
| [SPRINT-09](./SPRINT-09-go-migration.md) | Go Migration — Phase 1 | `osv/ecosystems/` parity, `osv/models.py` port | 🟡 P2 |
| [SPRINT-10](./SPRINT-10-go-migration-p2.md) | Go Migration — Phase 2 | `osv/impact.py`, `osv/sources.py` port | 🔵 P3 |

---

## Priority Legend

| Symbol | Ý nghĩa |
|--------|---------|
| 🔴 P0 | Critical — ảnh hưởng data freshness, security |
| 🟠 P1 | High — cần cho roadmap Q4 2026 |
| 🟡 P2 | Medium — cải thiện đáng kể |
| 🔵 P3 | Low — tốt nếu có |

---

## Trạng Thái Tổng Quát

```
COMPLETED:
  ✅ SPRINT-01: Foundation cleanup, bindings/ merge, tools/ reorganize
  ✅ SPRINT-02: pkg/kev (8), pkg/epss (4), pkg/classification (12), pkg/cwe (12), pkg/models (6)
  ✅ SPRINT-03: Webhook handler (GitHub + GitLab), NATS trigger, ConfigSourceResolver
  ✅ SPRINT-04: NVD converter, CVE5 converter + ADP merging (8 tests), NATS publisher
  ✅ SPRINT-05: KEV+EPSS threat intel pipeline, CWE enrichment stage
  ✅ SPRINT-06: Admin handlers (12 endpoints): sources, findings, vulns, health, API keys
  ✅ SPRINT-07: Search entity update (entity.go + OpenSearch mapping)
  ✅ SPRINT-08: API v2 endpoints (enrichment, related, timeline, batch), cvectl CLI
  ✅ SPRINT-09: Bug struct port, AliasGroup port, ecosystem parity tests
  ✅ SPRINT-10 (partial): OSS-Fuzz isolation README, search entity enrichment fields

IN PROGRESS:
  🔄 SPRINT-10: osv/impact.py port (TASK-10-01), osv/sources.py port (TASK-10-02)
  🔄 SPRINT-02/09: Ecosystem parity testdata collection

TODO (High Priority):
  📋 CPE → version detection (TASK-04-03, 5d)
  📋 gRPC converter interface (TASK-04-04, 2d)
  📋 Semantic/vector search (TASK-07-02, 3d)
  📋 Credential manager (TASK-03-02, 3d)
  📋 Impact analysis Go port (TASK-10-01, 7d)
  📋 CI/CD update (TASK-01-06, 0.5d)

Build Status (2026-06-03):
  ✅ pkg, converter, admin, ai-enrichment, source-sync, impact-analysis, api-gateway, cvectl
  ✅ Tests: 50+ PASS (kev:8, epss:4, classification:12, cwe:12, models:6, cve5:8)
```

---

## Links

- [01-codebase-analysis.md](../01-codebase-analysis.md)
- [02-reorganization.md](../02-reorganization.md)
- [03-deprecation.md](../03-deprecation.md)
- [04-roadmap.md](../04-roadmap.md)
- [05-go-migration.md](../05-go-migration.md)
- [06-new-features.md](../06-new-features.md)
