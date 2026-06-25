# ✅ COMPLETED — CR-DD-003 — Finding Deduplication Engine

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DD-003 |
| **Tiêu đề** | Finding Deduplication Engine — Hash-based, Unique-ID, Legacy Algorithms |
| **Nguồn tham chiếu** | `django-DefectDojo/specs/services/04-scan-orchestrator-service.md §7`, `SRS.md §3.4 FR-DEDUP-01 to FR-DEDUP-04` |
| **Target Service** | `scan-orchestrator` (phần của CR-DD-002) |
| **Ưu tiên** | 🔴 High |
| **Loại** | Feature Addition |
| **Ngày tạo** | 2026-06-13 |

---

## 1. Tổng quan

Deduplication là tính năng cốt lõi của DefectDojo — đảm bảo rằng cùng một vulnerability không tạo ra nhiều Finding records khi scan lặp lại. OSV hiện tại **không có** deduplication — mỗi import đều tạo finding mới.

### Deduplication Algorithms

| Algorithm | Key | Use case |
|-----------|-----|---------|
| **Hash-based** | SHA256(severity+title+cwe+description+endpoints) | Default — tất cả scanners |
| **Unique ID** | `vuln_id_from_tool` (unique per scanner) | Scanners cung cấp stable ID |
| **Legacy** | endpoint_url + CWE/title matching | Backward compat |

---

## 2. Gap Analysis

| Feature | OSV | DefectDojo |
|---------|-----|-----------|
| Hash-based dedup | ❌ | ✅ SHA256 của 5 fields |
| Unique ID dedup | ❌ | ✅ Stable ID from scanner |
| Legacy dedup | ❌ | ✅ Endpoint + CWE matching |
| Product-scope dedup | ❌ | ✅ Tìm trong toàn product |
| Engagement-scope dedup | ❌ | ✅ Chỉ trong 1 engagement |
| False positive history | ❌ | ✅ Auto-mark new dupes as FP |
| Duplicate cap (max_dupes) | ❌ | ✅ Configurable (default 10) |

---

## 3. Domain Design

### 3.1 Dedup Domain

```go
// scan-orchestrator/internal/domain/dedup/service.go

// DeduplicationService — port cho dedup engine
type DeduplicationService interface {
    Deduplicate(
        ctx context.Context,
        newFindings []*parser.ParsedFinding,
        dedupCtx *DedupContext,
    ) (*DedupResult, error)
}

// DedupContext — context cho dedup computation
// Mirrors Python: dojo/importers/base_importer.py dedup_context
type DedupContext struct {
    TestID               string
    EngagementID         string
    ProductID            string
    OnEngagement         bool   // deduplication_on_engagement flag
    Algorithm            DedupAlgorithm
    FalsePositiveHistory bool   // auto-mark same-hash FPs
    MaxDuplicates        int    // default: 10 (0 = delete all)
}

type DedupAlgorithm string
const (
    DedupAlgorithmHashCode  DedupAlgorithm = "hash_code"
    DedupAlgorithmUniqueID  DedupAlgorithm = "unique_id_from_tool"
    DedupAlgorithmLegacy    DedupAlgorithm = "legacy"
)

// DedupResult — output của dedup process
type DedupResult struct {
    NewFindings        []*parser.ParsedFinding  // Không có duplicate → cần tạo mới
    DuplicateFindings  []*parser.ParsedFinding  // Có duplicate → bỏ qua hoặc track
    Reactivated        []*ReactivatedFinding     // Đã closed → cần reactivate
    Untouched          []*ExistingFinding        // Không thay đổi
}

type ReactivatedFinding struct {
    ExistingFindingID string
    ParsedFinding     *parser.ParsedFinding
}

type ExistingFinding struct {
    FindingID string
    HashCode  string
}
```

### 3.2 Algorithm Implementations

```go
// scan-orchestrator/internal/infra/dedup/hash_code.go

// HashCodeDeduplicator — default algorithm
// Mirrors Python: dojo/utils.py::hash_code_fields_per_scanner()
// và dojo/dedupe/dupefinder.py::DupeFinder.find_duplicates_hash_code()

type HashCodeDeduplicator struct {
    findingClient findingv1.FindingServiceClient
}

// Deduplicate checks each new finding against existing findings in same scope
func (d *HashCodeDeduplicator) Deduplicate(
    ctx context.Context,
    newFindings []*parser.ParsedFinding,
    dedupCtx *dedup.DedupContext,
) (*dedup.DedupResult, error) {

    result := &dedup.DedupResult{}

    for _, pf := range newFindings {
        // 1. Compute hash_code (SHA256 of key fields)
        hashCode := computeHashCode(pf)
        pf.HashCode = hashCode

        // 2. Query Finding Service for existing finding with same hash
        existing, err := d.findingClient.FindByHashCode(ctx, &findingv1.FindByHashCodeRequest{
            HashCode:     hashCode,
            TestId:       dedupCtx.TestID,
            EngagementId: &dedupCtx.EngagementID,
            ProductId:    &dedupCtx.ProductID,
            OnEngagement: dedupCtx.OnEngagement,
        })

        if err != nil || existing == nil {
            // No duplicate → new finding
            result.NewFindings = append(result.NewFindings, pf)
            continue
        }

        // 3. Handle duplicate
        switch existing.Status {
        case "mitigated":
            // Previously closed but reappeared → reactivate
            result.Reactivated = append(result.Reactivated, &dedup.ReactivatedFinding{
                ExistingFindingID: existing.FindingId,
                ParsedFinding:     pf,
            })
        case "false_positive":
            if dedupCtx.FalsePositiveHistory {
                // Auto-mark as FP based on history
                pf.FalsePositive = true
                result.NewFindings = append(result.NewFindings, pf)
            }
            // else: skip (duplicate of known FP)
        default:
            // Active duplicate → untouched
            result.Untouched = append(result.Untouched, &dedup.ExistingFinding{
                FindingID: existing.FindingId,
                HashCode:  hashCode,
            })
        }
    }

    return result, nil
}

// computeHashCode — Go port của Django hash_code computation
// Mirrors Python: dojo/utils.py::finding_helper::hash_finding_fields()
//
// Hash formula: SHA256(severity|title|cwe|description[:256]|sorted_endpoints)
func computeHashCode(f *parser.ParsedFinding) string {
    h := sha256.New()

    parts := []string{
        strings.ToLower(string(f.Severity)),
        strings.ToLower(f.Title),
    }

    if f.CWE != 0 {
        parts = append(parts, fmt.Sprintf("%d", f.CWE))
    }

    if len(f.Description) > 0 {
        maxDesc := f.Description
        if len(maxDesc) > 256 {
            maxDesc = maxDesc[:256]
        }
        parts = append(parts, maxDesc)
    }

    // Sort endpoints for deterministic hash
    endpoints := make([]string, len(f.UnsavedEndpoints))
    copy(endpoints, f.UnsavedEndpoints)
    sort.Strings(endpoints)
    parts = append(parts, endpoints...)

    io.WriteString(h, strings.Join(parts, "|"))
    return fmt.Sprintf("%x", h.Sum(nil))
}
```

```go
// scan-orchestrator/internal/infra/dedup/unique_id.go

// UniqueIDDeduplicator — dùng khi scanner cung cấp stable unique ID
// Mirrors Python: dojo/dedupe/dupefinder.py::DupeFinder.find_duplicates_unique_id()
//
// Ví dụ scanners có unique ID: Snyk, Checkmarx, Veracode, Nuclei, SonarQube

type UniqueIDDeduplicator struct {
    findingClient findingv1.FindingServiceClient
}

func (d *UniqueIDDeduplicator) Deduplicate(
    ctx context.Context,
    newFindings []*parser.ParsedFinding,
    dedupCtx *dedup.DedupContext,
) (*dedup.DedupResult, error) {

    result := &dedup.DedupResult{}

    for _, pf := range newFindings {
        if pf.UniqueIDFromTool == "" {
            // Fallback to hash_code if no unique ID
            result.NewFindings = append(result.NewFindings, pf)
            continue
        }

        existing, err := d.findingClient.FindByUniqueID(ctx, &findingv1.FindByUniqueIDRequest{
            UniqueIdFromTool: pf.UniqueIDFromTool,
            ProductId:        dedupCtx.ProductID,
            EngagementId:     &dedupCtx.EngagementID,
            OnEngagement:     dedupCtx.OnEngagement,
        })

        if err != nil || existing == nil {
            result.NewFindings = append(result.NewFindings, pf)
            continue
        }

        // Handle same as hash_code algorithm
        // ...
    }

    return result, nil
}
```

```go
// scan-orchestrator/internal/infra/dedup/legacy.go

// LegacyDeduplicator — endpoint URL + CWE/title matching
// Mirrors Python: dojo/dedupe/dupefinder.py::DupeFinder.find_duplicates_legacy()
// Dùng cho backward compatibility với old parsers

type LegacyDeduplicator struct {
    findingClient findingv1.FindingServiceClient
}

func (d *LegacyDeduplicator) Deduplicate(
    ctx context.Context,
    newFindings []*parser.ParsedFinding,
    dedupCtx *dedup.DedupContext,
) (*dedup.DedupResult, error) {

    // Match by: endpoints URL + CWE + title (fuzzy)
    // If CWE matches and endpoint URL partially matches → duplicate
    // ...
}
```

---

## 4. Duplicate Management

### 4.1 Max Duplicates

```go
// After deduplication: enforce max_dupes limit
// Mirrors Python: dojo/settings/settings.dist.py::DUPLICATE_CLUSTER_CASCADE_DELETE

type DuplicateManager struct {
    findingClient findingv1.FindingServiceClient
    maxDuplicates int // default: 10
}

// EnforceMaxDuplicates: xóa oldest duplicates khi vượt giới hạn
func (dm *DuplicateManager) Enforce(ctx context.Context, originalFindingID string) error {
    duplicates, err := dm.findingClient.ListDuplicates(ctx, &findingv1.ListDuplicatesRequest{
        OriginalFindingId: originalFindingID,
        OrderBy:           "date ASC",  // oldest first
    })

    if len(duplicates) <= dm.maxDuplicates {
        return nil // No cleanup needed
    }

    // Delete oldest duplicates
    toDelete := duplicates[:len(duplicates)-dm.maxDuplicates]
    for _, dup := range toDelete {
        dm.findingClient.DeleteFinding(ctx, &findingv1.DeleteFindingRequest{Id: dup.Id})
    }

    return nil
}
```

### 4.2 False Positive History

```go
// false_positive_history: khi finding mới có cùng hash với finding đã mark FP
// → tự động mark finding mới là FP
// Mirrors Python: dojo/dedupe/dupefinder.py::handle_false_positive_history()

func (d *HashCodeDeduplicator) checkFalsePositiveHistory(
    ctx context.Context,
    hashCode string,
    productID string,
) bool {
    // Query finding-service: có finding nào cùng hash, cùng product, và false_p=true?
    exists, _ := d.findingClient.ExistsFalsePositiveByHash(ctx, &findingv1.ExistsFPByHashRequest{
        HashCode:  hashCode,
        ProductId: productID,
    })
    return exists.Exists
}
```

---

## 5. Algorithm Selection Logic

```go
// Chọn dedup algorithm dựa trên scanner type
// Mirrors Python: dojo/settings/settings.dist.py::DEDUPLICATION_ALGORITHM_PER_PARSER

var dedupAlgorithmPerParser = map[string]DedupAlgorithm{
    // Scanners with stable unique IDs
    "Snyk Scan":              DedupAlgorithmUniqueID,
    "SonarQube Scan":         DedupAlgorithmUniqueID,
    "Checkmarx Scan":         DedupAlgorithmUniqueID,
    "Veracode Scan":          DedupAlgorithmUniqueID,
    "Nuclei Scan":            DedupAlgorithmUniqueID,
    "Dependency-Check Scan":  DedupAlgorithmUniqueID,

    // Default (all others): hash_code
    // DedupAlgorithmHashCode is the fallback
}

func getDedupAlgorithm(scanType string) DedupAlgorithm {
    if algo, ok := dedupAlgorithmPerParser[scanType]; ok {
        return algo
    }
    return DedupAlgorithmHashCode // default
}
```

---

## 6. gRPC Extensions (Finding Service)

Để dedup hoạt động, `finding-service` cần thêm các RPCs:

```protobuf
// Extensions to finding.proto (requires changes to finding-service)
service FindingService {
    // Existing RPCs...

    // NEW for deduplication:
    rpc FindByHashCode(FindByHashCodeRequest) returns (FindByHashCodeResponse);
    rpc FindByUniqueID(FindByUniqueIDRequest) returns (FindByUniqueIDResponse);
    rpc ListDuplicates(ListDuplicatesRequest) returns (ListDuplicatesResponse);
    rpc ExistsFalsePositiveByHash(ExistsFPByHashRequest) returns (ExistsFPByHashResponse);
    rpc ReactivateFindings(ReactivateFindingsRequest) returns (ReactivateFindingsResponse);
}

message FindByHashCodeRequest {
    string hash_code    = 1;
    string test_id      = 2;
    optional string engagement_id = 3;
    optional string product_id    = 4;
    bool on_engagement  = 5;
}

message FindByHashCodeResponse {
    optional string finding_id = 1;
    optional string status     = 2;  // active|mitigated|false_positive
}

message FindByUniqueIDRequest {
    string unique_id_from_tool = 1;
    string product_id          = 2;
    optional string engagement_id = 3;
    bool on_engagement = 4;
}

message ExistsFPByHashRequest {
    string hash_code  = 1;
    string product_id = 2;
}

message ExistsFPByHashResponse {
    bool exists = 1;
}
```

---

## 7. System Settings

```yaml
# scan-orchestrator config.yaml
deduplication:
  # Default algorithm (nếu không có per-scanner config)
  default_algorithm: "hash_code"

  # False positive history
  false_positive_history: false

  # Max duplicates to keep (0 = delete all)
  max_duplicates: 10

  # Delete oldest duplicates automatically
  delete_duplicates: false
```

---

## 8. Acceptance Criteria

- [x] Import cùng scan file 2 lần → lần 2 tất cả findings là `untouched`
- [x] Import scan với finding mới → `created=1`, finding cũ không có → `closed=1`
- [x] Dedup trong product scope: finding ở Test A = finding ở Test B → duplicate
- [x] Dedup trong engagement scope: finding ở Test A ≠ finding ở Test B (khác engagement)
- [x] `vuln_id_from_tool` dedup: Snyk scan với cùng ID → không tạo mới
- [x] Finding đã mitigated nhưng xuất hiện lại → `reactivated=1`
- [x] `false_positive_history=true`: finding hash trùng với known FP → auto-mark FP
- [x] `max_duplicates=5`: khi có 6 duplicates, oldest 1 bị xóa tự động
- [x] `max_duplicates=0`: tất cả duplicates bị xóa
- [x] Hash computation đồng nhất: cùng finding data → cùng hash mọi lần

## Implementation Status: ✅ DONE

> `scan-service/internal/infra/dedup/engine.go` — 3 algorithms: hash_code (SHA256 of severity+title+cwe+description[:256]+sorted_endpoints), unique_id_from_tool, legacy endpoint matching
> `scan-service/internal/infra/dedup/` — DedupResult: NewFindings, Reactivated, Untouched, DuplicateFindings
> `finding-service/internal/delivery/grpc/server/finding_server.go` — ExistsFalsePositiveByHash, FindByHashCode, FindByUniqueID gRPC extensions
> Algorithm selection per scanner type: Snyk/SonarQube/Checkmarx/Veracode/Nuclei → unique_id; others → hash_code
> MaxDuplicates enforcement: ListDuplicates + delete oldest
