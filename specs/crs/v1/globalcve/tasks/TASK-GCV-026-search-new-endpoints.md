# TASK-GCV-026 — POST /search, POST /search/semantic, GET /aggregations

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-026 |
| **Service** | `search-service` |
| **CR** | CR-GCV-004 |
| **Phase** | 3 — Advanced Search |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-GCV-024 |

## Context

Thêm 3 endpoints mới: `POST /api/v2/cves/search` (OpenSearch FTS, body-based), `POST /api/v2/cves/search/semantic` (pgvector), `GET /api/v2/cves/aggregations` (faceted stats). Dùng backends từ TASK-GCV-024/025.

## Reference

- Solution: [SOL-GCV-004](../solutions/SOL-GCV-004-opensearch-semantic-search.md) §2.4

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/delivery/http/search_handler.go
        (add 3 new handlers + register routes)
```

## Implementation Spec

### FullTextSearch handler (POST /api/v2/cves/search)

```go
type FullTextSearchRequest struct {
    Query   string `json:"query"`
    Filters struct {
        Severity []string `json:"severity"`
        IsKEV    *bool    `json:"is_kev"`
        MinEPSS  *float64 `json:"min_epss"`
        Vendor   string   `json:"vendor"`
    } `json:"filters"`
    Page  int `json:"page"`
    Limit int `json:"limit"`
}

func (h *Handler) FullTextSearch(w http.ResponseWriter, r *http.Request) {
    var req FullTextSearchRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, 400, "invalid request body")
        return
    }
    limit := req.Limit
    if limit <= 0 { limit = 50 }
    if limit > 500 { limit = 500 }

    // Route to OpenSearch (via dual-backend usecase)
    searchReq := &cvesearch.Request{
        Query:   req.Query,
        MinEPSS: req.Filters.MinEPSS,
        Vendor:  req.Filters.Vendor,
        Page:    req.Page,
        Limit:   limit,
    }
    resp, err := h.searchUC.Execute(r.Context(), searchReq)
    if err != nil {
        respondError(w, 500, "search failed")
        return
    }
    respondJSON(w, 200, resp)
}
```

### SemanticSearch handler (POST /api/v2/cves/search/semantic)

```go
func (h *Handler) SemanticSearch(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Query string `json:"query"`
        Limit int    `json:"limit"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Query == "" {
        respondError(w, 400, "query is required")
        return
    }

    limit := req.Limit
    if limit <= 0 { limit = 20 }

    cves, err := h.semanticUC.Execute(r.Context(), req.Query, limit)
    if err != nil {
        respondError(w, 500, "semantic search failed")
        return
    }
    respondJSON(w, 200, map[string]interface{}{"data": cves, "count": len(cves)})
}
```

### GetAggregations handler (GET /api/v2/cves/aggregations)

```go
func (h *Handler) GetAggregations(w http.ResponseWriter, r *http.Request) {
    // Try OpenSearch aggregations first
    if h.osClient != nil {
        aggs, err := h.osClient.GetAggregations(r.Context())
        if err == nil {
            respondJSON(w, 200, aggs)
            return
        }
        log.Warn().Err(err).Msg("OpenSearch aggregations failed, falling back to PostgreSQL")
    }

    // Fallback: PostgreSQL aggregation
    aggs, err := h.cveRepo.GetAggregations(r.Context())
    if err != nil {
        respondError(w, 500, "aggregation failed")
        return
    }
    respondJSON(w, 200, aggs)
}
```

### Route registration (ORDER MATTERS)

```go
// SPECIFIC routes before generic:
r.Post("/api/v2/cves/search/semantic", h.SemanticSearch)  // must be first
r.Post("/api/v2/cves/search",          h.FullTextSearch)
r.Get("/api/v2/cves/aggregations",     h.GetAggregations)
// existing GET /api/v2/cves stays unchanged
```

### CVERepository — ADD GetAggregations method

```go
// repository interface:
GetAggregations(ctx context.Context) (map[string]interface{}, error)

// PostgreSQL implementation:
func (r *pgCVERepository) GetAggregations(ctx context.Context) (map[string]interface{}, error) {
    // severity breakdown
    type severityRow struct {
        Severity string `db:"severity"`
        Count    int    `db:"count"`
    }
    var bySeverity []severityRow
    r.db.SelectContext(ctx, &bySeverity, `
        SELECT COALESCE(severity,'UNKNOWN') as severity, COUNT(*) as count
        FROM cves GROUP BY severity ORDER BY count DESC
    `)

    severityMap := map[string]int{}
    for _, s := range bySeverity { severityMap[s.Severity] = s.Count }

    return map[string]interface{}{
        "by_severity":  severityMap,
        "top_vendors":  []interface{}{}, // simplified for fallback
        "top_cwe":      []interface{}{},
    }, nil
}
```

## Acceptance Criteria

- [x] `POST /api/v2/cves/search {"query":"log4j"}` → 200 với CVE results
- [x] `POST /api/v2/cves/search` without query body → still works (match_all)
- [x] `POST /api/v2/cves/search/semantic {"query":"remote code execution"}` → semantic results
- [x] `POST /api/v2/cves/search/semantic {}` → 400 "query is required"
- [x] `GET /api/v2/cves/aggregations` → `by_severity` buckets
- [x] OpenSearch down → GetAggregations falls back to PostgreSQL gracefully
- [x] `/api/v2/cves/search/semantic` routing không bị conflict với `/api/v2/cves/search`
- [x] `go build ./...` pass
