# SOL-SEED-003: Giải pháp thực thi — Findings Seed

> **CR:** [SEED-003-findings-seed.md](../SEED-003-findings-seed.md)  
> **Cập nhật:** 2026-06-18  
> **Domain:** `services/finding-service` + `apps/osv` (gateway)  
> **Priority:** 🔴 CRITICAL

---

## 1. Phân tích kiến trúc hiện tại

Theo `01-architecture.md §3.5`, `finding-service` có:
- **Deduplication**: SHA-256 hash `(title + component_name + component_version + cve_id)` — cần áp dụng trong bulk create.
- **State machine**: Finding bắt đầu ở `Active`. SLA tính từ `date + sla_days_for(severity)`.
- **NATS**: Publish `finding.created`, `finding.batch_created`, `finding.status.changed`.

Scan import flow hiện tại (§5.3): `POST /api/v2/import-scan` → SCA parser → dedup → SLA → `finding.batch_created`.

Bulk create mới cần tái dùng logic **dedup + SLA** nhưng không qua parser (input là raw JSON/CSV).

---

## 2. Các thay đổi cần thực hiện

### 2.1 Domain Layer — `services/finding-service/internal/domain/`

**File**: `internal/domain/entity/finding.go` — Thêm input types:

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
    Date             time.Time
    Tags             []string
    Notes            []FindingNoteInput
    // Lookup keys
    TestID    uuid.UUID
    ProductID uuid.UUID  // derived from Test → Engagement → Product chain
}

// FindingNoteInput là ghi chú kèm theo khi tạo finding
type FindingNoteInput struct {
    Content   string
    IsPrivate bool
}

// BulkCreateOptions là tùy chọn cho bulk create
type BulkCreateOptions struct {
    AutoCloseDuplicates bool
    AutoEnrichCVE       bool
    ComputeSLA          bool  // default: true
    MinimumSeverity     string
}

// BulkFindingResult là kết quả per-item
type BulkFindingResult struct {
    Index     int
    Status    string     // "created" | "duplicate" | "error"
    ID        *uuid.UUID
    HashCode  string
    Message   string
}
```

**File**: `internal/domain/repository/finding_repository.go` — Thêm methods:

```go
type FindingRepository interface {
    // ... existing ...
    
    // FindByHashAndProduct kiểm tra deduplication
    FindByHashAndProduct(ctx context.Context, hash string, productID uuid.UUID) (*entity.Finding, error)
    
    // CreateWithNotes tạo finding + notes trong 1 transaction
    CreateWithNotes(ctx context.Context, f *entity.Finding, notes []entity.FindingNoteInput) error
    
    // CreateBulk tạo nhiều findings trong 1 transaction
    CreateBulk(ctx context.Context, findings []*entity.Finding) ([]entity.BulkFindingResult, error)
}
```

---

### 2.2 Use Case Layer — `services/finding-service/internal/usecase/`

#### Tạo usecase `finding_bulk`

**File**: `internal/usecase/finding_bulk/usecase.go`

```go
package findingbulk

type UseCase struct {
    findingRepo  repository.FindingRepository
    testRepo     repository.TestRepository
    slaRepo      repository.SLAConfigRepository  // đọc SLA config của product
    aiClient     ai.Client                        // optional async enrich
    eventPub     events.Publisher
    hasher       hash.Hasher                      // SHA-256 dedup
}

// BulkCreate tạo nhiều findings với dedup + SLA auto-compute
func (uc *UseCase) BulkCreate(ctx context.Context, testID uuid.UUID, inputs []entity.FindingCreateInput, opts entity.BulkCreateOptions) (BulkResult, error)

// ImportFromJSON parse JSON array → BulkCreate
func (uc *UseCase) ImportFromJSON(ctx context.Context, testID uuid.UUID, r io.Reader, opts entity.BulkCreateOptions) (ImportResult, error)

// ImportFromCSV parse CSV → BulkCreate
func (uc *UseCase) ImportFromCSV(ctx context.Context, testID uuid.UUID, r io.Reader, opts entity.BulkCreateOptions) (ImportResult, error)
```

**Logic `BulkCreate`:**

```
1. Load Test → Engagement → Product (để lấy product_id + SLA config)
2. Load SLAConfiguration cho product (hoặc global default)
3. Begin transaction
4. For each FindingCreateInput (index i):
   a. Filter by MinimumSeverity nếu có
   b. ComputeHash(title + component + version + cve_id)
   c. Check dedup: FindByHashAndProduct(hash, product_id)
      - Nếu duplicate tồn tại:
        * opts.AutoCloseDuplicates = true → append result {status:"duplicate"}
        * Continue (không INSERT)
   d. Compute sla_expiration_date = input.Date + sla_config.days_for(severity)
   e. INSERT finding (active=true, sla_expiration_date)
   f. INSERT finding_notes nếu có
   g. Append result {status:"created", id, hash_code}
5. Commit
6. Publish NATS: finding.batch_created{test_id, finding_ids[], count}
7. Nếu AutoEnrichCVE: for each CVE ID → fire goroutine → POST to ai-service
8. Return BulkResult{created, duplicate, failed, results}
```

**Logic `ImportFromCSV`:**

```go
// CSV header mapping
var csvFieldMap = map[string]string{
    "title":             "Title",
    "severity":          "Severity",
    "cve":               "CveID",
    "cwe":               "Cwe",
    "description":       "Description",
    "mitigation":        "Mitigation",
    "component_name":    "ComponentName",
    "component_version": "ComponentVersion",
}

// Đọc header row → map columns → parse rows → validate → BulkCreate
```

---

### 2.3 Adapter Layer — HTTP Handlers

**File**: `internal/delivery/http/finding_handler.go` — Thêm:

```go
// BulkCreateFindings handles POST /api/v2/findings/bulk-create
func (h *Handler) BulkCreateFindings(w http.ResponseWriter, r *http.Request) {
    var req struct {
        TestID              uuid.UUID                    `json:"test_id"`
        Findings            []entity.FindingCreateInput `json:"findings"`
        AutoCloseDuplicates bool                         `json:"auto_close_duplicates"`
        AutoEnrichCVE       bool                         `json:"auto_enrich_cve"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, 400, errResp("invalid_body", err.Error()))
        return
    }
    opts := entity.BulkCreateOptions{
        AutoCloseDuplicates: req.AutoCloseDuplicates,
        AutoEnrichCVE:       req.AutoEnrichCVE,
        ComputeSLA:          true,
    }
    result, err := h.bulkUC.BulkCreate(r.Context(), req.TestID, req.Findings, opts)
    if err != nil {
        writeJSON(w, 500, errResp("internal", err.Error()))
        return
    }
    writeJSON(w, 207, result)
}

// ImportFindings handles POST /api/v2/findings/import
// Multipart: file + test_id + format + options
func (h *Handler) ImportFindings(w http.ResponseWriter, r *http.Request) {
    // Parse multipart (max 10MB)
    r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
    if err := r.ParseMultipartForm(10 << 20); err != nil {
        writeJSON(w, 413, errResp("payload_too_large", "file exceeds 10MB limit"))
        return
    }
    
    testIDStr := r.FormValue("test_id")
    format    := r.FormValue("format")  // "json" | "csv"
    testID, _ := uuid.Parse(testIDStr)
    
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

**File**: `internal/delivery/http/router.go` — Thêm routes:

```go
// SEED-003: Finding Bulk + Import endpoints
// IMPORTANT: literal paths TRƯỚC {id} wildcard
r.Post("/api/v2/findings/bulk-create", h.BulkCreateFindings)
r.Post("/api/v2/findings/import",      h.ImportFindings)
```

---

### 2.4 Chuẩn hóa `POST /api/v2/findings/bulk`

Endpoint này đã tồn tại nhưng schema không rõ. Cập nhật handler để hỗ trợ:

```go
// BulkUpdate handles POST /api/v2/findings/bulk
// Action: update_status | set_severity | set_tags | set_assignee | mark_false_positive | close | reopen | delete
func (h *Handler) BulkUpdate(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Action     string      `json:"action"`
        FindingIDs []uuid.UUID `json:"finding_ids"`
        Payload    json.RawMessage `json:"payload"`
    }
    // Route to appropriate usecase by action
}
```

---

### 2.5 SLA Auto-compute khi CREATE

Đảm bảo `POST /api/v2/findings` (single create) cũng tính SLA:

**File**: `internal/usecase/finding/create_finding.go`

```go
func (uc *CreateFindingUseCase) Execute(ctx context.Context, in CreateFindingInput) (*entity.Finding, error) {
    // ... existing logic ...
    
    // SEED-003: Auto-compute SLA expiration
    slaConfig, err := uc.slaRepo.FindForProduct(ctx, productID)
    if err == nil && slaConfig != nil {
        days := slaConfig.DaysForSeverity(finding.Severity)
        finding.SLAExpirationDate = finding.Date.AddDate(0, 0, days)
    }
    
    // ... save + publish ...
}
```

---

### 2.6 Gateway Layer — `apps/osv/internal/gateway/router.go`

```go
// ═══════════════════════════════════════════════
// FINDING SEED ENDPOINTS (SEED-003)
// ═══════════════════════════════════════════════
// bulk-create và import PHẢI đứng TRƯỚC /findings/{id}
mux.Handle("POST /api/v2/findings/bulk-create",
    protected(ratelimit.Wrap(proxy.Forward("finding-service:8085"), 5)))

// import: rate limit thấp hơn (2/min) — file upload nặng
mux.Handle("POST /api/v2/findings/import",
    protected(ratelimit.Wrap(proxy.Forward("finding-service:8085"), 2)))
```

> **Rate limits** theo `01-architecture.md §3.1`: `POST /api/v2/findings/bulk` → 10/min. Áp dụng tương tự cho `bulk-create` và `import`.

---

## 3. NATS Events

| Subject | Publisher | Consumers | Payload |
|---------|-----------|----------|---------|
| `finding.batch_created` | finding-service | notification-service, sla-service, audit-service | `{test_id, finding_ids[], created_count, import_source}` |

> `import_source: "bulk_create" | "json_import" | "csv_import"` — để audit-service ghi nhận nguồn.

---

## 4. File thay đổi tổng hợp

| File | Thay đổi |
|------|---------|
| `internal/domain/entity/finding.go` | Thêm `FindingCreateInput`, `BulkCreateOptions`, `BulkFindingResult` |
| `internal/domain/repository/finding_repository.go` | Thêm `FindByHashAndProduct`, `CreateWithNotes`, `CreateBulk` |
| `internal/usecase/finding_bulk/usecase.go` | **[NEW]** |
| `internal/usecase/finding_bulk/csv_parser.go` | **[NEW]** |
| `internal/usecase/finding/create_finding.go` | Thêm SLA auto-compute |
| `internal/delivery/http/finding_handler.go` | Thêm `BulkCreateFindings`, `ImportFindings`, chuẩn hóa `BulkUpdate` |
| `internal/delivery/http/router.go` | Thêm 2 routes (literal trước wildcard) |
| `internal/infra/postgres/finding_repo.go` | Implement `FindByHashAndProduct`, `CreateWithNotes`, `CreateBulk` |
| `apps/osv/internal/gateway/router.go` | Thêm 2 gateway routes |

---

## 5. Acceptance Criteria

1. `POST /api/v2/findings/bulk-create` với 10 findings → `207 {"created_count": 10}`, mỗi finding có `sla_expiration_date`.
2. `POST /api/v2/findings/bulk-create` với `auto_close_duplicates: true`, 2 findings trùng hash → `207 {"created_count": 8, "duplicate_count": 2}`.
3. `POST /api/v2/findings/import` (JSON, 25 findings) → `200 {"imported_count": 25}`.
4. `POST /api/v2/findings/import` (CSV) → `200` với kết quả tương tự JSON.
5. `POST /api/v2/findings/import` file > 10MB → `413 Payload Too Large`.
6. `POST /api/v2/findings/import` với `minimum_severity: "High"` → chỉ import Critical + High.
7. `POST /api/v2/findings` (single) → response có `sla_expiration_date` được tính tự động.
8. `POST /api/v2/findings/bulk` với action `set_tags` + `mode: "add"` → tags được merge, không replace.
