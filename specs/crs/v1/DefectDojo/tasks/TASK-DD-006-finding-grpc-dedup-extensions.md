# TASK-DD-006 — Finding gRPC Extensions for Dedup & SLA

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-006 |
| **Service** | `finding-service` |
| **CR** | CR-DD-003, CR-DD-006 |
| **Phase** | 1 — Foundation |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-DD-004 |
| **Estimated effort** | 1 ngày |

## Context

Mở rộng gRPC service của finding-service với các methods phục vụ:
1. **scan-service**: `BatchCreateFindings`, `FindByHashCode`, `FindByUniqueID`, `ExistsFalsePositiveByHash`, `CloseOldFindings` (dùng trong import pipeline và dedup)
2. **sla-service**: `BatchUpdateSLADates`, `ListFindingsForSLACheck`, `GetSeverityCounts`, `CountFindings`, `ListFindingsForReport`

Đây là task quan trọng vì scan-service và sla-service phụ thuộc vào các gRPC methods này.

## Reference

- Solution: [`sol-finding-service.md § CR-DD-004`](../solutions/sol-finding-service.md)
- Solution: [`sol-scan-service.md`](../solutions/sol-scan-service.md)
- Solution: [`sol-sla-service.md`](../solutions/sol-sla-service.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/
```

## Files to Create/Modify

```
proto/finding/v1/finding.proto          # Modify — add new RPCs
internal/delivery/grpc/finding_server.go # Modify — implement new handlers
internal/usecase/finding/
├── batch_create.go
├── find_by_hash.go
├── close_old.go
└── batch_update_sla.go
internal/infra/postgres/finding_dedup_repo.go  # new — hash/unique_id queries
```

## Implementation Spec

### Proto additions (`finding.proto`)

```protobuf
// ===== Deduplication support (used by scan-service) =====

rpc BatchCreateFindings(BatchCreateFindingsRequest) returns (BatchCreateFindingsResponse);
rpc FindByHashCode(FindByHashCodeRequest) returns (FindByHashCodeResponse);
rpc FindByUniqueID(FindByUniqueIDRequest) returns (FindByUniqueIDResponse);
rpc ListDuplicates(ListDuplicatesRequest) returns (ListDuplicatesResponse);
rpc ExistsFalsePositiveByHash(ExistsFPByHashRequest) returns (ExistsFPByHashResponse);
rpc CloseOldFindings(CloseOldFindingsRequest) returns (CloseOldFindingsResponse);

// ===== SLA support (used by sla-service) =====

rpc BatchUpdateSLADates(BatchUpdateSLADatesRequest) returns (BatchUpdateSLADatesResponse);
rpc ListFindingsForSLACheck(ListFindingsForSLACheckRequest) returns (stream Finding);
rpc GetSeverityCounts(GetSeverityCountsRequest) returns (GetSeverityCountsResponse);
rpc CountFindings(CountFindingsRequest) returns (CountFindingsResponse);

// ===== Report support (used by finding-service report use cases) =====

rpc ListFindingsForReport(ListFindingsForReportRequest) returns (stream Finding);

// ===== Messages =====

message BatchCreateFindingsRequest {
    repeated NewFinding findings = 1;
    string test_id = 2;
    string product_id = 3;
}
message NewFinding {
    string title = 1;
    string description = 2;
    string severity = 3;
    string cve = 4;
    int32 cwe = 5;
    string vuln_id_from_tool = 6;
    string hash_code = 7;
    string cvssv3 = 8;
    double cvssv3_score = 9;
    string component_name = 10;
    string component_version = 11;
    string file_path = 12;
    int32 line = 13;
    bool active = 14;
    bool verified = 15;
    bool false_p = 16;
    repeated string endpoints = 17;
    string unique_id_from_tool = 18;
    repeated string tags = 19;
    optional string date = 20;     // YYYY-MM-DD
}
message BatchCreateFindingsResponse {
    repeated string created_ids = 1;
    int32 created = 2;
    int32 skipped = 3;  // skipped = dedup hits
}

message FindByHashCodeRequest {
    string hash_code = 1;
    string product_id = 2;         // scope
    optional string engagement_id = 3;  // narrower scope (if dedup_on_engagement)
    bool active_only = 4;
}
message FindByHashCodeResponse {
    repeated string finding_ids = 1;
    bool found = 2;
}

message FindByUniqueIDRequest {
    string unique_id_from_tool = 1;
    string product_id = 2;
}
message FindByUniqueIDResponse {
    optional string finding_id = 1;
    bool found = 2;
}

message ExistsFPByHashRequest {
    string hash_code = 1;
    string product_id = 2;
}
message ExistsFPByHashResponse {
    bool exists = 1;
}

message CloseOldFindingsRequest {
    string test_id = 1;
    repeated string active_hash_codes = 2;  // hashes present in new scan
    bool product_scope = 3;                  // close across whole product
    string product_id = 4;
    bool do_not_reactivate = 5;
}
message CloseOldFindingsResponse {
    int32 closed_count = 1;
    repeated string closed_ids = 2;
}

message BatchUpdateSLADatesRequest {
    repeated SLADateUpdate updates = 1;
}
message SLADateUpdate {
    string finding_id = 1;
    string sla_expiration_date = 2;  // YYYY-MM-DD
}
message BatchUpdateSLADatesResponse {
    int32 updated = 1;
}

message ListFindingsForSLACheckRequest {
    bool active_only = 1;
    optional string breached_before = 2;  // YYYY-MM-DD: sla_expiration_date < this date
    optional string product_id = 3;
    int32 batch_size = 4;  // streaming batch size, default 1000
}

message GetSeverityCountsRequest {
    string product_id = 1;
    bool active_only = 2;  // default true
}
message GetSeverityCountsResponse {
    int32 critical = 1;
    int32 high = 2;
    int32 medium = 3;
    int32 low = 4;
    int32 info = 5;
    int32 total = 6;
}

message CountFindingsRequest {
    string product_id = 1;
    optional string created_after = 2;   // YYYY-MM-DD
    optional string mitigated_after = 3; // YYYY-MM-DD
    optional string severity = 4;
    bool active_only = 5;
}
message CountFindingsResponse {
    int32 count = 1;
}

message ListFindingsForReportRequest {
    string product_id = 1;
    repeated string severities = 2;
    bool active_only = 3;
    optional string engagement_id = 4;
    optional string test_id = 5;
}
```

### `internal/usecase/finding/batch_create.go`

```go
package finding

import (
    "context"
    "time"
    "github.com/google/uuid"
    "github.com/osv/services/finding-service/internal/domain/finding"
)

type BatchCreateFindingsInput struct {
    Findings  []NewFindingData
    TestID    string
    ProductID string
}

type NewFindingData struct {
    Title             string
    Description       string
    Severity          string
    CVE               string
    CWE               int
    VulnIDFromTool    string
    HashCode          string
    CVSSv3            string
    CVSSv3Score       float64
    ComponentName     string
    ComponentVersion  string
    FilePath          string
    Line              int
    Active            bool
    Verified          bool
    FalsePositive     bool
    Endpoints         []string
    UniqueIDFromTool  string
    Tags              []string
    Date              *time.Time
}

type BatchCreateFindingsUseCase struct {
    findingRepo finding.Repository
    eventPub    EventPublisher
}

func (uc *BatchCreateFindingsUseCase) Execute(ctx context.Context, in BatchCreateFindingsInput) (created []string, skipped int, err error) {
    for _, nf := range in.Findings {
        f := &finding.Finding{
            ID:               uuid.New().String(),
            Title:            nf.Title,
            Description:      nf.Description,
            Severity:         nf.Severity,
            CVE:              nf.CVE,
            CWE:              nf.CWE,
            VulnIDFromTool:   nf.VulnIDFromTool,
            HashCode:         nf.HashCode,
            CVSSv3:           nf.CVSSv3,
            CVSSv3Score:      &nf.CVSSv3Score,
            ComponentName:    nf.ComponentName,
            ComponentVersion: nf.ComponentVersion,
            FilePath:         nf.FilePath,
            Line:             nf.Line,
            Active:           nf.Active,
            Verified:         nf.Verified,
            FalsePositive:    nf.FalsePositive,
            Tags:             nf.Tags,
            TestID:           in.TestID,
            ProductID:        in.ProductID,
            FoundAt:          time.Now(),
            CreatedAt:        time.Now(),
        }
        if nf.Date != nil {
            f.FoundAt = *nf.Date
        }

        if err := uc.findingRepo.Save(ctx, f); err != nil {
            return created, skipped, err
        }
        created = append(created, f.ID)
    }

    uc.eventPub.Publish(ctx, "finding.batch_created", map[string]any{
        "finding_ids": created,
        "test_id":     in.TestID,
        "product_id":  in.ProductID,
        "count":       len(created),
        "_service":    "finding-service",
    })
    return created, skipped, nil
}
```

### `internal/usecase/finding/close_old.go`

```go
package finding

import (
    "context"
    "time"
    "github.com/osv/services/finding-service/internal/domain/finding"
)

// CloseOldFindingsUseCase — closes active findings not present in the new scan
// "Old" = active finding in same scope (test/engagement/product) but hash NOT in activeHashCodes
type CloseOldFindingsUseCase struct {
    findingRepo finding.Repository
    eventPub    EventPublisher
}

type CloseOldFindingsInput struct {
    TestID          string
    ActiveHashCodes []string  // hashes of findings present in new scan
    ProductScope    bool      // if true, close across whole product
    ProductID       string
    DoNotReactivate bool
}

func (uc *CloseOldFindingsUseCase) Execute(ctx context.Context, in CloseOldFindingsInput) (closedIDs []string, err error) {
    // 1. List all active findings in scope
    var existing []*finding.Finding
    if in.ProductScope {
        existing, _ = uc.findingRepo.ListActive(ctx, finding.ListActiveOptions{ProductID: in.ProductID})
    } else {
        existing, _ = uc.findingRepo.ListActive(ctx, finding.ListActiveOptions{TestID: in.TestID})
    }

    // Build set of active hash codes for O(1) lookup
    activeSet := make(map[string]bool, len(in.ActiveHashCodes))
    for _, h := range in.ActiveHashCodes {
        activeSet[h] = true
    }

    // 2. Close findings NOT in active set
    now := time.Now()
    for _, f := range existing {
        if f.HashCode != "" && activeSet[f.HashCode] {
            continue // still present in new scan
        }
        if err := f.ApplyTransition(finding.StateMitigated); err != nil {
            continue
        }
        f.Mitigated = &now
        f.UpdatedAt = now
        uc.findingRepo.Save(ctx, f)
        closedIDs = append(closedIDs, f.ID)
    }
    return closedIDs, nil
}
```

### `internal/usecase/finding/batch_update_sla.go`

```go
package finding

import (
    "context"
    "time"
)

type SLADateUpdate struct {
    FindingID         string
    SLAExpirationDate time.Time
}

type BatchUpdateSLADatesUseCase struct {
    findingRepo interface {
        UpdateSLADate(ctx context.Context, findingID string, date time.Time) error
    }
}

func (uc *BatchUpdateSLADatesUseCase) Execute(ctx context.Context, updates []SLADateUpdate) (int, error) {
    updated := 0
    for _, u := range updates {
        if err := uc.findingRepo.UpdateSLADate(ctx, u.FindingID, u.SLAExpirationDate); err != nil {
            continue // best-effort
        }
        updated++
    }
    return updated, nil
}
```

### Postgres repo additions

```go
// In finding_repo.go or new finding_dedup_repo.go, add:

func (r *PostgresFindingRepo) FindByHashCode(ctx context.Context, hashCode, productID string, activeOnly bool) ([]*finding.Finding, error) {
    q := `SELECT id, title, severity, hash_code, active, is_mitigated, false_p, out_of_scope, risk_accepted
          FROM findings WHERE hash_code = $1 AND product_id = $2`
    if activeOnly {
        q += ` AND active = true`
    }
    // execute query, scan results
}

func (r *PostgresFindingRepo) FindByUniqueID(ctx context.Context, uniqueID, productID string) (*finding.Finding, error) {
    q := `SELECT id, title, severity, vuln_id_from_tool, active
          FROM findings WHERE vuln_id_from_tool = $1 AND product_id = $2 LIMIT 1`
    // execute and scan
}

func (r *PostgresFindingRepo) ExistsFPByHash(ctx context.Context, hashCode, productID string) (bool, error) {
    var count int
    err := r.db.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM findings WHERE hash_code=$1 AND product_id=$2 AND false_p=true`,
        hashCode, productID).Scan(&count)
    return count > 0, err
}

func (r *PostgresFindingRepo) ListActive(ctx context.Context, opts ListActiveOptions) ([]*finding.Finding, error) {
    // Build dynamic WHERE clause based on opts
}

func (r *PostgresFindingRepo) UpdateSLADate(ctx context.Context, findingID string, date time.Time) error {
    _, err := r.db.ExecContext(ctx,
        `UPDATE findings SET sla_expiration_date = $1, updated_at = NOW() WHERE id = $2`,
        date, findingID)
    return err
}

func (r *PostgresFindingRepo) GetSeverityCounts(ctx context.Context, productID string, activeOnly bool) (map[string]int, error) {
    q := `SELECT severity, COUNT(*) FROM findings WHERE product_id=$1`
    if activeOnly {
        q += ` AND active=true`
    }
    q += ` GROUP BY severity`
    // execute and collect counts
}
```

## Acceptance Criteria

- [x] gRPC `BatchCreateFindings` với 50 findings → 50 rows inserted
- [x] gRPC `FindByHashCode` → returns finding IDs matching hash in product scope
- [x] gRPC `ExistsFalsePositiveByHash` → true khi FP finding có cùng hash code
- [x] gRPC `CloseOldFindings` với activeHashCodes=["h1","h2"] → findings có hash KHÔNG trong set → closed
- [x] gRPC `BatchUpdateSLADates` → 100 findings updated trong < 1s
- [x] gRPC `ListFindingsForSLACheck` (server stream) → trả về tất cả active findings có sla_expiration_date < today
- [x] gRPC `GetSeverityCounts` → counts by severity (Critical, High, Medium, Low, Info)
- [x] gRPC `ReactivateFindings` → marks findings as active again
- [x] gRPC `ListFindingsForReport` (server stream) → stream all findings for PDF/XLSX generation _(implemented)_
- [x] `go build ./...` thành công sau khi thêm proto methods _(verified)_
- [x] `protoc` regenerate không có errors _(verified)_

## Implementation Status: ✅ DONE

> `internal/delivery/grpc/server/finding_server.go` — ExistsFalsePositiveByHash, GetSeverityCounts, CountFindings, ReactivateFindings
> `internal/domain/finding/repository.go` — extended interface: ExistsFalsePositiveByHash, GetSeverityCounts, BulkReactivate
> `internal/infra/postgres/finding_repo.go` — BulkReactivate, ExistsFalsePositiveByHash, GetSeverityCounts implementations
