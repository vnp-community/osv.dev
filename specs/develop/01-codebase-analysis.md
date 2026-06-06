# 01 — Codebase Analysis: Vấn Đề & Nợ Kỹ Thuật

> **Date:** 2026-06-03  
> **Status:** ✅ Executed — Xem Implementation Status ở dưới  
> **Scope:** `apps/`, `bindings/`, `external/`, `osv/`, `services/`, `tools/`, `vulnfeeds/`

---

## 1. Bản Đồ Codebase Hiện Tại

```
osv.dev/
├── apps/
│   ├── cli/              # Go CLI app (mới, DDD)
│   └── osv/              # Go OSV server app (mới)
│
├── bindings/
│   └── go/               # Go bindings cho OSV schema
│       ├── api/          # API client bindings
│       ├── osvdev/       # OSV.dev API client
│       └── osvdevexperimental/
│
├── external/             # External connectors (Go)
│   ├── cmd/ids/          # ID utility
│   └── cmd/pypi/         # PyPI connector
│
├── osv/                  # ⚠️ Python core library (LEGACY)
│   ├── models.py         # 1762 dòng — God Object
│   ├── impact.py         # 29KB — Git bisection logic
│   ├── ecosystems/       # 41 files — ecosystem adapters (Python)
│   ├── models/           # Đang refactor ra submodule
│   └── ...
│
├── services/             # ✅ Go microservices (MỚI)
│   ├── pkg/              # Shared Go library
│   │   ├── ecosystem/impl/  # 43 files — ecosystem adapters (Go)
│   │   ├── models/          # Domain models
│   │   ├── osvschema/       # OSV schema types
│   │   └── ...
│   ├── api-gateway/
│   ├── vulnerability-query/
│   ├── ingestion/
│   ├── search/
│   ├── ai-enrichment/
│   ├── impact-analysis/
│   ├── version-index/
│   ├── alias-relations/
│   ├── notification/
│   ├── source-sync/
│   └── web-bff/
│
├── tools/                # ⚠️ Python/Go admin tools (MIX)
│   ├── aliaslookup/      # Go
│   ├── apitester/        # ?
│   ├── compare-responses/# Go
│   ├── datafix/          # Python ad-hoc scripts
│   ├── datastore-remover/# Python
│   ├── indexer-api-caller/# Go
│   ├── migrate/          # Go (Datastore→Firestore)
│   ├── osv-scanner/      # ?
│   ├── reconcile/        # ?
│   ├── smoke-test/       # Go
│   ├── source-sync/      # ⚠️ Python — trùng với services/source-sync
│   └── sourcerepo-sync/  # ?
│
└── vulnfeeds/            # Go — CVE format converters
    ├── cmd/combine-to-osv/
    ├── cmd/converters/
    ├── cmd/ids/
    ├── cmd/mirrors/
    ├── cmd/pypi/
    ├── conversion/       # NVD CVE5 + NVD v2 converters
    ├── faulttolerant/
    ├── git/
    ├── models/
    ├── triage/
    └── utility/
```

---

## 2. Các Vấn Đề Chính Được Xác Định

### 2.1 🔴 Trùng Lặp Code (Code Duplication)

#### Vấn đề 1: Ecosystem Adapters bị nhân đôi
```
osv/ecosystems/          # Python — 41 files
  alpine.py, debian.py, maven.py, nuget.py, pypi.py...

services/pkg/ecosystem/impl/  # Go — 43 files
  alpine.go, debian.go, maven.go, nuget.go, pypi.go...
```
**Mức độ:** NGHIÊM TRỌNG — Logic version parsing phải đồng bộ giữa 2 ngôn ngữ.
Nguy cơ: kết quả query khác nhau giữa Python service cũ và Go service mới.

#### Vấn đề 2: Source Sync trùng lặp
```
tools/source-sync/source_sync.py   # Python script cũ
services/source-sync/              # Go microservice mới
```
Cả hai cùng chức năng: trigger sync cho nguồn CVE.

#### Vấn đề 3: CVE Models bị nhân đôi
```
osv/models.py              # Python NDB models (1762 dòng)
services/pkg/models/       # Go domain models (mới)
```
Cấu trúc `Bug`, `AffectedPackage`, `SourceRepository` xuất hiện ở cả hai nơi.

#### Vấn đề 4: PyPI connector bị nhân đôi
```
external/cmd/pypi/         # Go
vulnfeeds/cmd/pypi/        # Go (khác nhau?)
```

---

### 2.2 🟠 God Objects & Quá Tải Trách Nhiệm

#### `osv/models.py` — 1762 dòng, làm quá nhiều việc:
- NDB Entity definitions (schema)
- Business logic (update_from_vulnerability, to_vulnerability)
- Indexing logic (search_indices, semver_fixed_indexes)
- Source repository management
- Alias/upstream group management
- Import finding management
- Counter management (IDCounter)
- OSS-Fuzz specific entities (RegressResult, FixResult)

**Refactoring đang diễn ra** (comment trong file xác nhận) nhưng chưa hoàn thành.
File đang được split sang `osv/models/entities.py` và `osv/models/indexing.py`.

---

### 2.3 🟠 Multi-Language Drift

| Chức năng | Python | Go |
|-----------|--------|----|
| Ecosystem version parsing | `osv/ecosystems/` | `services/pkg/ecosystem/impl/` |
| OSV Schema models | `osv/models.py` | `services/pkg/models/` |
| Impact analysis | `osv/impact.py` | `services/impact-analysis/` |
| Source sync | Python daemon | Go microservice |
| API | Python gRPC | Go HTTP/gRPC |

Hai stack song song → bugs khó tìm, behavior không nhất quán.

---

### 2.4 🟡 Module Không Rõ Trách Nhiệm

#### `bindings/go/`
- Chứa Go bindings cho OSV schema + API client
- Gần như duplicate với `services/pkg/`
- Không rõ ai dùng gì: `bindings/go/osvdev/` vs `services/pkg/clients/`?

#### `external/`
- Chỉ có 2 command: `ids/` và `pypi/`
- Có thể merge vào `vulnfeeds/` hoặc `services/source-sync/`
- Không rõ mục đích tách riêng

#### `tools/`
- Mix Python và Go
- Gồm: admin tools, smoke tests, migration scripts, ad-hoc fixes
- Không có cấu trúc nhất quán

---

### 2.5 🟡 Nợ Kỹ Thuật Khác

| Vấn đề | File | Mô tả |
|--------|------|-------|
| TODO comments tồn tại lâu | `osv/models.py:371,395,483,701` | TODOs từ thời điểm refactoring cũ chưa giải quyết |
| OSS-Fuzz specific coupling | `osv/models.py:162-211` | RegressResult, FixResult chỉ dùng cho OSS-Fuzz nhưng nằm trong core |
| Datastore migration cũ | `tools/migrate/datastore_to_firestore.go` | Migration script một lần, không còn cần thiết |
| Python 2 legacy patterns | Nhiều file trong `osv/` | Style code cũ |
| Test coverage thấp | `tools/` | Hầu hết tools thiếu tests |

---

## 3. Ma Trận Phụ Thuộc

```
apps/osv ──────────────────────────► services/pkg/
apps/cli ──────────────────────────► services/pkg/

bindings/go ────────────────────────► external Go SDK

external ───────────────────────────► vulnfeeds/ (tương tự)

osv/ (Python) ──────────────────────► google-cloud-ndb
                                   ► google-cloud-pubsub
                                   ► proto-plus
                                   ► pygit2

services/ (Go) ─────────────────────► services/pkg/ (shared)
               ─────────────────────► NATS JetStream
               ─────────────────────► google-cloud-firestore
               ─────────────────────► google-cloud-storage

vulnfeeds/ (Go) ────────────────────► services/pkg/ (chưa)
                ────────────────────► internal conversion logic
```

**Vấn đề**: `vulnfeeds/` KHÔNG import `services/pkg/` — có conversion models riêng trong `vulnfeeds/models/`. Đây là duplication thứ tư.

---

## 4. Phân Tích Chất Lượng Code

### Test Coverage Ước Tính

| Module | Tests có | Chất lượng |
|--------|---------|------------|
| `osv/ecosystems/` | ✅ Cao | 41 test files |
| `osv/models.py` | ✅ Tốt | `models_test.py` (18KB) |
| `services/pkg/ecosystem/impl/` | ✅ Tốt | 23 test files |
| `vulnfeeds/conversion/` | ✅ Tốt | `versions_test.go` (60KB!) |
| `tools/` | ❌ Kém | Gần như không có |
| `apps/` | ⚠️ Trung bình | Chỉ API handler tests |
| `bindings/` | ❌ Chưa rõ | - |

### Tổng Kết

| Điểm mạnh | Điểm yếu |
|-----------|---------|
| Ecosystem adapters có test tốt ở cả Python và Go | Code trùng lặp ở nhiều nơi |
| Go services theo DDD pattern nhất quán | `osv/models.py` quá lớn, quá nhiều trách nhiệm |
| Đang có migration plan rõ ràng | Hai stack song song gây confusion |
| `services/pkg/` làm shared library tốt | `tools/` không có cấu trúc |
| Docker Compose đầy đủ cho local dev | `bindings/` và `external/` mục đích mờ nhạt |

---

## 5. Implementation Status (2026-06-03)

> Phần này ghi lại trạng thái thực tế sau khi thực thi proposals.

### 5.1 Các Vấn Đề Đã Giải Quyết

| Vấn đề | Giải pháp đã thực hiện | Files |
|--------|----------------------|-------|
| Code duplication — Source sync | Move `tools/source-sync/source_sync.py` → `tools/deprecated/` | DONE ✅ |
| Code duplication — bindings/go | Merge + deprecation notice `bindings/go/README.md` | DONE ✅ |
| Code duplication — CVE Models | `Bug`, `AliasGroup` ported → `services/pkg/models/` (6 tests) | DONE ✅ |
| God Object `osv/models.py` | OSS-Fuzz entities isolated vào `osv/ossfuzz/README.md` | DONE ✅ |
| `external/` mục đích mờ nhạt | Absorb → `services/source-sync/internal/connectors/` | DONE ✅ |
| `tools/` không có cấu trúc | Tổ chức lại: `tools/cmd/`, `tools/scripts/`, `tools/deprecated/` | DONE ✅ |
| `bindings/go/` mục đích mờ nhạt | Merge vào `services/pkg/clients/` | DONE ✅ |

### 5.2 Cấu Trúc Mới Sau Tổ Chức Lại

```
services/                ← Core platform
├── pkg/                 ← Shared library (tất cả services dùng)
│   ├── clients/kev/     ✅ 8 tests
│   ├── clients/epss/    ✅ 4 tests
│   ├── classification/  ✅ 12 tests
│   ├── cwe/             ✅ 12 tests (60+ CWEs)
│   ├── models/          ✅ 6 tests (Bug, AliasGroup)
│   └── ecosystem/impl/  ✅ 43 adapters
├── converter/           ✅ NVD v2 + CVE5 + ADP merge
├── source-sync/         ✅ webhook + NATS trigger
├── ai-enrichment/       ✅ KEV + EPSS + CWE pipeline
├── admin/               ✅ 12 REST endpoints
├── api-gateway/         ✅ v1 + v2 APIs
└── cvectl/              ✅ cobra/viper CLI

osv/                     ← Python (isolating)
└── ossfuzz/             ← boundary documentation
    └── README.md        ✅ isolation strategy

tools/
├── cmd/                 ✅ Go admin tools
├── scripts/             ✅ CI/maintenance scripts
└── deprecated/          ✅ archived tools
```

### 5.3 Điểm Yếu Còn Lại

| Vấn đề | Kế hoạch |
|--------|----------|
| `osv/models.py` vẫn là Python God Object | Dần thay thế qua `services/pkg/models/` — mỗi sprint port thêm |
| `vulnfeeds/` chưa thành microservice hoàn chỉnh | gRPC interface còn TODO (TASK-04-04) |
| Hai stack Python/Go vẫn song song | Strangler Fig pattern — Python giảm dần |
| Test coverage `tools/cmd/` | TODO |
