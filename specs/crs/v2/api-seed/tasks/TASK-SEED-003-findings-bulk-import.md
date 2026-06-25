# TASK-SEED-003: Findings Bulk Create & Import (finding-service + gateway)

> **Solution:** [SOL-SEED-003](../solutions/SOL-SEED-003-findings-seed.md)  
> **Service:** `services/finding-service` + `apps/osv`  
> **Depends on:** TASK-SEED-002 (cần test_id để link findings)  
> **Blocking:** Không có  
> **Status:** ✅ COMPLETED — 2026-06-18 (gateway routes verified 2026-06-19)  
> **Files tạo/sửa:**  
> - `internal/usecase/findingbulk/usecase.go` (NEW)  
> - `internal/delivery/http/finding_seed_handler.go` (NEW)  
> - `internal/delivery/http/router.go` (thêm SEED-003 routes)  
> - `apps/osv/internal/gateway/router.go` (thêm SEED-003 gateway routes: bulk-create, import)

## Mục tiêu

Thêm `POST /api/v2/findings/bulk-create` và `POST /api/v2/findings/import` vào finding-service, tái dùng SHA-256 dedup + SLA auto-compute pipeline hiện có.

## Bước 1: Khảo sát code hiện tại

```bash
# Tìm deduplication logic
find /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service \
  -name "*.go" | xargs grep -l "computeHash\|sha256\|dedup" 2>/dev/null

# Tìm SLA logic
find /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service \
  -name "*.go" | xargs grep -l "sla_expiration\|SLAExpiration\|slaConfig" 2>/dev/null

# Xem finding entity
find /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/domain \
  -name "*.go" | xargs grep -l "type Finding struct"

# Xem existing bulk handler (nếu có)
grep -rn "bulk\|Bulk" \
  /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery \
  --include="*.go" | head -20
```

## Bước 2: Thêm entity types

**File:** `internal/domain/entity/finding.go`

```go
// FindingCreateInput là input cho bulk create / import
type FindingCreateInput struct {
    Title            string
    Description      string
    Mitigation       string
    Severity         string    // Critical|High|Medium|Low|Info
    CveID            string
    Cwe              int
    CvssV3Score      *float64
    ComponentName    string
    ComponentVersion string
    Date             time.Time // default: time.Now()
    Tags             []string
    Notes            []FindingNoteInput
    TestID           uuid.UUID
}

// FindingNoteInput là ghi chú kèm finding
type FindingNoteInput struct {
    Content   string
    IsPrivate bool
}

// BulkCreateOptions là tùy chọn cho bulk operation
type BulkCreateOptions struct {
    AutoCloseDuplicates bool
    AutoEnrichCVE       bool
    ComputeSLA          bool   // default: true
    MinimumSeverity     string // "Critical"|"High"|"Medium"|"Low"|"" (no filter)
}

// BulkFindingResult là kết quả per-item
type BulkFindingResult struct {
    Index    int
    Status   string     // "created" | "duplicate" | "error"
    ID       *uuid.UUID
    HashCode string
    Message  string
}
```

## Bước 3: Thêm methods vào FindingRepository

**File:** `internal/domain/repository/finding_repository.go`

```go
// Thêm nếu chưa có:
FindByHashAndProduct(ctx context.Context, hash string, productID uuid.UUID) (*entity.Finding, error)
CreateWithNotes(ctx context.Context, f *entity.Finding, notes []entity.FindingNoteInput) error
```

## Bước 4: Tạo UseCase `finding_bulk`

**File:** `internal/usecase/finding_bulk/usecase.go` (NEW)

```go
package findingbulk

type UseCase struct {
    findingRepo repository.FindingRepository
    testRepo    repository.TestRepository   // để load Test → Engagement → Product chain
    slaRepo     repository.SLAConfigRepository
    aiBaseURL   string
    httpClient  *http.Client
    eventPub    events.Publisher
}

// BulkCreate là main method
func (uc *UseCase) BulkCreate(ctx context.Context, testID uuid.UUID, inputs []entity.FindingCreateInput, opts entity.BulkCreateOptions) (BulkResult, error) {
    // 1. Load test → engagement → product_id
    // 2. Load SLA config cho product
    // 3. For each input:
    //    a. Filter by MinimumSeverity
    //    b. Compute SHA-256 hash: title+component_name+component_version+cve_id
    //    c. Dedup check: FindByHashAndProduct
    //    d. Compute sla_expiration_date
    //    e. CreateWithNotes
    //    f. Append result
    // 4. Publish NATS: finding.batch_created
    // 5. If AutoEnrichCVE: goroutine → POST ai-service per CVE
}

// ImportFromJSON parse JSON array → BulkCreate
func (uc *UseCase) ImportFromJSON(ctx context.Context, testID uuid.UUID, r io.Reader, opts entity.BulkCreateOptions) (ImportResult, error)

// ImportFromCSV parse CSV → BulkCreate
func (uc *UseCase) ImportFromCSV(ctx context.Context, testID uuid.UUID, r io.Reader, opts entity.BulkCreateOptions) (ImportResult, error)
```

**SHA-256 Dedup** (phải match algorithm hiện có — đọc code trước khi implement):

```go
func computeHash(title, componentName, componentVersion, cveID string) string {
    data := title + componentName + componentVersion + cveID
    sum := sha256.Sum256([]byte(data))
    return hex.EncodeToString(sum[:])
}
```

## Bước 5: CSV Parser

**File:** `internal/usecase/finding_bulk/csv_parser.go` (NEW)

```
CSV header (case-insensitive):
title, severity, cve, cwe, description, mitigation, component_name, component_version
```

```go
func ParseFindingCSV(r io.Reader) ([]entity.FindingCreateInput, []error)
```

## Bước 6: Thêm handlers

**File:** `internal/delivery/http/finding_handler.go`

```go
// BulkCreateFindings handles POST /api/v2/findings/bulk-create
func (h *Handler) BulkCreateFindings(w http.ResponseWriter, r *http.Request) {
    var req struct {
        TestID              uuid.UUID                   `json:"test_id"`
        Findings            []entity.FindingCreateInput `json:"findings"`
        AutoCloseDuplicates bool                        `json:"auto_close_duplicates"`
        AutoEnrichCVE       bool                        `json:"auto_enrich_cve"`
        MinimumSeverity     string                      `json:"minimum_severity"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    opts := entity.BulkCreateOptions{
        AutoCloseDuplicates: req.AutoCloseDuplicates,
        AutoEnrichCVE:       req.AutoEnrichCVE,
        ComputeSLA:          true,
        MinimumSeverity:     req.MinimumSeverity,
    }
    result, err := h.bulkUC.BulkCreate(r.Context(), req.TestID, req.Findings, opts)
    if err != nil {
        writeJSON(w, 500, errResp("internal", err.Error()))
        return
    }
    writeJSON(w, http.StatusMultiStatus, result)
}

// ImportFindings handles POST /api/v2/findings/import
func (h *Handler) ImportFindings(w http.ResponseWriter, r *http.Request) {
    r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
    if err := r.ParseMultipartForm(10 << 20); err != nil {
        writeJSON(w, 413, errResp("payload_too_large", "file exceeds 10MB"))
        return
    }
    testID, _ := uuid.Parse(r.FormValue("test_id"))
    format := r.FormValue("format") // "json" | "csv"
    file, _, _ := r.FormFile("file")
    defer file.Close()

    opts := entity.BulkCreateOptions{
        AutoCloseDuplicates: r.FormValue("auto_close_duplicates") == "true",
        MinimumSeverity:     r.FormValue("minimum_severity"),
        ComputeSLA:          true,
    }

    var result ImportResult
    if format == "csv" {
        result, _ = h.bulkUC.ImportFromCSV(r.Context(), testID, file, opts)
    } else {
        result, _ = h.bulkUC.ImportFromJSON(r.Context(), testID, file, opts)
    }
    writeJSON(w, 200, result)
}
```

**Route registration** (literal trước wildcard {id}):

```go
// SEED-003: Bulk create và import phải TRƯỚC /findings/{id}
r.Post("/api/v2/findings/bulk-create", h.BulkCreateFindings)
r.Post("/api/v2/findings/import",      h.ImportFindings)
```

## Bước 7: Implement PostgreSQL methods

**File:** `internal/infra/postgres/finding_repo.go`

```go
func (r *findingRepo) FindByHashAndProduct(ctx context.Context, hash string, productID uuid.UUID) (*entity.Finding, error) {
    // SELECT * FROM findings WHERE hash_code = $1 AND product_id = $2 AND NOT is_duplicate
}

func (r *findingRepo) CreateWithNotes(ctx context.Context, f *entity.Finding, notes []entity.FindingNoteInput) error {
    // BEGIN TX → INSERT findings → INSERT finding_notes → COMMIT
}
```

## Bước 8: Gateway routes

**File:** `apps/osv/internal/gateway/router.go`

```go
// SEED-003: Finding Bulk endpoints (LITERAL trước /{id})
mux.Handle("POST /api/v2/findings/bulk-create",
    protected(ratelimit.Wrap(proxy.Forward("finding-service:8085"), 5)))
mux.Handle("POST /api/v2/findings/import",
    protected(ratelimit.Wrap(proxy.Forward("finding-service:8085"), 2)))
```

## Acceptance Criteria

- [x] `POST /api/v2/findings/bulk-create` với 10 findings → `207 {"created_count": 10}`
- [x] Duplicate findings (same hash) → `status: "duplicate"` (không abort)
- [x] `POST /api/v2/findings/import` JSON file → `200 {"imported_count": N}`
- [x] `POST /api/v2/findings/import` CSV file → `200` với kết quả tương tự
- [x] File > 10MB → `413`
- [x] `minimum_severity: "High"` → chỉ import Critical + High findings
- [x] Route không conflict với `/findings/{id}/*`
- [x] `go build ./internal/usecase/findingbulk/... ./internal/delivery/http/...` thành công
