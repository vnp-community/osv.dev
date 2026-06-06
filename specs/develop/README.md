# OSV.dev — Development Proposals

> **Date:** 2026-06-03  
> **Status:** ✅ In Execution (Sprint 1-10)  
> **Last Review:** 2026-06-03  
> **Scope:** Toàn bộ codebase: `apps/`, `bindings/`, `external/`, `osv/`, `services/`, `tools/`, `vulnfeeds/`

---

## Tài Liệu Trong Thư Mục Này

| # | Tài liệu | Nội dung | Status |
|---|----------|---------|----|
| 01 | [Codebase Analysis](./01-codebase-analysis.md) | Phân tích tổng quan codebase hiện tại: vấn đề, nợ kỹ thuật, trùng lặp | ✅ Đã thực hiện |
| 02 | [Reorganization Plan](./02-reorganization.md) | Tổ chức lại code — gộp, tách, đổi tên các module | ✅ Đã thực hiện |
| 03 | [Deprecation List](./03-deprecation.md) | Danh sách code cần xóa hoặc deprecate | 🔄 Đang thực hiện |
| 04 | [Development Roadmap](./04-roadmap.md) | Hướng phát triển cho từng thành phần | 🔄 Đang thực hiện |
| 05 | [Go Migration Strategy](./05-go-migration.md) | Lộ trình hoàn thành migration Python → Go | 🔄 Đang thực hiện |
| 06 | [New Features](./06-new-features.md) | Các tính năng mới đề xuất phát triển | 🔄 Đang thực hiện |

---

## Tóm Tắt Đề Xuất

### 🔴 Xóa Bỏ (Deprecate/Remove) — Trạng thái thực tế

| Item | Đề xuất | Trạng thái |
|------|---------|-----------|
| `tools/source-sync/source_sync.py` | Xóa/move deprecated | ✅ Đã move → `tools/deprecated/` |
| `tools/migrate/` | Xóa (one-time script) | ✅ Đã move → `tools/deprecated/` |
| `tools/datastore-remover/` | Xóa | ✅ Đã move → `tools/deprecated/` |
| `bindings/go/` | Merge vào `services/pkg/` | ✅ Đã merge + deprecation notice |
| `external/` → `source-sync/connectors/` | Merge connectors | ✅ Đã tích hợp |
| `osv/` Python core | Deprecated dần (Strangler Fig) | 🔄 OSS-Fuzz isolated |

### 🟡 Tổ Chức Lại (Reorganize) — Trạng thái thực tế

| Item | Đề xuất | Trạng thái |
|------|---------|-----------|
| `vulnfeeds/` → `services/converter/` | Chuyển thành microservice Go | ✅ converter/ tạo mới, NVD+CVE5 converter done |
| `external/` → `services/source-sync/` | Tích hợp external connectors | ✅ connectors/ absorb |
| `tools/` → `cmd/` + `services/admin/` | Tổ chức lại | ✅ tools/cmd/ + tools/scripts/ |
| `apps/osv/` → `services/api-gateway/` | Hợp nhất vào gateway | ✅ v2 handlers done |

### 🟢 Phát Triển Thêm (Develop) — Trạng thái thực tế

| Item | Đề xuất | Trạng thái |
|------|---------|-----------|
| `services/pkg/ecosystem/` | Hoàn thiện 30+ ecosystem adapters | ✅ 43 impls, parity tests |
| `services/ai-enrichment/` | KEV, EPSS, CWE, auto-tagging | ✅ KEV+EPSS+CWE stages done |
| `services/source-sync/` | Webhook support, credential manager | ✅ Webhook done; credential TODO |
| `services/admin/` | Admin API + dashboard backend | ✅ 12 REST endpoints implemented |
| `services/converter/` | Unified CVE format converter | ✅ NVD v2 + CVE5 + ADP merge done |
| `services/pkg/cwe/` | CWE database package | ✅ 60+ CWEs, 12 tests pass |
| `services/pkg/models/` | Bug, AliasGroup entities | ✅ Bug + AliasGroup ported |
| `services/cvectl/` | CLI tool | ✅ cobra/viper CLI done |

---

## Implementation Progress (2026-06-03)

### Build Status
```
✅ ALL 8 SERVICES BUILD PASS:
   admin, ai-enrichment, api-gateway, converter,
   cvectl, impact-analysis, pkg, source-sync

✅ 50+ TESTS PASS:
   pkg/cwe (12)  pkg/models (6)  pkg/clients/kev (8)
   pkg/clients/epss (4)  pkg/classification (12)
   converter/domain/cve5 (8)
```

### Services Implemented
```
services/
├── pkg/
│   ├── clients/kev/        ✅ 8/8 tests
│   ├── clients/epss/       ✅ 4/4 tests
│   ├── classification/     ✅ 12/12 tests
│   ├── cwe/                ✅ 12/12 tests (NEW)
│   ├── models/             ✅ 6/6 tests (Bug + AliasGroup)
│   └── ecosystem/impl/     ✅ 43 adapters + parity tests
├── converter/              ✅ NVD v2 + CVE5 + ADP merge (8 tests)
├── source-sync/            ✅ webhook (GitHub+GitLab) + NATS
├── ai-enrichment/          ✅ KEV + EPSS + CWE pipeline
├── admin/                  ✅ 12 REST endpoints (handler.go)
├── api-gateway/            ✅ v2 endpoints + Firestore store
├── cvectl/                 ✅ cobra/viper CLI
└── impact-analysis/        🔄 bisector (partial)
```

### Remaining High-Priority Tasks
| Task | Effort | Note |
|------|--------|------|
| CPE → version detection (TASK-04-03) | 5d | Complex |
| gRPC converter interface (TASK-04-04) | 2d | Proto + server |
| Semantic/vector search (TASK-07-02) | 3d | OpenSearch k-NN |
| Credential manager (TASK-03-02) | 3d | GCP Secret Manager |
| Port osv/impact.py → Go (TASK-10-01) | 7d | git2go needed |
| CI/CD pipeline update (TASK-01-06) | 0.5d | GitHub Actions |

### Detailed Task Tracking
→ Xem [tasks/BACKLOG.md](./tasks/BACKLOG.md) và các SPRINT files trong [tasks/](./tasks/).
