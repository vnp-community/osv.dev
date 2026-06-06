# SPRINT-05 — AI Enrichment Enhancement

> **Thời gian:** Q4 2026, Tháng 5 (3 tuần)  
> **Mục tiêu:** Nâng cấp pipeline enrichment với KEV, EPSS, CWE, exploit check  
> **Refs:** [04-roadmap.md §2.5](../04-roadmap.md), [06-new-features.md §3](../06-new-features.md)

---

## Tổng Quan

```
Sprint Goal: "Mỗi CVE được enrich với KEV status, EPSS score, CWE, và exploit availability"

Pipeline mục tiêu (in order):
  1. CVSSEnrichment      — Parse/normalize CVSS vectors (đã có?)
  2. KEVEnrichment       — Tag KEV, store metadata (✅ skeleton done)
  3. EPSSEnrichment      — Fetch/store EPSS score (✅ skeleton done)
  4. CWEEnrichment       — CWE classification + CAPEC (✅ DONE)
  5. ExploitCheck        — PoC availability (GitHub, ExploitDB) (✅ DONE)
  6. AutoTagging         — Rule-based + ML tags (🔄 partial)
  7. LLMSummarization    — AI-generated summary (đã có)
  8. VectorEmbedding     — Semantic search embedding (đã có)
```

---

## TASK-05-01 · Wire Threat Intel Pipeline vào ai-enrichment Service [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 1 ngày  
**Files:**
- [threatintel.go](../../../../services/ai-enrichment/internal/domain/service/threatintel/threatintel.go)
- [cwe_stage.go](../../../../services/ai-enrichment/internal/domain/service/threatintel/cwe_stage.go)
- [handler.go](../../../../services/ai-enrichment/internal/application/command/enrich_vulnerability/handler.go)

### Đã hoàn thành
- [x] `KEVStage` — cached KEV lookup, thread-safe refresh, 24h TTL
- [x] `EPSSStage` — batch EPSS scoring
- [x] `Pipeline` với `Enrich(ctx, vuln)` method
- [x] `NewThreatIntelPipeline(kevClient, epssClient, log)`
- [x] `enrich_vulnerability.Handler` — orchestrates 6-stage pipeline:
  1. Embedding generation
  2. Severity classification
  3. LLM technical summary + remediation
  4. Tag extraction
  5. Persist to Firestore
  6. Publish AIEnrichmentCompleted event

### Còn phải làm (Follow-up)
- [ ] TASK-05-01b: Store KEV/EPSS data vào Enriched Record Firestore fields
- [ ] TASK-05-01c: Prometheus metrics (kev_enriched_total, epss_score_distribution)

---

## TASK-05-02 · CWE Enrichment Stage [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 2 ngày  
**Priority:** P1  
**Files:**
- [cwe_stage.go](../../../../services/ai-enrichment/internal/domain/service/threatintel/cwe_stage.go)

### Đã implement
- [x] `CWEStage` struct với `Enrich(ctx, vuln)` method
- [x] `EnrichWithResult(vuln)` — trả về `CWEEnrichmentResult` có `CWEIDs`, `Tags`, `Categories`
- [x] `extractCWEIDsFromVuln(vuln)` — scan Related field + text pattern matching
- [x] `scanCWEPattern(text)` — regex-like scan cho CWE-NNN trong sưmmary/details
- [x] Tích hợp với `pkg/cwe`: `Tags()`, `TagsForAll()`, `GetCategory()`
- [x] `NewFullEnrichmentPipeline()` — KEV → EPSS → CWE pipeline factory
- [x] Build pass: `go build ./ai-enrichment/...` ✅

### Còn thiếu (follow-up)
- [ ] Ancestor traversal cho CWE hierarchy
- [ ] CAPEC mapping integration
- [ ] LLM-based CWE suggestion (khi không có CWE ID trong data)
- [ ] Tests cho `cwe_stage.go`

- [ ] Store: `cwe_ids`, `cwe_categories` (top-level ancestors)
- [ ] Tests

---

## TASK-05-03 · Exploit Availability Check [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 3 ngày  
**Priority:** P2

### Mục tiêu
Kiểm tra xem CVE có PoC exploit công khai không (GitHub, ExploitDB, PacketStorm).

### Sources để Check
1. **GitHub Search API** — search repo names và topics
2. **ExploitDB** — https://www.exploit-db.com/ (có API/RSS)
3. **NVD References** — references có tag `Exploit`

### Subtasks

- [ ] Tạo `services/ai-enrichment/internal/domain/service/exploit/`
- [ ] Implement `ExploitCheckStage`:
  ```go
  type ExploitInfo struct {
      HasPublicPoC     bool
      PoCURLs          []string
      ExploitMaturity  string  // "proof-of-concept", "functional", "weaponized"
      Sources          []string
  }
  ```
- [ ] Implement GitHub search: `{CVE-ID} exploit OR poc`
  - Rate limit: 30 requests/min (GitHub Search API)
  - Cache results: Redis TTL = 1 giờ
- [ ] Implement ExploitDB lookup (CSV download, search by CVE ID)
- [ ] Implement NVD reference tag check (references với tag "Exploit")
- [ ] Store: `exploit_available`, `exploit_urls`, `exploit_maturity`
- [ ] Tests với known exploit CVEs (CVE-2021-44228 Log4Shell)

### Rate Limiting
```go
// GitHub Search API: 30 requests/min unauthenticated, 30/min authenticated
// → Use token + rate limiter
// → Batch CVEs, không check từng cái riêng lẻ
```

---

## TASK-05-04 · Auto-Tagging Enhancement [🔄 IN PROGRESS]

**Status:** ✅ Hoàn thành — LLM+rule-based MITRE ATT&CK tagger (9/9 tests)  
**Effort:** 3 ngày  
**Priority:** P1

### Đã có (từ Sprint 02)
- [x] `TagsFromCVSSVector()` — attack:*, impact:* từ CVSS vector
- [x] `TagsFromDescription()` — keyword-based tagging
- [x] `Classify(vuln)` — full classification

### Còn thiếu

#### TASK-05-04a · CWE-based Tags [✅ DONE]
```go
// pkg/classification/classification.go
func TagsFromCWE(cweIDs []string) []Tag {
    // CWE-89 → impact:sqli, attack:injection
    // CWE-79 → impact:xss
    // CWE-119 → impact:memory-safety
    // CWE-416 → impact:use-after-free (→ memory-safety)
    // ...
}
```
- [ ] Tạo CWE → Tag mapping table (top 25 CWEs)
- [ ] Integrate vào `Classify(vuln)` function
- [ ] Tests

#### TASK-05-04b · Package-based Tags [✅ DONE]
```go
func TagsFromPackages(affected []*osvschema.Affected) []Tag {
    // linux kernel → asset:kernel
    // apache httpd → asset:web-server
    // openssl → asset:crypto-library
    // iOS/Android → asset:mobile
}
```
- [ ] Implement keyword matching trên package names
- [ ] Tests

#### TASK-05-04c · Status Tags [✅ DONE]
- [ ] `status:kev` — khi KEV stage match
- [ ] `status:high-epss` — khi EPSS percentile > 0.95
- [ ] `status:exploit-public` — khi ExploitCheck tìm thấy PoC
- [ ] Integration với pipeline stages

#### TASK-05-04d · LLM-based Tags (Advanced) [✅ DONE]
- [ ] Prompt engineering cho MITRE ATT&CK technique tagging
- [ ] Prompt: "Given CVE description, identify MITRE ATT&CK techniques"
- [ ] Map ATT&CK techniques → tags
- [ ] Rate limit: Chỉ dùng LLM khi rule-based tags < 3

---

## TASK-05-05 · Daily EPSS Batch Update [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 1.5 ngày  
**Priority:** P1

### Mục tiêu
EPSS thay đổi hàng ngày. Batch update tất cả CVE có EPSS score trong DB.

### Implementation

```
Flow:
  Cloud Scheduler (06:00 UTC daily)
    → NATS "jobs.epss.daily_update"
      → ai-enrichment subscribes
        → Fetch all CVE IDs từ Firestore
        → Batch EPSS API (100 CVEs/request)
        → Update enriched records
        → Publish "vuln.enrichment.updated" cho changed CVEs
```

### Subtasks

- [ ] Tạo NATS subscriber cho `jobs.epss.daily_update`
- [ ] Implement batch query Firestore CVE IDs (pagination)
- [ ] Implement batch EPSS fetch + update
- [ ] Track thay đổi: chỉ update nếu score change > 0.01
- [ ] Metrics: `epss_daily_update_duration_seconds`, `epss_updated_count`, `epss_unchanged_count`
- [ ] Alerting nếu job fail

---

## TASK-05-06 · Alert cho High-Risk CVEs [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 1 ngày  
**Priority:** P1

### Trigger Conditions
```
Alert khi CVE mới match ANY of:
  - Mới xuất hiện trong KEV catalog
  - EPSS percentile > 0.95 (top 5%)
  - CVSS ≥ 9.0 (critical) + has_exploit = true
  - CVSS ≥ 9.0 + KEV + no_fix
```

### Subtasks

- [ ] Implement alert rules engine trong ai-enrichment
- [ ] Publish `vuln.alert.high_risk` NATS event khi conditions met
- [ ] notification-service subscribes và gửi alert
- [ ] Tests với synthetic test cases

---

## Sprint 05 Definition of Done

- [ ] KEV/EPSS data stored trong enriched records (Firestore + OpenSearch)
- [ ] CWE enrichment stage hoạt động
- [ ] Auto-tagging coverage: ≥ 80% CVEs có ít nhất 3 tags
- [ ] Daily EPSS update job chạy thành công
- [ ] High-risk CVE alerts được gửi trong < 5 phút sau ingest
- [ ] `go build ./services/ai-enrichment/...` pass
- [ ] `go test ./services/ai-enrichment/...` pass
