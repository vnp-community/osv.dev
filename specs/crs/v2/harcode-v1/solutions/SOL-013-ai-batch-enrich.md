# SOL-013: AI Batch Enrichment UseCase — ai-service

**CR:** CR-HC-013 | **Priority:** 🟡 Medium | **Sprint:** 2  
**Service:** `services/ai-service`

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-012
**Note:** batch_enrich gọi real enrich.UseCase thay TODO stub
**Build:** ✅ `go build ./...` passes

---

---

## Context phân tích code

**File:** `ai-service/internal/usecase/batch_enrich/usecase.go`

```go
type UseCase struct {
    maxConcurrency int
    // Không có dependencies → không thể làm gì thật
}

func (uc *UseCase) ExecuteAsync(ctx context.Context, cveIDs []string) []Result {
    for i, cveID := range cveIDs {
        go func() {
            // TODO: call enrich_cve usecase per CVE  ← shell rỗng
            results[idx] = Result{CVEID: id}  // không có Err, không có output
        }()
    }
}
```

**Infrastructure đã có:**
- `ai-service/internal/usecase/enrich/usecase.go` — single CVE enrichment (cần kiểm tra implement state)
- `ai-service/internal/usecase/enrich_cve/handler.go` — NATS handler
- `ai-service/internal/provider/chain.go` — provider chain

---

## Solution

### Bước 1: Kiểm tra single enrich usecase

```bash
cat ai-service/internal/usecase/enrich/usecase.go
```

Nếu chưa implement → implement trước (cần cho batch).

### Bước 2: Định nghĩa ports trong domain

**File mới:** `ai-service/internal/usecase/batch_enrich/ports.go`

```go
package batch_enrich

import "context"

// CVEEnricher enriches a single CVE.
// Implemented by: usecase/enrich.UseCase (single enrich)
type CVEEnricher interface {
    Execute(ctx context.Context, cveID string) error
}

// BatchStatusRepo persists batch job status.
// Implemented by: infra/postgres/batch_status_repo
type BatchStatusRepo interface {
    // Record completion/failure for audit
    RecordResult(ctx context.Context, jobID, cveID string, success bool, errMsg string) error
}
```

### Bước 3: Refactor batch_enrich UseCase với real dependencies

**File sửa:** `ai-service/internal/usecase/batch_enrich/usecase.go`

```go
package batch_enrich

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"
)

// UseCase handles bulk CVE enrichment with concurrency control and status tracking.
type UseCase struct {
    enricher       CVEEnricher  // single CVE enricher
    maxConcurrency int
    log            zerolog.Logger
}

// New creates a UseCase. enricher must not be nil.
func New(enricher CVEEnricher, maxConcurrency int, log zerolog.Logger) *UseCase {
    if maxConcurrency <= 0 {
        maxConcurrency = 5
    }
    return &UseCase{
        enricher:       enricher,
        maxConcurrency: maxConcurrency,
        log:            log,
    }
}

// Result holds enrichment outcome for one CVE.
type Result struct {
    CVEID     string
    Success   bool
    Err       error
    DurationMs int64
}

// ExecuteAsync enriches CVEs in parallel with rate-limiting.
// Returns when all CVEs are processed or ctx is cancelled.
// [FIX CR-HC-013] Full implementation — no more TODO
func (uc *UseCase) ExecuteAsync(ctx context.Context, cveIDs []string) []Result {
    if len(cveIDs) == 0 {
        return nil
    }

    jobID := uuid.New().String()
    uc.log.Info().
        Str("job_id", jobID).
        Int("count", len(cveIDs)).
        Int("concurrency", uc.maxConcurrency).
        Msg("batch_enrich: started")

    sem := make(chan struct{}, uc.maxConcurrency)
    results := make([]Result, len(cveIDs))
    var wg sync.WaitGroup

    for i, cveID := range cveIDs {
        wg.Add(1)
        go func(idx int, id string) {
            defer wg.Done()

            // Acquire semaphore slot
            select {
            case sem <- struct{}{}:
                defer func() { <-sem }()
            case <-ctx.Done():
                results[idx] = Result{CVEID: id, Err: ctx.Err()}
                return
            }

            start := time.Now()
            err := uc.enricher.Execute(ctx, id)
            dur := time.Since(start).Milliseconds()

            results[idx] = Result{
                CVEID:      id,
                Success:    err == nil,
                Err:        err,
                DurationMs: dur,
            }

            if err != nil {
                uc.log.Warn().Err(err).Str("cve_id", id).Msg("batch_enrich: single enrich failed")
            } else {
                uc.log.Debug().Str("cve_id", id).Int64("ms", dur).Msg("batch_enrich: enriched")
            }
        }(i, cveID)
    }
    wg.Wait()

    // Compute summary
    success, failed := 0, 0
    for _, r := range results {
        if r.Success {
            success++
        } else {
            failed++
        }
    }
    uc.log.Info().
        Str("job_id", jobID).
        Int("success", success).
        Int("failed", failed).
        Msg("batch_enrich: completed")

    return results
}

// ExecuteSingle enriches a single CVE — convenience wrapper.
func (uc *UseCase) ExecuteSingle(ctx context.Context, cveID string) error {
    return uc.enricher.Execute(ctx, cveID)
}
```

### Bước 4: Batch enrich HTTP handler

**File sửa:** `ai-service/internal/delivery/http/ai_handler.go`

```go
// [FIX CR-HC-013] Batch enrich endpoint
func (h *AIHandler) BatchEnrich(w http.ResponseWriter, r *http.Request) {
    var req struct {
        CVEIDs []string `json:"cve_ids"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if len(req.CVEIDs) == 0 {
        writeError(w, http.StatusBadRequest, "cve_ids required")
        return
    }
    if len(req.CVEIDs) > 100 {
        writeError(w, http.StatusBadRequest, "maximum 100 CVEs per batch request")
        return
    }

    // Run async — respond immediately with job started
    jobID := uuid.New().String()
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
        defer cancel()
        results := h.batchEnrichUC.ExecuteAsync(ctx, req.CVEIDs)
        
        success := 0
        for _, r := range results {
            if r.Success { success++ }
        }
        h.log.Info().
            Str("job_id", jobID).
            Int("success", success).
            Int("total", len(results)).
            Msg("batch enrich job completed")
    }()

    writeJSON(w, http.StatusAccepted, map[string]interface{}{
        "job_id":  jobID,
        "count":   len(req.CVEIDs),
        "status":  "queued",
        "message": "Batch enrichment queued. Processing in background.",
    })
}
```

### Bước 5: Wire trong embed/server.go

```go
// [FIX CR-HC-013] Wire batch_enrich dengan real dependencies
singleEnricher := enrich.New(providerChain, pgVectorStore, mongoRepo, log)
batchEnrichUC := batch_enrich.New(singleEnricher, maxConcurrency, log)
aiHTTPHandler.SetBatchEnrichUseCase(batchEnrichUC)
```

---

## Files cần tạo/sửa

| Action | File |
|--------|------|
| NEW | `ai-service/internal/usecase/batch_enrich/ports.go` |
| MODIFY | `ai-service/internal/usecase/batch_enrich/usecase.go` — full implementation |
| MODIFY | `ai-service/internal/delivery/http/ai_handler.go` — BatchEnrich handler |
| MODIFY | `ai-service/embed/server.go` — wire batchEnrichUC |

---

## Verification

```bash
cd services/ai-service && go build ./...

curl -X POST -H "Authorization: Bearer $TOKEN" \
  -d '{"cve_ids":["CVE-2021-44228","CVE-2021-45046"]}' \
  "https://c12.openledger.vn/api/v1/ai/enrichment/batch"
# Expect: {"job_id":"...","count":2,"status":"queued"}

# Verify embeddings được tạo trong DB
psql $DATABASE_URL -c "
SELECT cve_id, model, created_at 
FROM cve_embeddings 
WHERE cve_id IN ('CVE-2021-44228','CVE-2021-45046');
"
```
