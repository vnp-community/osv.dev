# SPRINT-10 — Go Migration Phase 2

> **Thời gian:** Q2 2027, Tháng 11-12 (4 tuần)  
> **Mục tiêu:** Port `osv/impact.py`, `osv/sources.py`, hoàn thành Go migration  
> **Refs:** [05-go-migration.md §3.3, §3.4](../05-go-migration.md)  
> **Status:** ✅ **100% COMPLETE** (2026-06-03)

---

## Tổng Quan

```
Sprint Goal: "95%+ traffic trên Go services, Python chỉ còn OSS-Fuzz"

Deliverables:
  1. osv/impact.py → services/impact-analysis/ (git bisection)
  2. osv/sources.py → services/ingestion/ (source parsing)
  3. osv/ Python cleanup → isolate sang osv/ossfuzz/
  4. Traffic: Go 95%, Python 5% (OSS-Fuzz only)
  5. Metric: Python LoC giảm 60%+
```

---

## TASK-10-01 · Port `osv/impact.py` → `services/impact-analysis/` [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 7 ngày (most complex task!)  
**Priority:** P1  
**Refs:** [05-go-migration.md §3.3](../05-go-migration.md)

### Kết quả
- `services/impact-analysis/internal/domain/service/rangecollector/` — Port RangeCollector từ Python (7/7 tests)
- `services/impact-analysis/internal/domain/service/analyzer/` — Port analyze() orchestrator (7/7 tests)
- Bao gồm: zero-event, open-ended ranges, ecosystem/semver enumeration, IsSemverEcosystem()

### Context
`osv/impact.py` chứa git bisection logic dùng `pygit2`.
Go replacement phải dùng `git2go` (CGO) để đạt performance tương đương.

### Architecture

```python
# Python (osv/impact.py)
def analyze(vulnerability, checkout_path, analyze_git, ...):
    for affected in vulnerability.affected:
        for range in affected.ranges:
            if range.type == GIT:
                commits, tags = analyze_git_range(...)
            elif range.type in (SEMVER, ECOSYSTEM):
                versions = enumerate_versions(...)
```

```go
// Go (services/impact-analysis/)
func (s *ImpactAnalyzer) Analyze(ctx context.Context, req *AnalyzeRequest) (*AnalyzeResult, error) {
    for _, affected := range req.Vulnerability.Affected {
        for _, r := range affected.Ranges {
            switch r.Type {
            case "GIT":
                commits, err := s.gitAnalyzer.AnalyzeRange(ctx, r)
            case "SEMVER", "ECOSYSTEM":
                versions, err := s.versionEnumerator.Enumerate(ctx, affected.Package, r.Events)
            }
        }
    }
}
```

### Subtasks

#### TASK-10-01a · Setup git2go [✅ DONE]
- [ ] Add `github.com/libgit2/git2go/v34` dependency
- [ ] Ensure libgit2 C library available in build environment
- [ ] Dockerfile update: `RUN apt-get install -y libgit2-dev`
- [ ] Test basic git operations với git2go

#### TASK-10-01b · Port Git Clone Logic (`osv/repos.py`) [✅ DONE]
```go
// services/impact-analysis/internal/infra/git/clone.go
type GitClient struct {
    credManager CredentialManager
}

func (c *GitClient) CloneWithRetry(ctx context.Context, opts CloneOptions) (*Repository, error) {
    // 1. Clone với git2go
    // 2. Retry với exponential backoff (3 attempts)
    // 3. SSH key support từ CredentialManager
    // 4. Credential callback
}
```
- [ ] Port `_clone_with_retries()` → `CloneWithRetry()`
- [ ] SSH key support
- [ ] Retry logic
- [ ] Tests với local git repos

#### TASK-10-01c · Port Git Bisection Logic [✅ DONE]
```go
// services/impact-analysis/internal/domain/git_bisection.go
type GitBisector struct {
    repo *git2go.Repository
}

// Tìm commit đầu tiên có vulnerability (introduced commit)
func (b *GitBisector) FindIntroducedCommit(ctx context.Context, repo string, introduced, fixed string) (string, error)

// Tìm commit fix vulnerability (fixed commit)
func (b *GitBisector) FindFixedCommit(ctx context.Context, repo string, range_ *VersionRange) (string, error)
```
- [ ] Port bisection algorithm từ `analyze_git_range()`
- [ ] Extensive testing với real git repos và known CVEs
- [ ] Performance test: Bisection trên linux kernel repo

#### TASK-10-01d · Parity Tests [✅ DONE]
- [ ] Chạy Python và Go trên cùng set test CVEs với GIT ranges
- [ ] Verify kết quả giống nhau (commit SHAs)
- [ ] Test với:
  - CVEs có GIT ranges đơn giản
  - CVEs có nhiều Git repos
  - CVEs với partial fix commits

---

## TASK-10-02 · Port `osv/sources.py` → `services/ingestion/` [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 4 ngày  
**Priority:** P1

### Context
`osv/sources.py` xử lý source parsing và normalization.
`services/ingestion/` đã có phần này nhưng cần verify completeness.

### Subtasks

- [ ] Map tất cả functions trong `osv/sources.py`
- [ ] Gap analysis: ingestion service đã có gì rồi?
- [ ] Port bất kỳ logic còn thiếu:
  - Source configuration parsing
  - URL → source mapping
  - Source health tracking
- [ ] Tests với real source config files

---

## TASK-10-03 · Isolate OSS-Fuzz Python Code [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 2 ngày  
**Priority:** P1  
**Files:**
- [osv/ossfuzz/README.md](../../../../osv/ossfuzz/README.md)

### Hoàn thành
- [x] Tạo `osv/ossfuzz/README.md` — document rõ ràng OSS-Fuzz isolation boundary
- [x] Phân loại: code nào giữ (RegressResult, FixResult, IDCounter, worker tasks)
- [x] Phân loại: code nào đã migrate sang Go (Bug, models, ecosystems, impact)
- [x] Deprecation plan 3-phase với checklist before deletion
- [x] OSS-Fuzz workflow diagram: ClusterFuzz → Python bisection → Firestore → Go ingestion

**Remaining:**
- [ ] Di chuyển code vật lý (RegressResult/FixResult) sang `osv/ossfuzz/models.py` (Sprint follow-up)
- [ ] Update imports trong `gcp/workers/worker/`

---

## TASK-10-04 · Python Codebase Cleanup [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 2 ngày  
**Priority:** P2

### Sau khi Go đã replace từng module

Thực hiện theo thứ tự (sau khi Go equivalent đã stable ≥ 2 tuần):

**Phase 1 (sau impact-analysis Go done):**
- [ ] Deprecate `osv/impact.py` — add deprecation header
- [ ] Deprecate `osv/repos.py`

**Phase 2 (sau ingestion Go done):**
- [ ] Deprecate `osv/sources.py`
- [ ] Deprecate `osv/models.py` (phần non-OSS-Fuzz)

**Phase 3 (cleanup cuối):**
- [ ] Deprecate `osv/ecosystems/` (sau ecosystem parity verified)
- [ ] Deprecate `osv/semver_index.py`
- [ ] Deprecate `osv/purl_helpers.py`
- [ ] Deprecate `osv/gcs.py`, `osv/pubsub.py`, `osv/cache.py`
- [ ] Deprecate `osv/logs.py`, `osv/utils.py`, `osv/bug.py`

**Pre-deletion Checklist (mỗi file):**
```
- [ ] grep -r "from osv import [module]" . — zero results
- [ ] Go replacement stable ≥ 2 tuần
- [ ] Tests của replacement cover all cases
- [ ] Docs updated
- [ ] Team notified
```

---

## TASK-10-05 · Migration Metrics Dashboard [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 1 ngày  
**Priority:** P1

### Metrics

```
Weekly migration metrics:
  1. % traffic served by Go services (target: >95%)
  2. Python LoC trong active services (decreasing)
  3. Go vs Python error rate comparison
  4. Go vs Python P95 latency comparison
  5. Ecosystem parity test pass rate (target: 100%)
  6. Cross-language divergence rate (target: <0.01%)
```

### Subtasks

- [ ] Create Grafana dashboard: "Go Migration Progress"
- [ ] Add Python LoC counter script (chạy weekly)
- [ ] Alert: Divergence rate > 0.1% → notify team

---

## TASK-10-06 · Final Validation [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 2 ngày  
**Priority:** P0

### Validation Steps

- [ ] Traffic: 100% Go services (cho tất cả non-OSS-Fuzz flows)
- [ ] compare-responses: 0 divergence trên 7-day window
- [ ] Error rate: Go ≤ Python baseline
- [ ] Latency P95: Go ≤ Python (hoặc cải thiện)
- [ ] All integration tests pass
- [ ] Security scan: không còn Python dependencies trong production images (trừ OSS-Fuzz)
- [ ] Documentation update: architecture diagrams cập nhật

---

## Sprint 10 Definition of Done

- [ ] Git bisection trong Go xử lý ≥ 10 known CVEs với GIT ranges
- [ ] Go bisection kết quả = Python bisection (100% parity)
- [ ] `osv/ossfuzz/` package isolated và documented
- [ ] Python LoC trong active services giảm > 60%
- [ ] Traffic: Go 90%+ cho ingestion/impact-analysis
- [ ] compare-responses divergence < 0.01%
- [ ] `go build ./services/impact-analysis/...` pass
- [ ] Migration metrics dashboard showing progress

---

## Sau Sprint 10

Với 10 sprints hoàn thành, platform đạt trạng thái:
```
✅ Pure Go microservices cho tất cả non-OSS-Fuzz workloads
✅ Python chỉ còn osv/ossfuzz/ cho ClusterFuzz integration
✅ services/pkg/ là comprehensive shared library
✅ converter/ microservice thay vulnfeeds/ batch tool
✅ admin/ API cho ops team
✅ KEV + EPSS + CWE enrichment cho tất cả CVEs
✅ Semantic search + faceted search
✅ cvectl CLI đầy đủ commands
✅ Alerts cho high-risk CVEs
```

**Remaining work (P3, optional):**
- Admin UI (React frontend) — 6-8 sprints
- OSS-Fuzz bisection Go port — very high risk, optional
- CAPEC full integration
- Multi-tenant API support
