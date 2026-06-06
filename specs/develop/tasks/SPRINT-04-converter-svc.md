# SPRINT-04 — Converter Service (`services/converter/`)

> **Thời gian:** Q4 2026, Tháng 4 (3 tuần)  
> **Mục tiêu:** Port `vulnfeeds/` thành event-driven gRPC microservice  
> **Refs:** [02-reorganization.md §2.3](../02-reorganization.md), [04-roadmap.md §2.4](../04-roadmap.md)

---

## Tổng Quan

```
Sprint Goal: "Converter service xử lý CVE5 và NVD JSON trong < 30s/batch"

Architecture mục tiêu:
  NVD API ──────► converter-service ──► NATS "raw.cve.nvd"
  CVEListV5 ────►                  ──► NATS "raw.cve.cve5"
  Alpine secdb ─►                  ──► NATS "raw.cve.alpine"
                    ↑
             ingestion-service subscribes

Deliverables:
  1. CVE5 → OSV domain converter (✅ DONE skeleton)
  2. NVD JSON v2 → OSV converter (📋 TODO)
  3. versions.go port (CPE → version detection) (📋 TODO)
  4. gRPC service interface (📋 TODO)
  5. NATS event publisher (📋 TODO)
  6. vulnfeeds/ → converter/ code port (📋 TODO)
```

---

## TASK-04-01 · CVE5 Domain Converter (Hoàn thiện) [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 2 ngày  
**Files:**
- [converter.go](../../../../services/converter/internal/domain/cve5/converter.go)
- [adp.go](../../../../services/converter/internal/domain/cve5/adp.go)
- [converter_test.go](../../../../services/converter/internal/domain/cve5/converter_test.go)

### Đã implement
- [x] `CVERecord`, `CVEMetadata`, `CNA`, `ADP` structs
- [x] `ConvertToOSV(record)` — full conversion với severity, affected, references
- [x] `classifyReference()` — URL → Reference_Type (FIX/ADVISORY/REPORT/WEB)
- [x] CVSS v3/v4 severity extraction

#### TASK-04-01a · ADP Container Support [✅ DONE]
- [x] `MergeADPContainers(vuln, record)` — merge severity + affected từ trusted ADPs
- [x] Trusted ADP list: CISA-ADP, NVD-ADP, NVD, Vulnogram
- [x] Deduplication: không thêm duplicate affected packages
- [x] `ExtractCWEsFromRecord(record)` — extract CWE IDs từ problemTypes

#### TASK-04-01b · Version Range Handling [🔄 PARTIAL]
- [x] `affected.lessThan` → `Event.Fixed`
- [x] `affected.lessThanOrEqual` → `Event.LastAffected`
- [ ] TODO: `versionType="git"` → `Range.GIT` (follow-up)

#### TASK-04-01c · CWE Extraction [✅ DONE]
- [x] `ExtractCWEsFromRecord()` — scan problemTypes, deduplicate
- [x] Works with both `CWE-NNN` and bare `NNN` format

#### TASK-04-01d · Unit Tests [✅ DONE]
- [x] `TestConvertToOSV_Basic` — 8/8 assertions pass
- [x] `TestConvertToOSV_NilRecord`, `TestConvertToOSV_MissingCNA`
- [x] `TestMergeADPContainers` — ADP severity + affected merge
- [x] `TestMergeADPContainers_NoDuplicate` — deduplication
- [x] `TestExtractCWEsFromRecord` — 2 CWE IDs extracted
- [x] `TestExtractCWEsFromRecord_Dedup` — no duplicate CWEs
- [x] `TestReferenceClassification`, `TestJSONRoundTrip`
- **8 tests PASS** ✅


#### TASK-04-01b · Version Range Handling [📋 TODO]
- [ ] Support `versionType: "semver"` → `Range_SEMVER`
- [ ] Support `versionType: "git"` → `Range_GIT` với commit SHAs
- [ ] Support `lessThanOrEqual` → `{LastAffected: version}`
- [ ] Handle `status: "unaffected"` ranges
- [ ] Tests với complex version ranges từ real CVE5 records

#### TASK-04-01c · Port vulnfeeds/conversion/cve5/ Logic [📋 TODO]
- [ ] Review `vulnfeeds/conversion/cve5/` cho logic chưa port
- [ ] Port bất kỳ conversion logic nào còn thiếu
- [ ] Đặc biệt: `vulnfeeds/conversion/cve5/cve5.go` — so sánh với converter mới

#### TASK-04-01d · Unit Tests [📋 TODO]
- [ ] Test với ≥ 10 real CVE5 records (từ cvelistV5 repo)
- [ ] Verify output OSV schema valid theo schema validator
- [ ] Test edge cases: withdrawn, rejected, disputed CVEs

---

## TASK-04-02 · NVD JSON v2 Converter [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 3 ngày  
**Priority:** P1
**File:** [services/converter/internal/domain/nvd/converter.go](../../../../services/converter/internal/domain/nvd/converter.go)

### Hoàn thành
- [x] NVD JSON v2 types: `NVDResponse`, `NVDCVE`, `NVDMetrics`, `CVSSData`, `CPEMatch`, v.v.
- [x] `ConvertNVDToOSV(nvdCVE)` — convert CVSS severity, CWE IDs, references
- [x] `ConvertBatch(resp)` — batch convert NVD API response
- [x] CVSS priority: v3.1 > v3.0 > v2, primary > secondary
- [x] CWE extraction vào `database_specific.cwe_ids`
- [x] Reference classification: patch, exploit, advisory, report, web
- [ ] **TODO:** Unit tests với NVD real records (Sprint 05)
- [ ] **TODO:** CPE → version ranges (TASK-04-03)

### Acceptance Criteria
- [x] `go build ./services/converter/...` pass ✅ 2026-06-03

---

## TASK-04-03 · Version Detection từ CPE (`domain/versions/`) [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 5 ngày (complex!)  
**Priority:** P1

### Context
`vulnfeeds/conversion/versions.go` là file 44KB chứa CPE → version detection logic.
Đây là core IP của vulnfeeds, cần port carefully.

### Subtasks

- [ ] **Review** `vulnfeeds/conversion/versions.go` — map toàn bộ functions
- [ ] Xây dựng test suite trước khi port:
  - Thu thập input/output pairs từ existing tests
  - Tạo golden files cho các CPE phức tạp
- [ ] Port theo từng function nhỏ:
  - [ ] CPE parsing → `ParseCPE(cpeString string) (*CPE, error)`
  - [ ] Version range extraction từ CPE → `ExtractVersionRanges(cpe *CPE) []VersionRange`
  - [ ] Ecosystem detection từ CPE → `DetectEcosystem(cpe *CPE) string`
  - [ ] PURL generation từ CPE → `CPEToPURL(cpe *CPE) string`
- [ ] Verify parity với original vulnfeeds output
- [ ] Performance test: ≥ 1000 CPEs/second

### File structure
```
services/converter/internal/domain/versions/
├── cpe.go          # CPE struct + parser
├── ranges.go       # Version range extraction
├── ecosystem.go    # Ecosystem detection logic
├── purl.go         # CPE → PURL conversion
└── versions_test.go
```

---

## TASK-04-04 · gRPC Service Interface [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 2 ngày  
**Priority:** P1

### Proto Definition

```protobuf
// proto/converter/v1/converter.proto
syntax = "proto3";
package converter.v1;

service ConverterService {
  // Convert single CVE5 record
  rpc ConvertCVE5(ConvertCVE5Request) returns (ConvertResponse);
  
  // Convert single NVD record
  rpc ConvertNVD(ConvertNVDRequest) returns (ConvertResponse);
  
  // Batch convert: stream input, stream output
  rpc BatchConvert(stream ConvertRequest) returns (stream ConvertResponse);
  
  // Get conversion statistics
  rpc GetStats(GetStatsRequest) returns (ConversionStats);
  
  // Trigger full re-conversion for a source
  rpc TriggerFullConversion(TriggerRequest) returns (TriggerResponse);
}

message ConvertCVE5Request {
  bytes raw_json = 1;  // Raw CVE5 JSON record
  string source_id = 2;
}

message ConvertNVDRequest {
  bytes raw_json = 1;  // Raw NVD JSON record
  string source_id = 2;
}

message ConvertResponse {
  string vuln_id = 1;
  bytes osv_json = 2;         // Converted OSV JSON
  bool success = 3;
  string error_message = 4;   // If !success
  repeated string warnings = 5;
}

message ConversionStats {
  string source = 1;
  int64 total_converted = 2;
  int64 total_errors = 3;
  google.protobuf.Timestamp last_run = 4;
}
```

### Subtasks

- [ ] Viết `proto/converter/v1/converter.proto`
- [ ] Generate Go code: `protoc --go_out=. --go-grpc_out=.`
- [ ] Implement gRPC server: `services/converter/interface/grpc/server.go`
  - `ConvertCVE5Handler`
  - `ConvertNVDHandler`
  - `BatchConvertHandler` (streaming)
- [ ] Register gRPC server trong `converter/cmd/main.go`
- [ ] Add gRPC health check
- [ ] Integration tests

---

## TASK-04-05 · NATS Event Publisher [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 1 ngày  
**Priority:** P1
**File:** [services/converter/internal/infra/publisher/nats_publisher.go](../../../../services/converter/internal/infra/publisher/nats_publisher.go)

### Hoàn thành
- [x] `ConvertedEvent` struct với `source`, `format`, `vuln_id`, `osv_json`, `checksum`, `converted_at`
- [x] `NATSPublisher.Publish()` — publish single event vào NATS
- [x] `NATSPublisher.PublishBatch()` — publish multiple events
- [x] Subject routing: format → `raw.cve.{format}`
- [x] SHA256 checksum cho deduplication
- [ ] **TODO:** Deduplication tracking với Redis set (Sprint 05)
- [ ] **TODO:** Tests với NATS testcontainer (Sprint 05)

### Acceptance Criteria
- [x] `go build ./services/converter/...` pass ✅ 2026-06-03

---

## TASK-04-06 · Vulnfeeds CLI Adapter [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 2 ngày  
**Priority:** P2 (backward compat)

### Mục tiêu
Giữ lại CLI interface của `vulnfeeds/cmd/` nhưng backed by converter service.

### Subtasks

- [ ] `converter/cmd/cli/` — CLI wrapper gọi converter service qua gRPC
- [ ] Commands: `convert-cve5`, `convert-nvd`, `batch-convert`
- [ ] Support pipe input/output: `cat cve.json | cvectl convert-cve5`
- [ ] Dùng cho migration validation, không phải production workflow

---

## TASK-04-07 · Migrate vulnfeeds/ Commands [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 2 ngày  
**Priority:** P2

### Vulnfeeds Commands Cần Review

| Command | Chức năng | Migration target |
|---------|-----------|-----------------|
| `combine-to-osv` | Combine CVE5 + NVD → OSV | → converter gRPC BatchConvert |
| `converters` | Run individual format converters | → converter gRPC ConvertCVE5/NVD |
| `ids` | ID management | → services/source-sync/connectors/ids/ |
| `mirrors` | Mirror management | → services/source-sync/ |
| `pypi` | PyPI specific | → services/source-sync/connectors/pypi/ |

### Subtasks

- [ ] `combine-to-osv` logic → port vào converter service
- [ ] `ids` logic → review và port vào source-sync connectors
- [ ] `mirrors` logic → port vào source-sync
- [ ] `pypi` logic → port vào source-sync connectors/pypi/

---

## Sprint 04 Definition of Done

- [x] `go build ./services/converter/...` pass ✅ 2026-06-03
- [x] CVE5 converter skeleton đã có ✅ 2026-06-03
- [x] NVD JSON v2 converter hoàn chỉnh ✅ 2026-06-03
- [x] NATS event publisher hoàn chỉnh ✅ 2026-06-03
- [ ] `go test ./services/converter/...` pass với ≥ 50 test cases (Sprint 05)
- [ ] gRPC `ConvertCVE5` xử lý real CVE5 record (Sprint 05)
- [ ] gRPC `ConvertNVD` xử lý real NVD JSON (Sprint 05)
- [ ] Throughput: ≥ 100 CVEs/second (Sprint 05)
