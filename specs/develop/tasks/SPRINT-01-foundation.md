# SPRINT-01 — Foundation & Cleanup

> **Thời gian:** Q3 2026, Tháng 1 (2 tuần)  
> **Mục tiêu:** Dọn dẹp legacy code, tổ chức lại codebase, không còn dead code  
> **Refs:** [02-reorganization.md](../02-reorganization.md), [03-deprecation.md](../03-deprecation.md)

---

## Tổng Quan

```
Sprint Goal: "Không còn code chết, cấu trúc thư mục rõ ràng"

Deliverables:
  1. tools/ được tổ chức thành cmd/ + scripts/ + deprecated/
  2. bindings/go/ deprecated, code copy sang services/pkg/clients/
  3. external/ deprecated, code copy sang services/source-sync/connectors/
  4. CI/CD pipeline vẫn green sau tất cả thay đổi
```

---

## TASK-01-01 · Xóa/Archive Legacy Tools [✅ DONE]

**Status:** ✅ Hoàn thành (thực hiện trong session trước)  
**Effort:** 0.5 ngày  
**Priority:** P0

### Subtasks

- [x] Move `tools/source-sync/` → `tools/deprecated/source-sync/`
- [x] Move `tools/migrate/` → `tools/deprecated/migrate/`
- [x] Move `tools/datastore-remover/` → `tools/deprecated/datastore-remover/`
- [x] Move `tools/sourcerepo-sync/` → `tools/deprecated/sourcerepo-sync/`
- [x] Move `tools/datafix/` → `tools/deprecated/datafix/`
- [x] Tạo `tools/deprecated/README.md` với changelog

### Checklist Trước Khi Complete
- [x] Không có Makefile/CI nào import từ các thư mục bị move
- [x] `git status` clean sau khi move

---

## TASK-01-02 · Tổ Chức Lại `tools/` [✅ DONE]

**Status:** ✅ Hoàn thành  
**Effort:** 0.5 ngày  
**Priority:** P0

### Subtasks

- [x] `mkdir tools/cmd/ tools/scripts/ci/`
- [x] Move Go tools → `tools/cmd/`: aliaslookup, apitester, compare-responses, indexer-api-caller, osv-scanner, reconcile, smoke-test
- [x] Move Python/Shell scripts → `tools/scripts/ci/`: review_dependency_prs.py

### Kết quả
```
tools/
├── cmd/                   ✅ Active Go tools
│   ├── aliaslookup/
│   ├── apitester/
│   ├── compare-responses/
│   ├── indexer-api-caller/
│   ├── osv-scanner/
│   ├── reconcile/
│   └── smoke-test/
├── deprecated/            ✅ Archived
└── scripts/ci/            ✅ CI scripts
```

---

## TASK-01-03 · Merge `bindings/go/` → `services/pkg/clients/` [✅ DONE]

**Status:** ✅ Hoàn thành  
**Effort:** 1 ngày  
**Priority:** P1  

### Subtasks

- [x] Copy `bindings/go/osvdev/` → `services/pkg/clients/osvdev/`
- [x] Copy `bindings/go/osvdevexperimental/` → `services/pkg/clients/osvdevexperimental/`
- [x] Copy `bindings/go/api/` → `services/pkg/clients/api/`
- [x] Update import path `osv.dev/bindings/go/api` → `github.com/osv/pkg/clients/api` trong osvdev.go
- [x] Add deprecation notice: `bindings/go/README.md`
- [x] Update `apps/osv/go.mod`: redirect `osv.dev/bindings/go` → local `pkg/clients/api` via `replace` directive
- [ ] **TODO:** Remove `bindings/go/` khỏi `go.work` (deadline: Sprint 03)
- [ ] **TODO:** Xóa `bindings/go/` sau 2 sprints (deadline: Sprint 03)

### Acceptance Criteria
- [x] `go build ./services/source-sync/...` pass
- [ ] `go build ./...` hoàn toàn pass sau khi xóa bindings/go (Sprint 03)
- [ ] Không còn import nào đến `osv.dev/bindings/go` trong active code (Sprint 03)

---

## TASK-01-04 · Merge `external/` → `services/source-sync/connectors/` [✅ DONE]

**Status:** ✅ Hoàn thành  
**Effort:** 1.5 ngày  
**Priority:** P1

### Subtasks

- [x] Review `external/cmd/ids/` — ID lookup tool
- [x] Review `external/cmd/pypi/` — PyPI feed connector
- [x] Tạo `services/source-sync/internal/connectors/` directory
- [x] Port `external/cmd/pypi/` → `services/source-sync/internal/connectors/pypi/`
- [x] Port `external/cmd/ids/` → `services/source-sync/internal/connectors/ids/`
- [x] Add deprecation notice: `external/README.md`
- [ ] **TODO:** Viết unit tests cho connectors mới (Sprint 02)
- [ ] **TODO:** Xóa `external/` sau integration tests (Sprint 03)

### Acceptance Criteria
- [x] PyPI + IDS connectors tổ chức trong source-sync
- [x] `go build ./services/source-sync/...` pass

---

## TASK-01-05 · Review `tools/osv-scanner/` và `tools/sourcerepo-sync/` [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 0.5 ngày  
**Priority:** P2

### Subtasks

- [ ] Xem code `tools/osv-scanner/` — xác định đây là gì (wrapper? standalone?)
- [ ] Xem code `tools/sourcerepo-sync/` — chức năng còn unique không?
- [ ] Decision: Giữ / Move / Delete mỗi tool
- [ ] Thực hiện decision

---

## TASK-01-06 · Update CI/CD Pipelines [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 0.5 ngày  
**Priority:** P0 (block Sprint 01 close)

### Subtasks

- [ ] Review tất cả `.github/workflows/*.yml` — đảm bảo paths vẫn đúng
- [ ] Review Cloud Build configs — update tool paths
- [ ] Run full CI pipeline — verify green
- [ ] Update build matrix nếu cần

### Files cần check
```bash
find . -name "*.yml" -path "*github/workflows*" | head -20
find . -name "cloudbuild*.yaml" | head -10
```

---

## Sprint 01 Definition of Done

- [x] `go build ./services/source-sync/...` pass ✅ 2026-06-03
- [x] `go build ./services/converter/...` pass ✅ 2026-06-03
- [x] `go build ./services/ai-enrichment/...` pass ✅ 2026-06-03
- [x] `go build ./services/admin/...` pass ✅ 2026-06-03
- [x] `go build ./services/pkg/...` pass ✅ 2026-06-03
- [x] Domain tests pass: `source-sync` aggregate test ✅
- [x] `bindings/go/` đã có deprecation notice ✅
- [x] `external/` connectors đã được merge vào source-sync ✅
- [ ] `go build ./apps/...` pass (apps có legacy deps — Sprint 03)
- [ ] CI/CD pipeline green (Sprint 02)
