# TASK-HC-012: AI Batch Enrichment UseCase thật

**Status:** ✅ DONE  
**Sprint:** 2 | **Ước lượng:** 4 giờ  
**Phụ thuộc:** TASK-HC-006 (generate_embedding usecase cần hoàn thành trước)  
**Solution:** [SOL-013](../solutions/SOL-013-ai-batch-enrich.md)  
**Service:** `services/ai-service`
**Completed:** 2026-06-24

---

## Implementation Summary

| File | Action | Status |
|------|--------|--------|
| `internal/usecase/batch_enrich/usecase.go` | MODIFY — `CVEEnricher` dependency, semaphore concurrency, real `ExecuteAsync` | ✅ |
| `internal/delivery/http/ai_handler.go` | MODIFY — `BatchEnrich` endpoint trả `202 Accepted` với `job_id` | ✅ |
| `embed/server.go` | MODIFY — wire `batchEnrichUC` với `enrich.UseCase` | ✅ |

**Build:** `go build ./...` ✅ PASS  
**Acceptance Criteria Met:**
- ✅ `batch_enrich.UseCase` có `CVEEnricher` dependency (inject qua constructor)
- ✅ `ExecuteAsync` thực sự gọi enrichment cho từng CVE
- ✅ Concurrency kiểm soát qua semaphore (`maxConcurrency` config)
- ✅ `POST /api/v1/ai/enrichment/batch` trả `202 Accepted` với `job_id`
- ✅ `go build ./...` pass trong `services/ai-service`

---

## Mô tả

`batch_enrich/usecase.go` không có dependencies và không làm gì thật. Cần inject `CVEEnricher` interface, implement semaphore concurrency, và expose qua HTTP endpoint.

---

## Acceptance Criteria

- [x] `batch_enrich.UseCase` có `CVEEnricher` dependency (không empty struct)
- [x] `ExecuteAsync` thực sự gọi enrichment cho từng CVE
- [x] Concurrency bị kiểm soát qua semaphore (`maxConcurrency` config)
- [x] `POST /api/v1/ai/enrichment/batch` trả `202 Accepted` với job_id
- [x] Kết quả enrichment được log với success/failed count
- [x] `go build ./...` pass trong `services/ai-service`

---

## Files cần sửa/tạo

| Action | File | Thay đổi |
|--------|------|---------|
| NEW | `services/ai-service/internal/usecase/batch_enrich/ports.go` | CVEEnricher interface |
| MODIFY | `services/ai-service/internal/usecase/batch_enrich/usecase.go` | Full implementation |
| MODIFY | `services/ai-service/internal/delivery/http/ai_handler.go` | BatchEnrich endpoint |
| MODIFY | `services/ai-service/embed/server.go` | Wire batch_enrich với enrich UC |

---

## Bước thực thi

### 1. Khảo sát enrich usecase

```bash
cat services/ai-service/internal/usecase/enrich/usecase.go | head -40
grep -n "func.*Execute\|Execute(" services/ai-service/internal/usecase/enrich/usecase.go
```

Ghi lại method signature của single enrich — sẽ dùng cho CVEEnricher interface.

### 2. Khảo sát batch_enrich hiện tại

```bash
cat services/ai-service/internal/usecase/batch_enrich/usecase.go
```

### 3. Tạo ports.go

**File:** `services/ai-service/internal/usecase/batch_enrich/ports.go`

```go
package batch_enrich

import "context"

// CVEEnricher enriches a single CVE with AI analysis.
// Implemented by: usecase/enrich.UseCase
type CVEEnricher interface {
    Execute(ctx context.Context, cveID string) error
}
```

### 4. Refactor usecase.go với real implementation

**File:** `services/ai-service/internal/usecase/batch_enrich/usecase.go`

```go
package batch_enrich

import (
    "context"
    "sync"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"
)

type UseCase struct {
    enricher       CVEEnricher
    maxConcurrency int
    log            zerolog.Logger
}

// New creates UseCase. enricher must not be nil.
func New(enricher CVEEnricher, maxConcurrency int, log zerolog.Logger) *UseCase {
    if maxConcurrency <= 0 { maxConcurrency = 5 }
    return &UseCase{enricher: enricher, maxConcurrency: maxConcurrency, log: log}
}

type Result struct {
    CVEID      string
    Success    bool
    Err        error
    DurationMs int64
}

// ExecuteAsync enriches multiple CVEs concurrently.
// [FIX CR-HC-013] Full semaphore-based implementation.
func (uc *UseCase) ExecuteAsync(ctx context.Context, cveIDs []string) []Result {
    if len(cveIDs) == 0 {
        return nil
    }

    jobID := uuid.New().String()
    uc.log.Info().
        Str("job_id", jobID).
        Int("count", len(cveIDs)).
        Int("concurrency", uc.maxConcurrency).
        Msg("batch_enrich started")

    sem := make(chan struct{}, uc.maxConcurrency)
    results := make([]Result, len(cveIDs))
    var wg sync.WaitGroup

    for i, cveID := range cveIDs {
        wg.Add(1)
        go func(idx int, id string) {
            defer wg.Done()
            select {
            case sem <- struct{}{}:
                defer func() { <-sem }()
            case <-ctx.Done():
                results[idx] = Result{CVEID: id, Err: ctx.Err()}
                return
            }

            start := time.Now()
            err := uc.enricher.Execute(ctx, id)
            results[idx] = Result{
                CVEID:      id,
                Success:    err == nil,
                Err:        err,
                DurationMs: time.Since(start).Milliseconds(),
            }
            if err != nil {
                uc.log.Warn().Err(err).Str("cve_id", id).Msg("enrich failed")
            }
        }(i, cveID)
    }
    wg.Wait()

    success, failed := 0, 0
    for _, r := range results {
        if r.Success { success++ } else { failed++ }
    }
    uc.log.Info().
        Str("job_id", jobID).
        Int("success", success).
        Int("failed", failed).
        Msg("batch_enrich completed")

    return results
}
```

### 5. Thêm BatchEnrich HTTP handler

```bash
grep -n "func.*BatchEnrich\|batch.*enrich\|batchEnrich" \
  services/ai-service/internal/delivery/http/ai_handler.go | head -5
```

Thêm (hoặc sửa existing stub):
```go
// [FIX CR-HC-013] BatchEnrich — async với job_id response
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
        writeError(w, http.StatusBadRequest, "maximum 100 CVEs per batch")
        return
    }
    if h.batchEnrichUC == nil {
        writeError(w, http.StatusServiceUnavailable, "batch enrichment not configured")
        return
    }

    jobID := uuid.New().String()
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
        defer cancel()
        h.batchEnrichUC.ExecuteAsync(ctx, req.CVEIDs)
    }()

    writeJSON(w, http.StatusAccepted, map[string]interface{}{
        "job_id":  jobID,
        "count":   len(req.CVEIDs),
        "status":  "queued",
        "message": "Batch enrichment queued. Processing in background.",
    })
}
```

### 6. Wire trong embed/server.go

```bash
grep -n "batch_enrich\|batchEnrich\|NewBatch\|enrich.New\|enrichUC" \
  services/ai-service/embed/server.go | head -10
```

```go
import (
    batchenrich "github.com/osv/ai-service/internal/usecase/batch_enrich"
)

// Wire: singleEnricher đến từ enrich.UseCase (đã implement trong TASK-HC-006)
batchEnrichUC := batchenrich.New(singleEnricher, cfg.MaxBatchConcurrency, logger)
aiHTTPHandler.SetBatchEnrichUseCase(batchEnrichUC)
```

> `cfg.MaxBatchConcurrency` từ env `BATCH_ENRICH_CONCURRENCY`, default 5.

### 7. Build check
```bash
cd services/ai-service && go build ./...
```

---

## Verification

```bash
# Batch enrich
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"cve_ids":["CVE-2021-44228","CVE-2023-44487"]}' \
  "https://c12.openledger.vn/api/v1/ai/enrichment/batch" | jq '{status, job_id, count}'
# PASS nếu status = "queued"

# Sau ~30s, verify embeddings trong DB
sleep 30
psql $DATABASE_URL -c "
SELECT cve_id, model FROM cve_embeddings 
WHERE cve_id IN ('CVE-2021-44228','CVE-2023-44487')
ORDER BY created_at DESC;
"
# PASS nếu cả 2 rows tồn tại
```
