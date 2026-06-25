# SOL-006: AI-Service — Triage Queue, Enrichment, Insights

> **Bugs giải quyết**: BUG-011  
> **Service**: `services/ai-service`  
> **Port**: 9103  
> **Architecture ref**: §3.11 AI-Service  
> **Status**: `[x] DONE`

## Kết Quả Thực Thi

**Đã hoàn thành trong ai-service:**

| Fix | File | Trạng thái |
|---|---|---|
| `GET /api/v1/ai/triage/queue` → `GetTriageQueue` handler | `internal/delivery/http/ai_handler.go` | ✅ Đã có |
| `GET /api/v1/ai/enrichment` → `GetEnrichmentStatus` | `internal/delivery/http/ai_handler.go` | ✅ Đã có |
| `POST /api/v1/ai/enrichment/trigger` → `TriggerEnrichment` | `internal/delivery/http/ai_handler.go` | ✅ Đã có |
| `GET /api/v1/ai/insights` → `GetInsights` | `internal/delivery/http/ai_handler.go` | ✅ Đã có |
| Routes mount đúng trong `NewRouter()` | `internal/delivery/http/ai_handler.go` | ✅ Đã có (TASK-006) |
| Fix wrong module path `google/osv.dev` → `osv/ai-service` | `internal/usecase/enrich/usecase.go` | ✅ Fixed |
| Wire redis client (triage queue không crash khi nil) | `cmd/server/main.go` | ✅ Fixed (TASK-006) |

**Build verify**: `go build ./...` ✅ ai-service


---

## Phân Tích

Theo architecture §3.11, ai-service ĐÃ có HTTP endpoints thiết kế sẵn:

```
| GET  | /api/v1/ai/triage/queue           | Get triage queue status  |
| GET  | /api/v1/ai/enrichment             | Enrichment pipeline status |
| POST | /api/v1/ai/enrichment/trigger     | Trigger manual enrichment |
| GET  | /api/v1/ai/insights               | (implied from UI)         |
```

Redis cache cho queue status:
- `ai:enrichment:status` (5m TTL)
- `ai:triage:queue` (1h TTL)

Có 2 khả năng xảy ra 404:
1. **Gateway chưa route** đến ai-service (→ fix trong SOL-001)
2. **ai-service chưa implement HTTP handler** cho những endpoints này

---

## Fix 1: Gateway Routing (SOL-001 đã cover)

```go
// apps/osv/internal/gateway/setup.go
mux.Handle("GET /api/v1/ai/triage/queue",       protected(proxy.Forward("ai-service:9103")))
mux.Handle("GET /api/v1/ai/enrichment",          protected(proxy.Forward("ai-service:9103")))
mux.Handle("POST /api/v1/ai/enrichment/trigger", protected(proxy.Forward("ai-service:9103")))
mux.Handle("GET /api/v1/ai/insights",            protected(proxy.Forward("ai-service:9103")))
```

---

## Fix 2: AI-Service HTTP Handlers

### GET /api/v1/ai/triage/queue

```go
// services/ai-service/internal/delivery/http/triage_handler.go

// GET /api/v1/ai/triage/queue
func (h *TriageHandler) GetQueue(w http.ResponseWriter, r *http.Request) {
    // Try Redis cache first (ai:triage:queue, 1h TTL per architecture §4.3)
    cacheKey := "ai:triage:queue"
    if cached, err := h.redis.Get(r.Context(), cacheKey).Bytes(); err == nil {
        w.Header().Set("X-Cache", "HIT")
        w.Header().Set("Content-Type", "application/json")
        w.Write(cached)
        return
    }
    
    filter := TriageQueueFilter{
        Status: r.URL.Query().Get("status"),  // "pending", "processing", "completed"
        Page:   parseIntParam(r, "page", 1),
        Limit:  parseIntParam(r, "limit", 20),
    }
    
    queue, total, err := h.triageUC.GetQueue(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    result := map[string]interface{}{
        "items": queue,
        "total": total,
        "stats": map[string]int{
            "pending":    countByStatus(queue, "pending"),
            "processing": countByStatus(queue, "processing"),
            "completed":  countByStatus(queue, "completed"),
        },
    }
    
    // Cache for 1 hour
    h.redis.Set(r.Context(), cacheKey, result, 1*time.Hour)
    
    respondJSON(w, http.StatusOK, result)
}
```

### GET /api/v1/ai/enrichment

```go
// GET /api/v1/ai/enrichment?status=completed&page=1&limit=20
func (h *EnrichmentHandler) GetPipelineStatus(w http.ResponseWriter, r *http.Request) {
    // Try Redis cache (ai:enrichment:status, 5m TTL per architecture §4.3)
    cacheKey := "ai:enrichment:status"
    if cached, err := h.redis.Get(r.Context(), cacheKey).Bytes(); err == nil {
        w.Header().Set("X-Cache", "HIT")
        w.Header().Set("Content-Type", "application/json")
        w.Write(cached)
        return
    }
    
    filter := EnrichmentFilter{
        Status: r.URL.Query().Get("status"),
        Page:   parseIntParam(r, "page", 1),
        Limit:  parseIntParam(r, "limit", 20),
    }
    
    // Query from osv_ai schema (cve_embeddings, severity_cache tables)
    items, total, err := h.enrichUC.List(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    // Pipeline stats
    stats, _ := h.enrichUC.GetStats(r.Context())
    
    result := map[string]interface{}{
        "items": items,
        "total": total,
        "pipeline": map[string]interface{}{
            "pending":    stats.Pending,
            "processing": stats.Processing,
            "completed":  stats.Completed,
            "failed":     stats.Failed,
            "last_run":   stats.LastRun,
        },
    }
    
    // Cache 5 minutes
    h.redis.Set(r.Context(), cacheKey, result, 5*time.Minute)
    
    respondJSON(w, http.StatusOK, result)
}
```

### POST /api/v1/ai/enrichment/trigger

```go
// POST /api/v1/ai/enrichment/trigger
func (h *EnrichmentHandler) Trigger(w http.ResponseWriter, r *http.Request) {
    var req struct {
        CveIDs []string `json:"cve_ids"` // Optional: specific CVEs to enrich
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    userID := r.Header.Get("X-User-ID")
    
    jobID, err := h.enrichUC.TriggerBatch(r.Context(), BatchEnrichRequest{
        CveIDs:  req.CveIDs,
        ActorID: userID,
    })
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    // Invalidate cache
    h.redis.Del(r.Context(), "ai:enrichment:status")
    
    // 202 Accepted pattern (async job)
    respondJSON(w, http.StatusAccepted, map[string]string{
        "job_id":  jobID,
        "status":  "queued",
        "message": "Enrichment batch job queued successfully",
    })
}
```

### GET /api/v1/ai/insights (Feature mới)

`ai/insights` chưa có trong architecture. Cần implement use case mới:

```go
// services/ai-service/internal/usecase/insights_usecase.go

type InsightsUseCase struct {
    findingClient FindingServiceClient  // gRPC hoặc HTTP client
    cveRepo       CVERepository
    llmChain      *provider.Chain       // Ollama → OpenAI failover
    cache         *redis.Client
}

func (uc *InsightsUseCase) Generate(ctx context.Context) ([]Insight, error) {
    // Check cache
    cacheKey := "ai:insights:latest"
    if cached, err := uc.cache.Get(ctx, cacheKey).Bytes(); err == nil {
        var insights []Insight
        json.Unmarshal(cached, &insights)
        return insights, nil
    }
    
    // Aggregate data
    // 1. Get top critical findings from finding-service
    // 2. Get recent KEV entries from cve data
    // 3. Get SLA breach metrics
    
    // Generate insights via LLM (with context)
    prompt := buildInsightPrompt(criticalFindings, kevEntries, slaBreaches)
    response, err := uc.llmChain.Complete(ctx, prompt)
    if err != nil {
        // Fallback: return rule-based insights
        return uc.generateRuleBasedInsights(ctx), nil
    }
    
    insights := parseInsights(response)
    uc.cache.Set(ctx, cacheKey, insights, 1*time.Hour)
    
    return insights, nil
}
```

```go
// HTTP Handler
func (h *InsightsHandler) GetInsights(w http.ResponseWriter, r *http.Request) {
    insights, err := h.insightsUC.Generate(r.Context())
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "items":        insights,
        "generated_at": time.Now().UTC(),
    })
}
```

### Router Registration (ai-service)

```go
// services/ai-service/internal/delivery/http/router.go

// THÊM MỚI:
r.GET("/api/v1/ai/triage/queue",       authMiddleware(triageHandler.GetQueue))
r.GET("/api/v1/ai/enrichment",          authMiddleware(enrichHandler.GetPipelineStatus))
r.POST("/api/v1/ai/enrichment/trigger", authMiddleware(enrichHandler.Trigger))
r.GET("/api/v1/ai/insights",            authMiddleware(insightsHandler.GetInsights))

// Đã có:
r.POST("/api/v1/ai/triage/{findingId}",        authMiddleware(triageHandler.TriageFinding))
r.POST("/api/v1/ai/triage/{findingId}/review", authMiddleware(triageHandler.ReviewTriage))
r.GET("/api/v1/ai/enrichment/{cveId}",         authMiddleware(enrichHandler.GetCVEEnrichment))
```

## Response Schemas

```go
type TriageQueueItem struct {
    FindingID   string    `json:"finding_id"`
    CveID       string    `json:"cve_id,omitempty"`
    ProductName string    `json:"product_name,omitempty"`
    Severity    string    `json:"severity"`
    Status      string    `json:"status"`  // "pending", "processing", "completed"
    QueuedAt    time.Time `json:"queued_at"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
    TriageResult *string  `json:"triage_result,omitempty"`  // LLM output
}

type Insight struct {
    ID          string    `json:"id"`
    Type        string    `json:"type"`     // "trend", "anomaly", "recommendation"
    Title       string    `json:"title"`
    Content     string    `json:"content"`
    Severity    string    `json:"severity"` // "critical", "high", "medium", "info"
    Source      string    `json:"source"`   // "llm", "rule-based"
    GeneratedAt time.Time `json:"generated_at"`
}
```

## Database Schema (osv_ai)

```sql
-- Verify/add tables if missing
CREATE TABLE IF NOT EXISTS triage_queue (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id   UUID NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending',
    queued_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    result_text  TEXT,
    actor_id     UUID
);

CREATE INDEX IF NOT EXISTS idx_triage_queue_status ON triage_queue(status, queued_at DESC);
```
