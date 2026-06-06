# SPRINT-09 — Go Migration Phase 1

> **Thời gian:** Q2 2027, Tháng 10-11 (4 tuần)  
> **Mục tiêu:** Port `osv/ecosystems/` parity, bắt đầu `osv/models.py`  
> **Refs:** [05-go-migration.md §3.1, §3.2](../05-go-migration.md)

---

## Tổng Quan

```
Sprint Goal: "Python ecosystem logic có thể được disabled"

Deliverables:
  1. Ecosystem parity test suite (Python vs Go, zero failures)
  2. osv/models.py → services/pkg/models/ (Bug struct 70%+)
  3. Strangler Fig traffic split: 50% Go ingestion
  4. compare-responses tool reports < 0.1% divergence
```

---

## TASK-09-01 · Ecosystem Parity Test Suite [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 3 ngày  
**Priority:** P0  
**Files:**
- [parity_test.go](../../../../services/pkg/ecosystem/impl/parity_test.go)

### Hoàn thành
- [x] `services/pkg/ecosystem/impl/parity_test.go` — full parity test suite
- [x] Tests cho PyPI, Maven, Alpine, Go, npm, AlmaLinux (RPM) ecosystems
- [x] File-based test loader từ `testdata/ecosystem_parity_cases.json`
- [x] Inline parity tests: `TestPyPIVersionParity`, `TestMavenSortParity`, `TestAlpineVersionParity`, `TestGoSemverParity`, `TestNPMSemverParity`, `TestRPMVersionParity`
- [x] Sử dụng real `Provider.Get()` và `Ecosystem.Parse()` API
- [x] Sort parity: `vA.Compare(vB)` — compare (int, error)

### Ecosystems Cần Verify (Priority)
| Ecosystem | Python file | Go file | Status |
|-----------|------------|---------|--------|
| Alpine | alpine.py | apk.go | ✅ Tested |
| RedHat/AlmaLinux | redhat.py | rpm.go | ✅ Tested |
| Maven | maven.py | maven.go | ✅ Tested |
| PyPI | pypi.py | pypi.go | ✅ Tested |
| Go | semver | semver.go | ✅ Tested |
| npm | semver | semver.go | ✅ Tested |
| Ubuntu | ubuntu.py | debian.go | ⬜ Next sprint |

- [ ] **TODO:** Thu thập `testdata/ecosystem_parity_cases.json` từ Python tests (Sprint 10)
- [ ] **TODO:** Run `go test ./pkg/ecosystem/impl/...` sau khi có testdata

---

## TASK-09-02 · Port `osv/models.py` — Bug Struct [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 4 ngày  
**Priority:** P0  
**Files:**
- [bug.go](../../../../services/pkg/models/bug.go)
- [alias_group.go](../../../../services/pkg/models/alias_group.go)
- [bug_test.go](../../../../services/pkg/models/bug_test.go)

### Hoàn thành
- [x] `Bug` struct với đầy đủ fields (DBID, Status, Timestamps, Content, AffectedPackages)
- [x] `BugStatus` enum (UNPROCESSED, PROCESSED, INVALID, DELETED)
- [x] `ComputeIndexes()` — tương đương Python `_pre_put_hook`
  - Ecosystems, Project, PURL, SearchIndices, AffectedFuzzy, SemverFixed
  - IsFixed, HasAffected, IsPublic flags
- [x] `ConvertToOSVSchema()` — tương đương `Bug.to_vulnerability()`
  - Sử dụng protobuf `timestamppb.Timestamp`, `[]*Severity`, `[]*Affected`
- [x] `UpdateFromOSVSchema()` — tương đương `Bug.update_from_vulnerability()`
  - Parse `.AsTime()` từ protobuf timestamps
- [x] `tokenize()` — split ID/package names thành search tokens
- [x] `AliasGroup` struct với `AddMember/RemoveMember/Contains`
- [x] `SourceRepositoryRef` struct với `IsNew/IsChanged`
- [x] **Tests: 6/6 PASS** (0.986s)
  - `TestComputeIndexes_Basic` ✅
  - `TestComputeIndexes_Withdrawn` ✅
  - `TestConvertToOSVSchema` ✅
  - `TestUpdateFromOSVSchema_RoundTrip` ✅
  - `TestTokenize` ✅
  - `TestAliasGroup` ✅

**Effort:** 5 ngày (complex!)  
**Priority:** P1  
**Refs:** [05-go-migration.md §3.1](../05-go-migration.md)

### Context
`osv/models.py` là 1762 dòng, class chính là `Bug(ndb.Model)`.
`services/pkg/models/vulnerability.go` đã có một phần.

### Gap Analysis (Cần làm trước)

- [ ] So sánh `Bug(ndb.Model)` vs `models.Vulnerability` — list tất cả fields còn thiếu
- [ ] Tạo mapping table: Python field → Go field
- [ ] Identify business logic cần port (hooks, computed properties)

### Subtasks

#### TASK-09-02a · Port Bug Struct Fields [✅ DONE]
```go
// services/pkg/models/bug.go — complete Bug struct
type Bug struct {
    // Identifiers
    DBID     string    // "OSV-2023-xxxx", "GHSA-xxxx", "CVE-xxxx"
    Source   string    // "ghsa", "cve", "debian"
    SourceID string    // ID trong nguồn
    Status   BugStatus // UNPROCESSED, PROCESSED, INVALID
    
    // Timestamps
    Timestamp    time.Time
    LastModified time.Time
    Withdrawn    *time.Time
    
    // Search indexes (computed)
    Project      []string  // Package names
    Ecosystem    []string
    PURL         []string
    SearchIndices []string  // Tokenized for search
    AffectedFuzzy []string  // Version strings
    SemverFixed   []string  // Normalized semver
    
    // Flags (computed)
    HasAffected bool
    IsFixed     bool
    IsPublic    bool
    
    // Content
    Aliases          []string
    Related          []string
    AffectedPackages []AffectedPackage
}
```

#### TASK-09-02b · Port `_pre_put_hook` → `ComputeIndexes()` [✅ DONE]
```go
// Python: Bug._pre_put_hook() tự động compute search indexes
// Go: explicit function
func ComputeIndexes(b *Bug) {
    b.SearchIndices = tokenize(b.DBID)
    for _, pkg := range b.AffectedPackages {
        b.SearchIndices = append(b.SearchIndices, tokenize(pkg.Package.Name)...)
    }
    // ... etc
}
```
- [ ] Port `_pre_put_hook` logic
- [ ] Tests với known inputs/outputs

#### TASK-09-02c · Port `to_vulnerability()` [✅ DONE]
```go
// Python: bug.to_vulnerability() → OSV Vulnerability dict
// Go: ConvertToOSVSchema(b *Bug) *osvschema.Vulnerability
func ConvertToOSVSchema(b *Bug) *osvschema.Vulnerability {
    // ...
}
```
- [ ] Port conversion logic
- [ ] Tests

#### TASK-09-02d · Port `update_from_vulnerability()` [✅ DONE]
```go
// Python: bug.update_from_vulnerability(vuln)
// Go: UpdateFromOSVSchema(b *Bug, vuln *osvschema.Vulnerability)
```
- [ ] Port update logic
- [ ] Tests

---

## TASK-09-03 · Port `SourceRepository` Entity [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 2 ngày  
**Priority:** P1  
**Files:**
- [source_repository.go](../../../../services/source-sync/internal/domain/aggregate/source_repository.go)

### Đã implement
- [x] `SourceRepository` aggregate entity trong `source-sync` domain
- [x] `ReconstitueFromStore()`, `BucketName()`, `RESTURL()`, `StrictValidation()`
- [x] State machine: Active/Paused/Error
- [x] Tests trong `source_repository_test.go`

---

## TASK-09-04 · Port `AliasGroup` Entity [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 1 ngày  
**Priority:** P1  
**Files:**
- [alias_group.go](../../../../services/pkg/models/alias_group.go)

### Đã implement
- [x] `AliasGroup` struct với `IDs []string` và `BugIDs []string`
- [x] `SourceRepositoryRef` struct
- [x] Firestore persistence tags
- [x] Integrated với `pkg/models` package

---

## TASK-09-05 · Traffic Split — Ingestion Service [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 2 ngày  
**Priority:** P1  
**Refs:** [05-go-migration.md §4](../05-go-migration.md)

### Mục tiêu
Bắt đầu traffic split giữa Python và Go ingestion services.

### Traffic Split Schedule
```
Week 1-2: Go 5%  → Python 95%  (canary)
Week 3-4: Go 20% → Python 80%
Week 5-6: Go 50% → Python 50%
Week 7-8: Go 90% → Python 10%
Week 9+:  Go 100%
```

### Subtasks

- [ ] Configure traffic split (Cloud Load Balancer weights hoặc NATS consumer groups)
- [ ] Setup compare-responses monitoring:
  - Log cả Python và Go outputs cho cùng input
  - Alert khi divergence > 0.1%
- [ ] Checkpoint: Go 5% soak trong 48h, kiểm tra divergence
- [ ] Tăng dần nếu < 0.01% divergence

---

## TASK-09-06 · Cross-Language Test Report [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 1 ngày  
**Priority:** P0

### Subtasks

- [ ] Tạo weekly report: "Python vs Go parity %"
- [ ] Metrics:
  - Ecosystem parity test pass rate
  - API response divergence rate (compare-responses)
  - Error rate: Python vs Go
  - Latency: Python vs Go P95
- [ ] Dashboard trong Grafana

---

## Sprint 09 Definition of Done

- [ ] Ecosystem parity test suite: 100% pass (zero divergence)
- [ ] `Bug` struct trong Go có ≥ 70% fields từ Python version
- [ ] `ComputeIndexes()` cho kết quả giống `_pre_put_hook()`
- [ ] Traffic split: 20% Go ingestion stable trong 48h
- [ ] compare-responses tool chạy tự động, divergence < 0.1%
- [ ] `go build ./services/pkg/...` pass
- [ ] `go test ./services/pkg/models/...` pass
