# SOL-009: Search-Service & Audit-Service — Missing Features

> **Bugs giải quyết**: BUG-003 (Semantic Suggestions), BUG-004 (Browse Root, DBInfo), BUG-016 (Audit Log), BUG-017 (Search Recent/Suggested)  
> **Services**: `services/search-service` (port 8083), `services/audit-service` (port 8090), `services/data-service` (port 8082)  
> **Architecture ref**: §3.3 Search-Service, §3.10 Audit-Service, §3.2 Data-Service  
> **Status**: `[x] DONE`

## Kết Quả Thực Thi

**Đã hoàn thành:**

| Fix | Service | File | Trạng thái |
|---|---|---|---|
| Mount `fullRouter` (NewRouter) trên `/api/v2/` | search-service | `cmd/server/main.go` | ✅ Fixed (TASK-010) |
| `BrowseHandler.Mount()` trên browseRouter | search-service | `cmd/server/main.go` | ✅ Fixed (TASK-010) |
| `GET /api/v2/browse` → search-service | gateway | `apps/osv/internal/gateway/router.go` | ✅ Đã có |
| `POST /api/v2/cves/search/semantic` → search-service | gateway | `apps/osv/internal/gateway/router.go` | ✅ Đã có |
| `GET /api/v2/cves/search/semantic/suggestions` | gateway | `apps/osv/internal/gateway/router.go` | ✅ Thêm mới (SOL-001 fix) |
| `GET /api/v1/search/recent` | gateway | `apps/osv/internal/gateway/router.go` | ✅ Thêm mới (SOL-001 fix) |
| `GET /api/v1/search/suggested` | gateway | `apps/osv/internal/gateway/router.go` | ✅ Thêm mới (SOL-001 fix) |
| `GET /api/v1/audit-log` → `ListAuditLog` handler | audit-service | `internal/delivery/http/router.go` | ✅ Đã có |
| `GET /api/v2/dbinfo` alias + wire InfoHandler | data-service | `internal/delivery/http/cve_handler.go`, `embed/server.go` | ✅ Fixed (TASK-010) |

**Build verify**: `go build ./...` ✅ search-service, data-service, apps/osv


---

## BUG-017: Global Search — Recent & Suggested

### Phân Tích

`/api/v1/search/recent` và `/api/v1/search/suggested` là **per-user** features thuộc identity context, không phải CVE data. Nên implement trong search-service với user context từ `X-User-ID` header.

### Schema

```sql
-- Thêm vào osv_identity hoặc tạo table mới trong search-service schema
CREATE TABLE IF NOT EXISTS search_history (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id   UUID NOT NULL,
    query     TEXT NOT NULL,
    type      VARCHAR(20) DEFAULT 'full-text', -- "full-text", "semantic", "command"
    result_count INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_search_history_user ON search_history(user_id, created_at DESC);

-- Auto-cleanup: giữ 100 recent searches per user
-- Implement via trigger hoặc cron
```

### HTTP Handlers

```go
// services/search-service/internal/delivery/http/search_handler.go

// GET /api/v1/search/recent
func (h *SearchHandler) GetRecentSearches(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    limit  := parseIntParam(r, "limit", 10)
    
    recent, err := h.searchHistoryRepo.GetRecent(r.Context(), userID, limit)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "items": recent,
        "total": len(recent),
    })
}

// GET /api/v1/search/suggested
func (h *SearchHandler) GetSuggestedSearches(w http.ResponseWriter, r *http.Request) {
    // Suggested searches là static curated list (hoặc từ DB)
    // Có thể mở rộng thành AI-generated suggestions sau
    
    suggested := []string{
        "Critical findings due this week",
        "Assets with KEV vulnerabilities",
        "High severity unresolved findings",
        "Log4j vulnerabilities",
        "Missing patches in production",
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "items": suggested,
    })
}

// GET /api/v1/search?q=log4j&type=cve (main search - nếu chưa có)
func (h *SearchHandler) GlobalSearch(w http.ResponseWriter, r *http.Request) {
    q     := r.URL.Query().Get("q")
    sType := r.URL.Query().Get("type") // "cve", "finding", "asset", "all"
    userID := r.Header.Get("X-User-ID")
    
    // Save to search history
    defer func() {
        if q != "" {
            h.searchHistoryRepo.Save(context.Background(), SearchHistoryEntry{
                UserID: userID,
                Query:  q,
                Type:   "full-text",
            })
        }
    }()
    
    results, err := h.searchUC.GlobalSearch(r.Context(), GlobalSearchRequest{
        Query:  q,
        Type:   sType,
        UserID: userID,
    })
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, results)
}
```

```go
// Router
r.GET("/api/v1/search/recent",    authMiddleware(h.GetRecentSearches))    // TRƯỚC wildcard
r.GET("/api/v1/search/suggested", authMiddleware(h.GetSuggestedSearches))
r.GET("/api/v1/search",           authMiddleware(h.GlobalSearch))
```

---

## BUG-003: CVE Semantic Suggestions

### Phân Tích

Theo architecture §3.3, search-service thực hiện cả BM25 và semantic search. `GET /api/v2/cves/search/semantic/suggestions` là **autocomplete** cho semantic search query box.

```go
// services/search-service/internal/delivery/http/semantic_handler.go

// GET /api/v2/cves/search/semantic/suggestions?q=log4j
func (h *SemanticHandler) GetSuggestions(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query().Get("q")
    if len(q) < 2 {
        respondJSON(w, http.StatusOK, map[string]interface{}{"suggestions": []string{}})
        return
    }
    
    // Strategy 1: Full-text search on CVE descriptions + titles
    // Trả về các cụm từ phổ biến liên quan đến query
    suggestions, err := h.opensearch.Autocomplete(r.Context(), q, 10)
    if err != nil {
        // Fallback: static suggestions
        suggestions = buildStaticSuggestions(q)
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "suggestions": suggestions,
        "query": q,
    })
}
```

**OpenSearch completion suggester**:
```json
{
  "suggest": {
    "cve-suggest": {
      "prefix": "log4j",
      "completion": {
        "field": "suggest",
        "size": 10,
        "skip_duplicates": true
      }
    }
  }
}
```

---

## BUG-004a: Browse Root — GET /api/v2/browse

### Routing Conflict

`GET /api/v2/browse` là static path — nếu router có `/browse/{vendor}` thì có thể `/browse` bị 404 do không match (không có `{vendor}`).

```go
// services/search-service/internal/delivery/http/browse_handler.go

// GET /api/v2/browse — Root browse (list all vendors)
func (h *BrowseHandler) BrowseRoot(w http.ResponseWriter, r *http.Request) {
    // Try cache
    cacheKey := "browse:vendors:all"
    if cached, err := h.redis.Get(r.Context(), cacheKey).Bytes(); err == nil {
        w.Header().Set("Content-Type", "application/json")
        w.Write(cached)
        return
    }
    
    vendors, total, err := h.browseUC.ListVendors(r.Context(), BrowseFilter{
        Limit: parseIntParam(r, "limit", 100),
        Page:  parseIntParam(r, "page", 1),
    })
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    result := map[string]interface{}{
        "vendors": vendors,
        "total":   total,
    }
    
    // Cache 24h (vendors don't change often)
    h.redis.Set(r.Context(), cacheKey, result, 24*time.Hour)
    
    respondJSON(w, http.StatusOK, result)
}
```

**Router** (CRITICAL order):
```go
// search-service router — STATIC trước WILDCARD
r.GET("/api/v2/browse",                     h.BrowseRoot)           // THÊM MỚI — TRƯỚC
r.GET("/api/v2/browse/{vendor}",            h.BrowseByVendor)       // Đã có — SAU
r.GET("/api/v2/browse/{vendor}/{product}",  h.BrowseByVendorProduct) // Đã có — SAU
```

---

## BUG-004b: DBInfo — GET /api/v2/dbinfo

### Phân Tích

Theo architecture §3.1: `Public | /api/v1/dbinfo | data-service:8082`. DBInfo là public endpoint.

```go
// services/data-service/internal/delivery/http/info_handler.go

// GET /api/v2/dbinfo (alias /api/v1/dbinfo đã map theo arch)
func (h *InfoHandler) GetDBInfo(w http.ResponseWriter, r *http.Request) {
    // Query thống kê database
    var info DBInfo
    
    // Count CVEs
    h.db.QueryRow(r.Context(), "SELECT COUNT(*) FROM cves").Scan(&info.CVECount)
    // Count KEV
    h.db.QueryRow(r.Context(), "SELECT COUNT(*) FROM kev_entries").Scan(&info.KEVCount)
    // Last sync time
    h.db.QueryRow(r.Context(), "SELECT MAX(modified_at) FROM cves").Scan(&info.LastUpdated)
    // Count CWE
    h.db.QueryRow(r.Context(), "SELECT COUNT(*) FROM cwe_weaknesses").Scan(&info.CWECount)
    // Count CAPEC
    h.db.QueryRow(r.Context(), "SELECT COUNT(*) FROM capec_patterns").Scan(&info.CAPECCount)
    
    info.Version = "2026.06"
    info.SchemaVersion = "3.0"
    
    respondJSON(w, http.StatusOK, info)
}

type DBInfo struct {
    Version       string `json:"version"`
    SchemaVersion string `json:"schema_version"`
    CVECount      int    `json:"cve_count"`
    KEVCount      int    `json:"kev_count"`
    CWECount      int    `json:"cwe_count"`
    CAPECCount    int    `json:"capec_count"`
    LastUpdated   string `json:"last_updated"`
}
```

**Gateway routing** (public — no auth):
```go
// apps/osv/internal/gateway/setup.go
mux.Handle("GET /api/v2/dbinfo",
    proxy.Forward("data-service:8082"))  // NO auth wrapper
```

---

## BUG-016: Audit Log — GET /api/v1/audit-log

### Phân Tích

Theo architecture §3.10, audit-service có:
- Table `audit_events` (monthly partitioned, HMAC-signed, read-only via RLS)
- 40+ NATS subscriptions → records all events
- Schema `osv_audit`

Endpoint chỉ thiếu HTTP handler — data đã có đầy đủ.

```go
// services/audit-service/internal/delivery/http/audit_handler.go

// GET /api/v1/audit-log?action=finding.close&user_id=...&from=...&to=...&page=1&limit=50
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
    filter := AuditFilter{
        Page:  parseIntParam(r, "page", 1),
        Limit: parseIntParam(r, "limit", 50),
    }
    
    // Optional filters
    if action := r.URL.Query().Get("action"); action != "" {
        filter.Action = action
    }
    if userID := r.URL.Query().Get("user_id"); userID != "" {
        filter.ActorID = userID
    }
    if resType := r.URL.Query().Get("resource_type"); resType != "" {
        filter.ResourceType = resType
    }
    if from := r.URL.Query().Get("from"); from != "" {
        filter.From, _ = time.Parse(time.RFC3339, from)
    }
    if to := r.URL.Query().Get("to"); to != "" {
        filter.To, _ = time.Parse(time.RFC3339, to)
    }
    
    events, total, err := h.auditRepo.List(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "items": events,
        "total": total,
        "page":  filter.Page,
        "limit": filter.Limit,
    })
}
```

**SQL** (tuân thủ RLS — read-only):
```sql
-- audit-service/internal/infra/postgres/audit_repo.go
SELECT id, actor_id, actor_email, action, resource_type, resource_id,
       before_json, after_json, created_at,
       COUNT(*) OVER() AS total
FROM audit_events
WHERE ($1 = '' OR action = $1)
  AND ($2::uuid IS NULL OR actor_id = $2)
  AND ($3 = '' OR resource_type = $3)
  AND ($4::timestamptz IS NULL OR created_at >= $4)
  AND ($5::timestamptz IS NULL OR created_at <= $5)
ORDER BY created_at DESC
LIMIT $6 OFFSET $7
```

**Router**:
```go
// services/audit-service/internal/delivery/http/router.go
r.GET("/api/v1/audit-log", adminMiddleware(h.List))
```

**Gateway**:
```go
// Sprint 3: /api/v1/audit-log → audit-service:8090 (Admin only)
mux.Handle("GET /api/v1/audit-log",
    adminOnly(proxy.Forward("audit-service:8090")))
```

### Response Schema

```go
type AuditEventResponse struct {
    ID           string      `json:"id"`
    ActorID      string      `json:"actor_id"`
    ActorEmail   string      `json:"actor_email,omitempty"`
    Action       string      `json:"action"`       // "finding.close", "product.create"
    ResourceType string      `json:"resource_type,omitempty"`
    ResourceID   string      `json:"resource_id,omitempty"`
    Changes      interface{} `json:"changes,omitempty"` // AfterJSON or diff
    IPAddress    string      `json:"ip,omitempty"`
    CreatedAt    time.Time   `json:"timestamp"`
}
```
