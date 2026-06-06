# 02 — Reorganization Plan: Tổ Chức Lại Code

> **Date:** 2026-06-03  
> **Status:** ✅ Executed — Xem Implementation Status ở dưới  
> **Goal:** Loại bỏ trùng lặp, làm rõ trách nhiệm từng module, chuẩn bị nền tảng cho phát triển dài hạn

---

## 1. Cấu Trúc Mục Tiêu (Target Structure)

```
osv.dev/
│
├── apps/                   # Entrypoints — không thay đổi
│   ├── cli/                # Go CLI tool (giữ nguyên)
│   └── osv/                # Go OSV server (giữ nguyên)
│
├── services/               # ✅ Core microservices (Go) — MỞ RỘNG
│   ├── pkg/                # Shared library — TĂNG CƯỜNG
│   ├── api-gateway/        # Giữ nguyên
│   ├── vulnerability-query/# Giữ nguyên
│   ├── ingestion/          # Giữ nguyên
│   ├── search/             # Giữ nguyên
│   ├── ai-enrichment/      # Giữ nguyên
│   ├── impact-analysis/    # Giữ nguyên
│   ├── version-index/      # Giữ nguyên
│   ├── alias-relations/    # Giữ nguyên
│   ├── notification/       # Giữ nguyên
│   ├── source-sync/        # Giữ nguyên + absorb external/
│   ├── web-bff/            # Giữ nguyên
│   ├── converter/          # 🆕 Từ vulnfeeds/ → microservice
│   └── admin/              # 🆕 Admin API + dashboard backend
│
├── tools/                  # Admin & dev tools — TỔ CHỨC LẠI
│   ├── cmd/                # Go CLI admin tools
│   │   ├── aliaslookup/
│   │   ├── apitester/
│   │   ├── reconcile/
│   │   └── smoke-test/
│   └── scripts/            # One-off Python/Shell scripts
│       └── datafix/        # Ad-hoc data fixes (không xóa, archive)
│
├── proto/                  # Protocol Buffers (giữ nguyên)
│
├── docs/                   # Documentation (giữ nguyên)
│
└── specs/                  # Specifications (giữ nguyên)

# === ĐƯỢC XÓA/MERGE ===
# bindings/go/    → services/pkg/ (merge)
# external/       → services/source-sync/ (merge)
# osv/ (Python)   → Deprecated dần (xem 05-go-migration.md)
# vulnfeeds/      → services/converter/ (chuyển đổi thành microservice)
# tools/migrate/  → scripts/ (archive, không còn cần)
# tools/source-sync/ → services/source-sync/ (đã replaced)
```

---

## 2. Chi Tiết Từng Thay Đổi

### 2.1 `bindings/go/` → Merge vào `services/pkg/`

**Hiện tại:**
```
bindings/go/
├── api/                 # HTTP API types
├── osvdev/              # OSV.dev client (v1)
└── osvdevexperimental/  # Experimental API client
```

**Đề xuất:**
```
services/pkg/
└── clients/
    ├── osvdev/          # ← từ bindings/go/osvdev/
    └── osvdevexperimental/ # ← từ bindings/go/osvdevexperimental/
```

**Lý do:**
- `services/pkg/clients/` đã có `gcs.go`, `cloudstorage.go`, `pubsub.go`
- `bindings/go/` là wrapper trên `osvschema` extern — hợp nhất tự nhiên
- Giảm số lượng `go.mod` riêng biệt

**Action:**
```bash
# Move và update imports
mv bindings/go/osvdev/* services/pkg/clients/osvdev/
mv bindings/go/osvdevexperimental/* services/pkg/clients/osvdevexperimental/
# Update go.work
# Deprecate bindings/go/
```

---

### 2.2 `external/` → Merge vào `services/source-sync/`

**Hiện tại:**
```
external/
├── cmd/ids/    # ID lookup utility
└── cmd/pypi/   # PyPI feed connector
```

**Đề xuất:**
```
services/source-sync/
└── internal/
    └── connectors/
        ├── ids/     # ← từ external/cmd/ids/
        └── pypi/    # ← từ external/cmd/pypi/ (merge với vulnfeeds/cmd/pypi/)
```

**Lý do:**
- `external/` chỉ có 2 commands, không đủ tầm để là package riêng
- Cả hai đều là "external connectors" — phù hợp với `source-sync`
- Giảm số lượng Go module riêng biệt

---

### 2.3 `vulnfeeds/` → `services/converter/`

**Hiện tại:**
```
vulnfeeds/             # Standalone Go tool
├── cmd/
│   ├── combine-to-osv/   # Tổng hợp CVE5 + NVD → OSV
│   ├── converters/       # Individual format converters
│   ├── ids/              # ID management
│   ├── mirrors/          # Mirror management
│   └── pypi/             # PyPI specific converter
├── conversion/           # Conversion logic
│   ├── cve5/             # CVE5 format parser
│   ├── nvd/              # NVD JSON v2 parser
│   └── versions.go       # Version detection (44KB!)
├── faulttolerant/        # Retry/fault tolerance
├── git/                  # Git utilities
└── models/               # Data models (riêng biệt!)
```

**Đề xuất:**
```
services/converter/       # 🆕 Go microservice
├── Dockerfile
├── Makefile
├── cmd/
│   └── main.go           # gRPC server entrypoint
├── config/
├── interface/
│   └── proto/            # gRPC service definition
├── internal/
│   ├── application/      # Use cases
│   ├── domain/
│   │   ├── cve5/         # ← từ vulnfeeds/conversion/cve5/
│   │   ├── nvd/          # ← từ vulnfeeds/conversion/nvd/
│   │   └── versions/     # ← từ vulnfeeds/conversion/versions.go
│   └── infra/
│       └── git/          # ← từ vulnfeeds/git/
└── go.mod
```

**Lý do:**
- `vulnfeeds/` hiện là CLI batch tool; phù hợp hơn khi là microservice gRPC
- Tích hợp với pipeline qua NATS/events thay vì cron job
- `vulnfeeds/models/` trùng với `services/pkg/models/` → sẽ dùng chung
- `vulnfeeds/conversion/versions.go` (44KB) là core logic, cần expose qua API

**gRPC API đề xuất:**
```protobuf
service ConverterService {
  rpc ConvertCVE5(ConvertCVE5Request) returns (Vulnerability);
  rpc ConvertNVD(ConvertNVDRequest) returns (Vulnerability);
  rpc BatchConvert(BatchConvertRequest) returns (stream Vulnerability);
}
```

---

### 2.4 `tools/` → Tổ Chức Lại

**Hiện tại (hỗn độn):**
```
tools/
├── aliaslookup/          # Go — vẫn cần
├── apitester/            # ? — cần review
├── compare-responses/    # Go — dev tool
├── datafix/              # Python — ad-hoc data fixes
├── datastore-remover/    # Python — migration one-time
├── indexer-api-caller/   # Go — dev/debug tool
├── migrate/              # Go — migration done
├── osv-scanner/          # ? — cần review
├── reconcile/            # ? — cần review
├── review_dependency_prs.py # Python — CI utility
├── smoke-test/           # Go — testing tool
├── source-sync/          # Python — REPLACED by services/
└── sourcerepo-sync/      # ? — cần review
```

**Đề xuất:**
```
tools/
├── cmd/                  # Active Go tools
│   ├── aliaslookup/      # ✅ Giữ
│   ├── apitester/        # ✅ Giữ (nếu vẫn dùng)
│   ├── compare-responses/# ✅ Giữ
│   ├── reconcile/        # ✅ Giữ (nếu dùng)
│   └── smoke-test/       # ✅ Giữ
│
├── scripts/              # Scripts Python/Shell không phải service
│   ├── ci/               # CI/CD utilities
│   │   └── review_dependency_prs.py
│   └── datafix/          # Archive — ad-hoc data fixes
│
└── deprecated/           # Moved here before deletion
    ├── source-sync/      # ← từ tools/source-sync/ (Python)
    ├── datastore-remover/# ← migration tool đã dùng xong
    └── migrate/          # ← Datastore→Firestore migration done
```

---

### 2.5 `services/pkg/` — Tăng Cường Shared Library

**Hiện tại:**
```
services/pkg/
├── clients/    # GCS, PubSub, CloudStorage clients
├── config/     # Config loader
├── database/   # Firestore operations
├── ecosystem/  # Ecosystem adapters
├── errors/     # Error types
├── grpcutil/   # gRPC utilities
├── health/     # Health check
├── logger/     # Structured logging
├── middleware/  # HTTP/gRPC middleware
├── models/     # Domain models
├── observability/ # OTel/tracing
├── osvschema/  # OSV Schema types
├── osvutil/    # OSV utilities
├── pagination/ # Cursor pagination
├── purl/       # PURL parsing
├── resilience/ # Circuit breaker, retry
├── semver/     # SemVer utilities
└── test/       # Test helpers
```

**Bổ sung đề xuất:**
```
services/pkg/
└── ...
    ├── clients/
    │   ├── osvdev/          # 🆕 ← từ bindings/go/osvdev/
    │   └── kev/             # 🆕 CISA KEV API client
    ├── classification/      # 🆕 CVE severity/tag logic
    ├── converter/           # 🆕 Format conversion helpers (từ vulnfeeds/conversion/)
    └── notification/        # 🆕 Notification templates/channels
```

---

## 3. Thứ Tự Thực Hiện (Priority Order)

| Ưu tiên | Thay đổi | Effort | Risk |
|---------|----------|--------|------|
| P0 | Xóa `tools/source-sync/` Python | Thấp | Thấp |
| P0 | Xóa `tools/migrate/` (migration done) | Thấp | Thấp |
| P1 | Merge `bindings/go/` → `services/pkg/clients/` | Trung | Thấp |
| P1 | Merge `external/` → `services/source-sync/` | Trung | Thấp |
| P2 | Chuyển `vulnfeeds/` → `services/converter/` microservice | Cao | Trung |
| P2 | Tổ chức lại `tools/` → `tools/cmd/` + `tools/scripts/` | Trung | Thấp |
| P3 | Complete Python → Go migration (`osv/` core) | Rất cao | Cao |

---

## 4. Go Workspace Update

Sau reorganization, `go.work` sẽ được cập nhật:

```
# go.work (target)
go 1.23

use (
    ./services/pkg
    ./services/api-gateway
    ./services/vulnerability-query
    ./services/ingestion
    ./services/search
    ./services/ai-enrichment
    ./services/impact-analysis
    ./services/version-index
    ./services/alias-relations
    ./services/notification
    ./services/source-sync
    ./services/web-bff
    ./services/converter      # 🆕
    ./services/admin          # 🆕
    ./apps/osv
    ./apps/cli
    ./tools/cmd               # 🔄 từ tools/*
)

# REMOVED:
# ./bindings/go             → merged vào services/pkg/
# ./external                → merged vào services/source-sync/
# ./vulnfeeds               → chuyển thành services/converter/
```

---

## 5. Implementation Status (2026-06-03)

### 5.1 Tất Cả Reorganization Đã Thực Hiện

| Phần | Proposal | Thực tế | Trạng thái |
|------|----------|---------|----------|
| `bindings/go/` → `services/pkg/clients/` | Merge client code | Code copy done + deprecation notice | ✅ |
| `external/` → `source-sync/connectors/` | Absorb PyPI + IDs | `connectors/pypi/pypi.go`, `connectors/ids/ids.go` | ✅ |
| `vulnfeeds/` → `services/converter/` | New microservice | `converter/` với NVD+CVE5 converters | ✅ |
| `tools/source-sync/source_sync.py` | Xóa | Move `tools/deprecated/` | ✅ |
| `tools/migrate/` | Archive | Move `tools/deprecated/` | ✅ |
| `tools/datastore-remover/` | Archive | Move `tools/deprecated/` | ✅ |
| `tools/aliaslookup/` → `tools/cmd/` | Tổ chức lại | Done | ✅ |
| `services/converter/` mới | New microservice | `converter/cmd/main.go` + domain layers | ✅ |
| `services/admin/` mới | New admin service | 12 REST endpoints | ✅ |
| `services/cvectl/` mới | New CLI tool | cobra/viper CLI | ✅ |

### 5.2 Cấu Trúc Thực Tế Sau Tổ Chức Lại

```
osv.dev/
│
├── apps/                   ✅ Không đổi
├── services/               ✅ Mở rộng (converter, admin, cvectl thêm mới)
│   ├── pkg/                ✅ Shared library đầy đủ (kev+epss+cwe+models+ecosystem)
│   ├── converter/          ✅ Mới từ vulnfeeds/ (CVE5+NVD converter)
│   ├── admin/              ✅ Mới (12 REST endpoints)
│   └── cvectl/             ✅ Mới (CLI tool)
├── tools/                  ✅ Tổ chức lại (cmd/ + scripts/ + deprecated/)
├── osv/                    ⚠️ Python (isolating — OSS-Fuzz boundary documented)
├── vulnfeeds/              ⚠️ Legacy (duy trì cho CLI tools chưa port)
├── bindings/               ⚠️ Deprecated (notice added)
└── external/               ✅ Merged vào source-sync/connectors/
```

### 5.3 Còn Lại

| Item | Trạng thái |
|------|----------|
| gRPC interface cho `services/converter/` | 📋 TODO (TASK-04-04) |
| `vulnfeeds/` hoàn toàn deprecated | 📋 TODO — cần port CPE detection trước |
| `osv/` Python hoàn toàn removed | 📋 TODO — sau khi impact.py + sources.py port xong |
| CI/CD update (build tất cả services mới) | 📋 TODO (TASK-01-06) |
