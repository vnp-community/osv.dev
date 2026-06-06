# SPRINT-02 — Shared Library Enhancement (`services/pkg/`)

> **Thời gian:** Q3 2026, Tháng 1-2 (3 tuần)  
> **Mục tiêu:** Mở rộng `services/pkg/` thành shared library đầy đủ cho tất cả services  
> **Refs:** [02-reorganization.md §2.5](../02-reorganization.md), [06-new-features.md §3](../06-new-features.md)

---

## Tổng Quan

```
Sprint Goal: "services/pkg/ là source of truth cho shared logic"

Deliverables:
  1. pkg/clients/kev/    — CISA KEV client (✅ DONE)
  2. pkg/clients/epss/   — EPSS scoring client (✅ DONE)
  3. pkg/classification/ — CVE severity + auto-tagging (✅ DONE)
  4. pkg/clients/osvdev/ — Merged từ bindings/go (🔄)
  5. pkg/cwe/            — CWE database (📋 TODO)
  6. Ecosystem parity audit vs Python
```

---

## TASK-02-01 · KEV Client (`pkg/clients/kev/`) [✅ DONE]

**Status:** ✅ Hoàn thành — 8/8 tests pass  
**Effort:** 2 ngày  
**File:** [services/pkg/clients/kev/kev.go](../../../../services/pkg/clients/kev/kev.go)

### Đã implement
- [x] `Client` struct với `FetchCatalog(ctx)` — HTTP download
- [x] `InMemoryLookup` — O(1) lookup từ cached catalog
- [x] `IsKEV(cveID)`, `Get(cveID)`, `Count()`
- [x] `BuildIndex(catalog)` — Build lookup map từ catalog
- [x] `WithHTTPClient()`, `WithCatalogURL()` options
- [x] Tests: TestBuildIndex, TestInMemoryLookup, TestFetchCatalog, TestFetchCatalog_ServerError

### Còn thiếu / Follow-up
- [ ] Scheduled refresh goroutine (chạy background refresh mỗi 24h)
- [ ] Metrics: catalog_fetch_total, catalog_fetch_errors_total, catalog_size_entries
- [ ] Alert khi catalog version thay đổi
- [ ] Integration test với thực tế CISA URL

---

## TASK-02-02 · EPSS Client (`pkg/clients/epss/`) [✅ DONE]

**Status:** ✅ Hoàn thành — 4/4 tests pass  
**Effort:** 1.5 ngày  
**File:** [services/pkg/clients/epss/epss.go](../../../../services/pkg/clients/epss/epss.go)

### Đã implement
- [x] `Client` với `Get(ctx, cveID)` và `GetBatch(ctx, cveIDs)`
- [x] Auto-chunking khi batch > 100 CVEs
- [x] `Score.Tier()` — CRITICAL/HIGH/MEDIUM/LOW từ percentile
- [x] Tests: TestGet, TestGet_NotFound, TestGetBatch, TestScore_Tier

### Còn thiếu / Follow-up
- [ ] Caching layer: Redis cache với TTL = 24h (EPSS update daily)
- [ ] Metrics: epss_requests_total, epss_cache_hits_total
- [ ] Batch daily update job (update tất cả CVE trong DB mỗi ngày)
- [ ] Percentile threshold alerts (>0.95 → high priority alert)

---

## TASK-02-03 · Classification Package (`pkg/classification/`) [✅ DONE]

**Status:** ✅ Hoàn thành — 12/12 tests pass  
**Effort:** 2 ngày  
**File:** [services/pkg/classification/classification.go](../../../../services/pkg/classification/classification.go)

### Đã implement
- [x] `CVSSTier(score float64)` — CRITICAL/HIGH/MEDIUM/LOW từ CVSS score
- [x] `TagsFromCVSSVector(vector)` — attack:network, impact:rce, etc.
- [x] `TagsFromDescription(text)` — keyword-based tagging (12 rules)
- [x] `Classify(vuln)` — Full vulnerability classification
- [x] Tag taxonomy: attack:*, impact:*, status:*, severity:*

### Còn thiếu / Follow-up
- [ ] **TASK-02-03a:** Thêm CWE-based tagging: `TagsFromCWE(cweIDs []string)`
- [ ] **TASK-02-03b:** Thêm package-based tagging: `TagsFromPackages(affected []Affected)`
- [ ] **TASK-02-03c:** MITRE ATT&CK mapping từ tags
- [ ] **TASK-02-03d:** Test coverage cho edge cases (empty vector, malformed input)
- [ ] **TASK-02-03e:** Thêm `pkg/classification/rules/` — external YAML rule file thay vì hardcode

### Tag Taxonomy Còn Thiếu
```go
// Cần thêm vào classification.go:
TagKernelRelated  Tag = "asset:kernel"
TagWebApp         Tag = "asset:web"
TagCloudService   Tag = "asset:cloud"
TagMobile         Tag = "asset:mobile"
TagNetwork        Tag = "asset:network"
TagIoT            Tag = "asset:iot"
```

---

## TASK-02-04 · CWE Database Package (`pkg/cwe/`) [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 3 ngày  
**Priority:** P2  
**Files:**
- [pkg/cwe/cwe.go](../../../../services/pkg/cwe/cwe.go)
- [pkg/cwe/data.go](../../../../services/pkg/cwe/data.go)
- [pkg/cwe/cwe_test.go](../../../../services/pkg/cwe/cwe_test.go)

### Đã implement
- [x] 60+ CWE entries embedded trong binary (top-200 common CWEs)
- [x] Phân loại thành 7 categories: Memory, Injection, Authentication, Authorization, Cryptography, Network, ResourceMgmt
- [x] `Get(id)` — lookup bằng CWE-79 hoặc "79"
- [x] `Tags(id)` — suggested OSV tags cho CWE
- [x] `TagsForAll(ids)` — deduplicated tags cho list CWE IDs
- [x] `GetCategory(id)` — category lookup
- [x] `IsKnown(id)` — validation
- [x] `List()` — all entries
- [x] `ByCategory(cat)` — filter bằng category
- [x] **Tests: 12/12 PASS** (0.323s)
  - TestGet, TestGetNotFound, TestTags, TestTagsNotFound ✅
  - TestTagsForAll, TestGetCategory, TestGetCategoryUnknown ✅
  - TestIsKnown, TestList, TestByCategory, TestXSSEntry, TestSQLInjectionEntry ✅

### Còn thiếu / Follow-up
- [ ] CAPEC mapping (CAPEC attack patterns từ CWE)
- [ ] Ancestor traversal: `GetAncestors(cweID)` — traverse parent chain
- [ ] `SuggestFromCVSS(vector)` — suggest CWEs từ CVSS vector
- [ ] Embed từ NVD XML thay vì hardcode (updater script)

---

## TASK-02-05 · Ecosystem Parity Audit [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 2 ngày  
**Priority:** P0 (block Python deprecation)  
**Refs:** [05-go-migration.md §3.2](../05-go-migration.md)

### Mục tiêu
Xác nhận `services/pkg/ecosystem/impl/` (Go) và `osv/ecosystems/` (Python) cho kết quả giống nhau.

### Subtasks

- [ ] Lập danh sách đầy đủ tất cả Python ecosystem adapters trong `osv/ecosystems/`
- [ ] Lập danh sách đầy đủ tất cả Go implementations trong `services/pkg/ecosystem/impl/`
- [ ] **Gap analysis:** Python có nhưng Go chưa có
  - [ ] ubuntu — Python có, Go có `debian.go` bao gồm không?
  - [ ] alpine (alpine.py) vs apk (apk.go) — logic identical?
  - [ ] redhat (redhat.py) vs rpm (rpm.go) — logic identical?
  - [ ] haskell (haskell.py) vs ghc.go + hackage.go — split correctly?
- [ ] Tạo parity test: cùng input → cùng output cho cả Python và Go
  ```bash
  # Tạo test data file: ecosystem_parity_cases.json
  # Format: [{ecosystem, version_input, expected_normalized}]
  ```
- [ ] Fix tất cả parity failures
- [ ] Document: `services/pkg/ecosystem/impl/PARITY.md`

### Acceptance Criteria
- [ ] Zero parity failures trên test suite
- [ ] 100% ecosystem coverage: Go có implementation cho tất cả ecosystem Python có

---

## TASK-02-06 · EPSS Daily Update Job [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 1 ngày  
**Priority:** P1

### Mục tiêu
EPSS scores thay đổi hàng ngày. Cần batch update tất cả CVE trong DB.

### Subtasks

- [ ] Tạo `services/ai-enrichment/internal/application/command/daily_epss_update/`
- [ ] Implement `DailyEPSSUpdateHandler`:
  1. Lấy danh sách tất cả CVE IDs từ Firestore (batch query)
  2. Batch fetch EPSS scores (100 CVEs/request)
  3. Update enriched records với scores mới
  4. Publish NATS events cho CVE có EPSS change > 0.1
- [ ] Integrate vào scheduler (Cloud Scheduler trigger hàng ngày 06:00 UTC)
- [ ] Metrics: epss_daily_update_duration_seconds, epss_updated_count
- [ ] Alerting: Nếu daily update fail > 2 lần liên tiếp

### Acceptance Criteria
- [ ] Daily job chạy thành công với test dataset
- [ ] EPSS scores trong DB accurate trong vòng 24h

---

## Sprint 02 Definition of Done

- [ ] `go build ./pkg/...` pass — không có breaking changes
- [ ] `go test ./pkg/...` — tất cả tests pass (hiện tại 24 tests)
- [ ] CWE package có thể lookup CWE-89 → SQL Injection
- [ ] Ecosystem parity: zero failures trên test suite
- [ ] EPSS daily update job documented và có unit tests
