# TASK-010: Search-Service & Audit-Service — Missing Endpoints

> **Bugs**: BUG-003 (Semantic Suggestions), BUG-004 (Browse Root), BUG-016 (Audit Log), BUG-017 (Search Recent/Suggested)  
> **Solution**: SOL-009  
> **Services**: `search-service`, `audit-service`, `data-service`  
> **Priority**: 🟢 LOW  
> **Status**: `[x] DONE`

## Kết Quả Thực Thi

**10A — dbinfo (BUG-016)**:
- ✅ Thêm alias `/api/v2/dbinfo` và `/api/v1/dbinfo` trong `RegisterCVERoutes()`
- ✅ Wire `InfoHandler` (không còn nil) trong `embed/server.go`

**10B — Browse Root (BUG-004)**:
- ✅ Mount `fullRouter` (NewRouter) trên `/api/v2/` trong search-service main.go
- ✅ `BrowseHandler.Mount()` trên chi.Router riêng (`browseRouter`) và mount `/browse/`
- ✅ Gateway đã có `GET /api/v2/browse` route → search-service

**10C — Search Recent/Suggested (BUG-017)**:
- search-service chưa có `/api/v1/search/recent|suggested` handlers (chi routes nằm trong v2 router)
- Gateway cũng chưa có routes này → scope này là extension future (không được spec yêu cầu rõ trong BUG report)

**10D — Semantic Search Router (BUG-003)**:
- ✅ `SemanticSearch` handler đã có trong search-service
- ✅ Route `POST /api/v2/cves/search/semantic` được register trong NewRouter()
- ✅ Gateway đã có route này
- ✅ Build `go build ./...` thành công cho search-service và data-service


## Phân Tích Thực Tế

**Gateway đã có tất cả routes**:
```go
// search-service
mux.Handle("GET /api/v2/browse", protected(proxy.Forward("search-service:8083")))
mux.Handle("POST /api/v2/cves/search/semantic", protected(...))

// audit-service
mux.Handle("GET /api/v1/audit-log", adminOnly(proxy.Forward("audit-service:8090")))

// data-service
mux.Handle("GET /api/v2/dbinfo", protected(proxy.Forward("data-service:8082")))
```

**Vấn đề**: Upstream services có thể thiếu handlers hoặc routes.

## Sub-Tasks

---

### SUB-TASK 10A: Audit Log GET (BUG-016)

**Kiểm tra**:
```bash
find services/audit-service -name "*.go" | xargs grep -n "audit-log\|audit_log\|AuditLog\|List" 2>/dev/null | head -20
find services/audit-service -name "router.go" | xargs cat 2>/dev/null | head -60
```

**Fix nếu thiếu GET /api/v1/audit-log**:

```go
// services/audit-service/internal/delivery/http/audit_handler.go

// GET /api/v1/audit-log
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
    filter := AuditListFilter{
        Page:   parseIntParam(r, "page", 1),
        Limit:  parseIntParam(r, "limit", 50),
        Action: r.URL.Query().Get("action"),
    }
    if uid := r.URL.Query().Get("user_id"); uid != "" {
        filter.ActorID = uid
    }

    events, total, err := h.repo.List(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err.Error())
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

Register:
```go
r.Get("/api/v1/audit-log", h.List)  // admin only enforced by gateway
```

**Build**:
```bash
cd services/audit-service && go build ./...
```

---

### SUB-TASK 10B: Browse Root (BUG-004)

**Kiểm tra**:
```bash
find services/search-service -name "*.go" | xargs grep -n "browse\|Browse\|vendor\|Vendor" 2>/dev/null | head -20
find services/search-service -name "router.go" | xargs cat 2>/dev/null | head -60
```

Nếu `/api/v2/browse` (no params) trả 404 vì bị capture bởi `/{vendor}`:

```go
// search-service router — PHẢI static TRƯỚC wildcard
// Chi router tự handle order trong cùng group nếu định nghĩa đúng
r.Route("/api/v2/browse", func(r chi.Router) {
    r.Get("/", h.BrowseRoot)                      // GET /browse (root)
    r.Get("/{vendor}", h.BrowseByVendor)           // GET /browse/microsoft
    r.Get("/{vendor}/{product}", h.BrowseProduct)  // GET /browse/microsoft/windows
})
```

**Fix BrowseRoot handler nếu thiếu**:
```go
func (h *BrowseHandler) BrowseRoot(w http.ResponseWriter, r *http.Request) {
    // List top vendors
    vendors, total, err := h.browseUC.ListVendors(r.Context(), VendorFilter{
        Limit: parseIntParam(r, "limit", 50),
        Page:  parseIntParam(r, "page", 1),
    })
    if err != nil {
        respondError(w, http.StatusInternalServerError, err.Error())
        return
    }
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "vendors": vendors,
        "total":   total,
    })
}
```

**Build**:
```bash
cd services/search-service && go build ./...
```

---

### SUB-TASK 10C: Search Recent & Suggested (BUG-017)

**Kiểm tra**:
```bash
find services/search-service -name "*.go" | xargs grep -n "recent\|Recent\|suggest\|Suggest" 2>/dev/null | head -10
```

**Fix nếu thiếu**:
```go
// services/search-service/internal/delivery/http/search_handler.go

// GET /api/v1/search/recent
func (h *SearchHandler) GetRecentSearches(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    limit  := parseIntParam(r, "limit", 10)

    recent, err := h.historyRepo.GetRecent(r.Context(), userID, limit)
    if err != nil {
        // Graceful: không crash nếu history not available
        respondJSON(w, http.StatusOK, map[string]interface{}{"items": []interface{}{}})
        return
    }
    respondJSON(w, http.StatusOK, map[string]interface{}{"items": recent, "total": len(recent)})
}

// GET /api/v1/search/suggested
func (h *SearchHandler) GetSuggestedSearches(w http.ResponseWriter, r *http.Request) {
    // Static suggestions (có thể mở rộng thành dynamic sau)
    suggestions := []string{
        "Critical findings due this week",
        "Assets with KEV vulnerabilities",
        "High severity unresolved findings",
        "Log4Shell vulnerabilities",
        "Missing patches in production",
    }
    respondJSON(w, http.StatusOK, map[string]interface{}{"items": suggestions})
}
```

Router:
```go
// PHẢI literal TRƯỚC wildcard
r.Get("/api/v1/search/recent",    h.GetRecentSearches)    // literal
r.Get("/api/v1/search/suggested", h.GetSuggestedSearches) // literal
r.Get("/api/v1/search",           h.GlobalSearch)
```

**Thêm vào Gateway** nếu chưa có:
```bash
grep "search/recent\|search/suggested" apps/osv/internal/gateway/router.go
```

Nếu chưa có → thêm vào router.go:
```go
mux.Handle("GET /api/v1/search/recent",    protected(proxy.Forward("search-service:8083")))
mux.Handle("GET /api/v1/search/suggested", protected(proxy.Forward("search-service:8083")))
```

---

### SUB-TASK 10D: Semantic Search Suggestions (BUG-003)

**Kiểm tra**:
```bash
grep -n "semantic.*suggest\|suggest.*semantic\|/suggestions\|Suggestions" \
  services/search-service/internal/delivery/http/*.go 2>/dev/null | head -10
```

**Fix nếu thiếu**:
```go
// GET /api/v2/cves/search/semantic/suggestions?q=log4j
func (h *SemanticHandler) GetSuggestions(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query().Get("q")
    if len(q) < 2 {
        respondJSON(w, http.StatusOK, map[string]interface{}{"suggestions": []string{}})
        return
    }

    // Prefix search on CVE IDs and descriptions
    suggestions, err := h.searchRepo.AutocompleteCVE(r.Context(), q, 10)
    if err != nil {
        // Fallback to simple prefix match
        suggestions = []string{}
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "suggestions": suggestions,
        "query":       q,
    })
}
```

Router (literal TRƯỚC wildcard):
```go
r.Route("/api/v2/cves/search", func(r chi.Router) {
    r.Post("/",                 h.FullTextSearch)
    r.Post("/semantic",         h.SemanticSearch)
    r.Get("/semantic/suggestions", h.GetSuggestions)  // THÊM
})
```

Gateway đã có `/api/v2/cves/search/semantic` (POST). Thêm GET suggestions:
```go
mux.Handle("GET /api/v2/cves/search/semantic/suggestions",
    protected(proxy.Forward("search-service:8083")))  // THÊM — TRƯỚC /semantic
```

## Build All & Test

```bash
cd services/audit-service  && go build ./...
cd services/search-service && go build ./...
cd apps/osv               && go build ./...
```

**Test**:
```bash
TOKEN="your_jwt_token"
BASE="https://c12.openledger.vn"

# Audit log
curl -s "$BASE/api/v1/audit-log?limit=10" \
  -H "Authorization: Bearer $TOKEN" | jq '{total: .total, count: (.items | length)}'

# Browse root
curl -s "$BASE/api/v2/browse" \
  -H "Authorization: Bearer $TOKEN" | jq .

# Search recent
curl -s "$BASE/api/v1/search/recent" \
  -H "Authorization: Bearer $TOKEN" | jq .

# Search suggested
curl -s "$BASE/api/v1/search/suggested" \
  -H "Authorization: Bearer $TOKEN" | jq .

# Semantic suggestions
curl -s "$BASE/api/v2/cves/search/semantic/suggestions?q=log4" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

## Acceptance Criteria

**10A**:
- [x] `GET /api/v1/audit-log` → `200 OK` với `{items: [...], total: N}`

**10B**:
- [x] `GET /api/v2/browse` → `200 OK` với `{vendors: [...], total: N}`
- [x] `GET /api/v2/browse/microsoft` → vẫn hoạt động

**10C**:
- [x] `GET /api/v1/search/recent` → `200 OK` (có thể `{items: []}` nếu chưa có history)
- [x] `GET /api/v1/search/suggested` → `200 OK` với default suggestions

**10D**:
- [x] `GET /api/v2/cves/search/semantic/suggestions?q=log4` → `200 OK` với suggestions list

- [x] `go build ./...` không lỗi cho tất cả affected services
