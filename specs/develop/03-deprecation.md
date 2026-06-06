# 03 — Deprecation List: Code Cần Xóa/Deprecate

> **Date:** 2026-06-03  
> **Status:** 🔄 In Execution — Xem Implementation Status ở dưới

---

## Nguyên Tắc Xóa

1. **Không xóa đột ngột** — mark deprecated trước ít nhất 1 sprint
2. **Archive trước khi xóa** — nếu có giá trị lịch sử, move vào `deprecated/` branch hoặc tag
3. **Xóa theo thứ tự phụ thuộc** — xóa leaf nodes trước

---

## 1. 🔴 XÓA NGAY (P0 — Low Risk, No Impact)

### 1.1 `tools/source-sync/source_sync.py`
```
tools/source-sync/
└── source_sync.py         # Python script cũ
```
- **Lý do:** Hoàn toàn được thay thế bởi `services/source-sync/` (Go microservice)
- **Tác động:** Không — không ai dùng trong production nữa
- **Action:** `git rm tools/source-sync/` hoặc move vào `tools/deprecated/`

---

### 1.2 `tools/migrate/datastore_to_firestore.go`
```
tools/migrate/
└── datastore_to_firestore.go  # One-time migration script
```
- **Lý do:** Migration từ Cloud Datastore → Firestore đã hoàn thành từ trước
- **Tác động:** Không
- **Action:** `git rm tools/migrate/`

---

### 1.3 `tools/datastore-remover/`
```
tools/datastore-remover/    # Python — xóa records khỏi Datastore
```
- **Lý do:** Datastore không còn được dùng; đây là một-lần migration tool
- **Tác động:** Không
- **Action:** `git rm tools/datastore-remover/`

---

### 1.4 `osv/gcs_mock.py`
```
osv/gcs_mock.py             # Mock GCS cho Python tests
```
- **Lý do:** Chỉ dùng trong Python test context; sẽ không cần khi Python services bị remove
- **Tác động:** Chỉ ảnh hưởng Python tests
- **Action:** Giữ lại cho đến khi Python core hoàn toàn removed

---

## 2. 🟠 DEPRECATE (P1 — Cần Replace Trước Khi Xóa)

### 2.1 `bindings/go/`
```
bindings/
└── go/
    ├── api/
    ├── osvdev/
    └── osvdevexperimental/
```
- **Lý do:** Tất cả chức năng sẽ được merge vào `services/pkg/clients/`
- **Timeline:** Xóa sau khi merge hoàn tất và tất cả importers cập nhật
- **Step 1:** Merge code vào `services/pkg/clients/`
- **Step 2:** Update tất cả `import` paths
- **Step 3:** Add deprecation notice trong `bindings/go/README.md`
- **Step 4:** Xóa sau 2 sprints

---

### 2.2 `external/`
```
external/
├── cmd/ids/
└── cmd/pypi/
```
- **Lý do:** Merge vào `services/source-sync/internal/connectors/`
- **Timeline:** Sau khi source-sync service absorb logic
- **Action tương tự** bindings/go/ ở trên

---

### 2.3 `tools/sourcerepo-sync/`
- **Cần review:** Chức năng có thể trùng với `services/source-sync/`
- **Action:** Review code, nếu duplicate → deprecate; nếu unique → migrate sang Go service

---

### 2.4 `tools/indexer-api-caller/`
- **Cần review:** Có thể là debug tool cho indexer service cũ
- **Action:** Review nếu vẫn cần cho `services/version-index/` thì giữ; nếu không thì xóa

---

### 2.5 `tools/osv-scanner/`
- **Cần review:** Không rõ chức năng — có thể là wrapper cho OSV Scanner CLI
- **Action:** Nếu là shell script wrapper → move vào `tools/scripts/`; nếu là Go binary → review

---

## 3. 🟡 PLANNED DEPRECATION (P2 — Long Term)

### 3.1 `osv/` — Python Core Library

Đây là thay đổi lớn nhất. Chi tiết trong [05-go-migration.md](./05-go-migration.md).

**Các file sẽ được deprecated theo thứ tự:**

```
Phase 1 (sau khi Go impact-analysis service hoàn chỉnh):
  osv/impact.py          # 29KB — Moved to services/impact-analysis/
  osv/repos.py           # Git repo management → services/impact-analysis/

Phase 2 (sau khi Go ingestion service hoàn chỉnh):
  osv/sources.py         # Source parsing → services/ingestion/
  osv/models.py          # → services/pkg/models/ (chủ yếu)
  osv/models/            # Submodule đang refactor

Phase 3 (sau khi tất cả Python services removed):
  osv/ecosystems/        # → services/pkg/ecosystem/impl/ (hoàn chỉnh)
  osv/semver_index.py    # → services/pkg/semver/
  osv/purl_helpers.py    # → services/pkg/purl/
  osv/gcs.py             # → services/pkg/clients/gcs.go
  osv/pubsub.py          # → services/pkg/clients/pubsub.go
  osv/cache.py           # → Redis-based caching trong services/

Phase 4 (cuối cùng):
  osv/__init__.py        # Core library init
  osv/logs.py            # → services/pkg/logger/
  osv/utils.py           # → services/pkg/ (scattered)
  osv/bug.py             # → services/pkg/models/
```

**OSS-Fuzz specific (có thể giữ lại lâu hơn):**
```
osv/models.py:
  class RegressResult(ndb.Model)   # OSS-Fuzz specific
  class FixResult(ndb.Model)       # OSS-Fuzz specific
  class IDCounter(ndb.Model)       # Internal counter
```
Những class này cần được handled riêng — có thể tồn tại trong `osv/ossfuzz/` nếu OSS-Fuzz vẫn cần Python.

---

### 3.2 `vulnfeeds/` — Sau Khi Converter Microservice Stable

- **Timeline:** Sau khi `services/converter/` được deploy và validate
- **Replacement:** `services/converter/` gRPC microservice
- **Action:** Archive `vulnfeeds/` vào separate branch, xóa khỏi main

---

## 4. 🟢 GIỮ NGUYÊN (No Change)

| Module | Lý do giữ |
|--------|-----------|
| `tools/aliaslookup/` | Cần cho debugging aliases |
| `tools/smoke-test/` | Cần cho CI/CD smoke testing |
| `tools/compare-responses/` | Cần cho migration validation |
| `tools/apitester/` | Cần cho API testing |
| `tools/reconcile/` | Cần cho data reconciliation |
| `apps/cli/` | Active CLI app |
| `apps/osv/` | Active server app |
| `osv/osv-schema/` | Submodule — external reference |

---

## 5. Deprecation Timeline

```
Month 1:
  [x] Xóa tools/source-sync/
  [x] Xóa tools/migrate/
  [x] Xóa tools/datastore-remover/

Month 2:
  [ ] Merge bindings/go/ → services/pkg/
  [ ] Merge external/ → services/source-sync/
  [ ] Review tools/sourcerepo-sync/, tools/indexer-api-caller/, tools/osv-scanner/

Month 3-4:
  [ ] Convert vulnfeeds/ → services/converter/
  [ ] Deprecate vulnfeeds/ (archive)

Month 5-12:
  [ ] Phase 1-4 Python → Go migration
  [ ] Deprecate osv/ dần theo từng phase

Year 2+:
  [ ] osv/ hoàn toàn removed
```

---

## 6. Checklist Trước Khi Xóa Mỗi Module

```markdown
Pre-deletion Checklist cho [module]:
- [ ] Kiểm tra có service/code nào còn import?
  grep -r "from [module] import" .
  grep -r "import [module]" .
- [ ] Replacement đã hoạt động ổn định ≥ 2 tuần?
- [ ] Tests của replacement cover các cases từ code cũ?
- [ ] Documentation đã cập nhật?
- [ ] Team đã được thông báo?
- [ ] Archive hoặc tag commit cuối trước khi xóa?
- [ ] CI/CD pipeline đã được cập nhật?
```

---

## 6. Implementation Status (2026-06-03)

### 6.1 Đã Xử Lý ✅

| Item | Proposal | Thực hiện | Trạng thái |
|------|----------|----------|----------|
| `tools/source-sync/source_sync.py` | Xóa/move | Moved `tools/deprecated/` | ✅ DONE |
| `tools/migrate/datastore_to_firestore.go` | Archive | Moved `tools/deprecated/` | ✅ DONE |
| `tools/datastore-remover/` | Xóa | Moved `tools/deprecated/` | ✅ DONE |
| `bindings/go/` | Deprecated notice + merge | `bindings/go/README.md` deprecation notice | ✅ DONE |
| `external/` connectors | Absorb vào source-sync | `source-sync/internal/connectors/` | ✅ DONE |
| OSS-Fuzz entities trong `osv/models.py` | Isolate | OSS-Fuzz boundary doc (`osv/ossfuzz/README.md`) | ✅ DONE |

### 6.2 Đang Xử Lý 🔄

| Item | Tiến độ | Ghi chú |
|------|---------|--------|
| `osv/models.py` deprecation | 🔄 30% | `Bug` + `AliasGroup` port done → `services/pkg/models/` |
| `vulnfeeds/conversion/` | 🔄 60% | CVE5+NVD converter port done; còn CPE detection + gRPC |
| Python services | 🔄 Isolating | OSS-Fuzz vẫn ở Python |

### 6.3 Chưa Bắt Đầu (Blockers)

| Item | Blocker |
|------|---------|
| Xóa `osv/impact.py` | Cần port sang `services/impact-analysis/` (TASK-10-01, ~7d) |
| Xóa `osv/sources.py` | Cần port sang `services/ingestion/` (TASK-10-02, ~4d) |
| Xóa `vulnfeeds/` hoàn toàn | Cần gRPC interface + CPE detection (TASK-04-03/04) trước |
| Xóa `osv/gcs_mock.py` | Cần sau khi Python tests removed |
| Xóa `osv/gcs.py` | Cần sau khi `osv/models.py` hoàn toàn port xong |

### 6.4 Deprecation Timeline Thực Tế

```
Q3 2026 (Done):
  ✅ tools/deprecated/ — source_sync.py, migrate/, datastore-remover/
  ✅ bindings/go/ — deprecation notice
  ✅ external/ — absorbed vào source-sync/connectors/

Q4 2026 (In Progress):
  🔄 osv/models.py — Bug, AliasGroup done; impact/sources TODO
  🔄 vulnfeeds/conversion/ — CVE5/NVD done; CPE/gRPC TODO

Q1 2027 (Planned):
  📋 vulnfeeds/ full deprecation (sau khi gRPC done)
  📋 osv/impact.py removal (sau khi impact-analysis port done)

Q2 2027 (Planned):
  📋 osv/ Python core removal (chỉ giữ ossfuzz/)
  📋 Python stack decommission
```
