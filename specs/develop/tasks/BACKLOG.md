# BACKLOG — Tổng Hợp Tất Cả Tasks

> **Cập nhật:** 2026-06-03 (Sprint 10 — 100% COMPLETE 🎉)  
> **Format:** [SPRINT-TASK] Task name | Effort | Status

---

## 🔴 P0 — Critical (Làm ngay)

| ID | Task | Effort | Status |
|----|------|--------|--------|
| [01-01](./SPRINT-01-foundation.md#task-01-01) | Xóa/Archive legacy tools | 0.5d | ✅ DONE |
| [01-02](./SPRINT-01-foundation.md#task-01-02) | Tổ chức lại `tools/` | 0.5d | ✅ DONE |
| [01-03](./SPRINT-01-foundation.md#task-01-03) | Merge `bindings/go/` → `pkg/clients/` | 1d | ✅ DONE |
| [01-06](./SPRINT-01-foundation.md#task-01-06) | Update CI/CD pipelines | 0.5d | ✅ DONE ← NEW |
| [02-01](./SPRINT-02-pkg-shared.md#task-02-01) | KEV Client pkg/clients/kev/ | 2d | ✅ DONE |
| [02-02](./SPRINT-02-pkg-shared.md#task-02-02) | EPSS Client pkg/clients/epss/ | 1.5d | ✅ DONE |
| [02-03](./SPRINT-02-pkg-shared.md#task-02-03) | Classification pkg/classification/ | 2d | ✅ DONE |
| [02-04](./SPRINT-02-pkg-shared.md#task-02-04) | CWE Database pkg/cwe/ | 3d | ✅ DONE |
| [02-05](./SPRINT-02-pkg-shared.md#task-02-05) | Ecosystem parity audit | 2d | ✅ DONE |
| [03-01](./SPRINT-03-source-sync.md#task-03-01) | Wire webhook handler | 1d | ✅ DONE |
| [03-02](./SPRINT-03-source-sync.md#task-03-02) | Credential manager | 3d | ✅ DONE |
| [09-01](./SPRINT-09-go-migration.md#task-09-01) | Ecosystem parity test suite | 3d | ✅ DONE |
| [10-06](./SPRINT-10-go-migration-p2.md#task-10-06) | Final validation | 2d | ✅ DONE ← NEW |

---

## 🟠 P1 — High (Sprint 3-6)

| ID | Task | Effort | Status |
|----|------|--------|--------|
| [01-04](./SPRINT-01-foundation.md#task-01-04) | Merge `external/` → source-sync | 1.5d | ✅ DONE |
| [02-06](./SPRINT-02-pkg-shared.md#task-02-06) | EPSS Daily Update Job | 1d | ✅ DONE |
| [03-01a](./SPRINT-03-source-sync.md#task-03-01a) | SourceResolver implementation | 1d | ✅ DONE |
| [03-01b](./SPRINT-03-source-sync.md#task-03-01b) | SyncTrigger via NATS | 0.5d | ✅ DONE |
| [03-01c](./SPRINT-03-source-sync.md#task-03-01c) | Register webhook routes | 0.5d | ✅ DONE |
| [03-03](./SPRINT-03-source-sync.md#task-03-03) | Source Admin API | 2d | ✅ DONE |
| [04-01](./SPRINT-04-converter-svc.md#task-04-01) | CVE5 converter + ADP merging | 2d | ✅ DONE |
| [04-02](./SPRINT-04-converter-svc.md#task-04-02) | NVD JSON v2 converter | 3d | ✅ DONE |
| [04-03](./SPRINT-04-converter-svc.md#task-04-03) | Version detection từ CPE | 5d | ✅ DONE |
| [04-04](./SPRINT-04-converter-svc.md#task-04-04) | gRPC service interface | 2d | ✅ DONE |
| [04-05](./SPRINT-04-converter-svc.md#task-04-05) | NATS Event Publisher | 1d | ✅ DONE |
| [05-01](./SPRINT-05-ai-enrichment.md#task-05-01) | Wire threat intel pipeline | 1d | ✅ DONE |
| [05-02](./SPRINT-05-ai-enrichment.md#task-05-02) | CWE Enrichment Stage | 2d | ✅ DONE |
| [05-04](./SPRINT-05-ai-enrichment.md#task-05-04) | Auto-tagging enhancement | 3d | ✅ DONE |
| [05-05](./SPRINT-05-ai-enrichment.md#task-05-05) | Daily EPSS batch update | 1.5d | ✅ DONE |
| [05-06](./SPRINT-05-ai-enrichment.md#task-05-06) | High-risk CVE alerts | 1d | ✅ DONE |
| [06-01](./SPRINT-06-admin-svc.md#task-06-01) | Source management handlers | 3d | ✅ DONE |
| [06-02](./SPRINT-06-admin-svc.md#task-06-02) | Import findings handlers | 2d | ✅ DONE |
| [06-03](./SPRINT-06-admin-svc.md#task-06-03) | Vulnerability admin operations | 2d | ✅ DONE |
| [06-06](./SPRINT-06-admin-svc.md#task-06-06) | API key management | 2d | ✅ DONE |
| [06-07](./SPRINT-06-admin-svc.md#task-06-07) | System health endpoint | 1d | ✅ DONE |
| [07-01](./SPRINT-07-search.md#task-07-01) | OpenSearch index mapping update | 1d | ✅ DONE |
| [07-02](./SPRINT-07-search.md#task-07-02) | Semantic/vector search | 3d | ✅ DONE |
| [07-03](./SPRINT-07-search.md#task-07-03) | Faceted search + aggregations | 2d | ✅ DONE |
| [08-01](./SPRINT-08-api-v2.md#task-08-01) | API v2 endpoints | 4d | ✅ DONE |
| [08-03](./SPRINT-08-api-v2.md#task-08-03) | Rate limiting + quota | 2d | ✅ DONE |
| [09-02](./SPRINT-09-go-migration.md#task-09-02) | Port osv/models.py Bug struct | 5d | ✅ DONE |
| [09-04](./SPRINT-09-go-migration.md#task-09-04) | Port AliasGroup entity | 1d | ✅ DONE |
| [09-05](./SPRINT-09-go-migration.md#task-09-05) | Traffic split — ingestion | 2d | ✅ DONE |
| [10-01](./SPRINT-10-go-migration-p2.md#task-10-01) | Port osv/impact.py (RangeCollector + Analyzer) | 7d | ✅ DONE ← NEW |
| [10-02](./SPRINT-10-go-migration-p2.md#task-10-02) | Port osv/sources.py | 4d | ✅ DONE ← NEW |
| [10-03](./SPRINT-10-go-migration-p2.md#task-10-03) | Isolate OSS-Fuzz Python | 2d | ✅ DONE |

---

## 🟡 P2 — Medium (Sprint 7-10)

| ID | Task | Effort | Status |
|----|------|--------|--------|
| [01-05](./SPRINT-01-foundation.md#task-01-05) | Review osv-scanner, sourcerepo-sync | 0.5d | ✅ DONE |
| [02-03a](./SPRINT-02-pkg-shared.md#task-02-03a) | CWE-based tags | 1d | ✅ DONE |
| [02-03b](./SPRINT-02-pkg-shared.md#task-02-03b) | Package-based tags | 1d | ✅ DONE |
| [03-04](./SPRINT-03-source-sync.md#task-03-04) | Smart scheduling | 2d | ✅ DONE ← NEW |
| [04-06](./SPRINT-04-converter-svc.md#task-04-06) | Vulnfeeds CLI adapter (`convert cve5/nvd/batch`) | 2d | ✅ DONE ← NEW |
| [04-07](./SPRINT-04-converter-svc.md#task-04-07) | Migrate vulnfeeds/ commands | 2d | ✅ DONE ← NEW |
| [05-03](./SPRINT-05-ai-enrichment.md#task-05-03) | Exploit availability check | 3d | ✅ DONE |
| [06-04](./SPRINT-06-admin-svc.md#task-06-04) | Data quality monitoring | 3d | ✅ DONE |
| [06-05](./SPRINT-06-admin-svc.md#task-06-05) | Audit trail | 1.5d | ✅ DONE |
| [07-04](./SPRINT-07-search.md#task-07-04) | Saved searches + alerts | 4d | ✅ DONE ← NEW |
| [08-02](./SPRINT-08-api-v2.md#task-08-02) | cvectl CLI enhancement | 4d | ✅ DONE |
| [09-03](./SPRINT-09-go-migration.md#task-09-03) | Port SourceRepository entity | 2d | ✅ DONE |
| [09-06](./SPRINT-09-go-migration.md#task-09-06) | Cross-language test report | 1d | ✅ DONE ← NEW |
| [10-04](./SPRINT-10-go-migration-p2.md#task-10-04) | Python codebase cleanup | 2d | ✅ DONE ← NEW |
| [10-05](./SPRINT-10-go-migration-p2.md#task-10-05) | Migration metrics dashboard | 1d | ✅ DONE ← NEW |

---

## 🔵 P3 — Low (Follow-up)

| ID | Task | Effort | Status |
|----|------|--------|--------|
| [05-04d](./SPRINT-05-ai-enrichment.md#task-05-04d) | LLM-based tags (MITRE ATT&CK) | 3d | ✅ DONE ← NEW |
| [08-04](./SPRINT-08-api-v2.md#task-08-04) | Local dev environment improvements | 1d | ✅ DONE |

---

## Effort Summary (Final — 2026-06-03)

| Category | Total Estimate |
|----------|----------------|
| ✅ Completed | **~191 days (ALL tasks)** |
| 🔄 In Progress | 0 |
| 📋 Remaining | **0** |
| **Completion** | **100% 🎉** |

---

## Final Progress Overview (2026-06-03)

### ✅ All Services Complete
```
services/
├── pkg/
│   ├── clients/kev/             ✅ 8/8 tests
│   ├── clients/epss/            ✅ 4/4 tests
│   ├── classification/          ✅ 12/12 tests
│   ├── classification/tagging/  ✅ 18/18 tests (CWE+package)
│   ├── cwe/                     ✅ 12/12 tests
│   ├── models/                  ✅ 6/6 tests
│   ├── search/semantic/         ✅ DONE
│   ├── search/faceted/          ✅ DONE
│   └── search/savedsearch/      ✅ 9/9 tests ← NEW
├── converter/
│   ├── domain/cve5/             ✅ 8/8 tests
│   ├── domain/nvd/              ✅ DONE
│   ├── domain/cpe/              ✅ 10/10 tests
│   └── interface/grpc/          ✅ DONE (proto + server)
├── source-sync/
│   ├── infra/webhook/           ✅ DONE
│   ├── infra/credential/        ✅ 11/11 tests
│   ├── infra/scheduler/         ✅ 7/7 tests ← NEW (smart scheduling)
│   └── application/sourcesloader/ ✅ 12/12 tests ← NEW (osv/sources.py port)
├── ai-enrichment/
│   ├── threatintel pipeline     ✅ KEV/EPSS/CWE stages
│   ├── exploit/checker          ✅ DONE
│   ├── epss_daily_update/       ✅ DONE
│   └── mitretagger/             ✅ 9/9 tests ← NEW (LLM+rule-based)
├── admin/
│   ├── handlers (12 REST)       ✅ DONE
│   ├── dataquality/             ✅ 7/7 tests
│   └── infra/audit/             ✅ 8/8 tests
├── api-gateway/
│   ├── v2 endpoints             ✅ DONE
│   └── infra/ratelimit/         ✅ 8/8 tests
├── cvectl/
│   ├── sources/vuln/admin cmds  ✅ DONE
│   └── internal/convert/        ✅ DONE ← NEW (cve5/nvd/batch)
└── impact-analysis/
    ├── bisector                 ✅ DONE
    ├── domain/service/analyzer  ✅ 7/7 tests ← NEW (osv/impact.py port)
    └── domain/service/rangecollector ✅ 7/7 tests ← NEW
```

### 📊 Aggregated Test Counts
| Sprint/Batch | New Tests |
|---|---|
| Previous sessions | 70 |
| This session (final) | 92 |
| **TOTAL** | **162** |

### 📁 Docs & CI/CD Added
- `.github/workflows/ci-services.yml` — full CI/CD pipeline
- `docs/cross-language-test-report.md` — Go vs Python parity report
- `docs/migration-metrics.md` — migration dashboard  
- `docs/python-cleanup-plan.md` — deprecation plan

---

## Gantt Overview (Final)

```
Q3 2026 (Months 1-3):    Foundation + pkg/ + source-sync  ✅ DONE
  Sprint 01: Foundation & Cleanup                          ✅
  Sprint 02: Shared Library (pkg/)                         ✅
  Sprint 03: Source Sync Enhancement                       ✅

Q4 2026 (Months 4-6):    Converter + AI Enrichment + Admin ✅ DONE
  Sprint 04: Converter Service                             ✅
  Sprint 05: AI Enrichment                                 ✅
  Sprint 06: Admin Service                                 ✅

Q1 2027 (Months 7-9):    Search + API v2 + CLI            ✅ DONE
  Sprint 07: Search Enhancement                            ✅
  Sprint 08: API v2 & CLI                                  ✅

Q2 2027 (Months 10-12):  Go Migration                     ✅ DONE
  Sprint 09: Go Migration Phase 1                          ✅
  Sprint 10: Go Migration Phase 2                          ✅ 100%
```
