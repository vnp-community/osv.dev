# SOL-V6-005: Implement Missing Routes — 13 Routes chưa có (404)

**Bugs:** BUG-V6-001 → BUG-V6-013  
**Task:** TASK-V6-005  
**Services:** `identity-service` (:8081), `search-service` (:8083), `finding-service` (:8085), `jira-service` (:8088), `audit-service` (:8090), `ai-service` (:9103), `apps/osv` (gateway)  
**Kiến trúc tham chiếu:** `01-architecture.md §3.1, §3.4, §3.5, §3.9, §3.10, §3.11`, `02-technical-design.md §9, §10, §14`

---

## Nhóm 1: Auth MFA Routes (BUG-V6-001, BUG-V6-002)

**Service:** `identity-service` (:8081)  
**Phân tích:** `01-architecture.md §3.4` định nghĩa MFA aliases rõ ràng:
```
MFA aliases (BFF path rewrite trong gateway):
  GET  /api/v1/auth/mfa/setup   → /api/v1/auth/totp/setup
  POST /api/v1/auth/mfa/confirm → /api/v1/auth/totp/verify
```

Giải pháp: Gateway thực hiện path rewrite (BFF pattern), KHÔNG cần implement handler mới trong identity-service.

### Fix — Gateway BFF Path Rewrite

**File:** `apps/osv/internal/gateway/router.go`

```go
// BUG-V6-001: GET /api/v1/auth/mfa/setup → rewrite to /api/v1/auth/totp/setup
mux.Handle("GET /api/v1/auth/mfa/setup",
    authMiddleware(proxy.ForwardWithRewrite(
        identityServiceAddr,
        func(path string) string { return "/api/v1/auth/totp/setup" },
    )))

// BUG-V6-002: POST /api/v1/auth/mfa/confirm → rewrite to /api/v1/auth/totp/verify
mux.Handle("POST /api/v1/auth/mfa/confirm",
    authMiddleware(proxy.ForwardWithRewrite(
        identityServiceAddr,
        func(path string) string { return "/api/v1/auth/totp/verify" },
    )))
```

**ForwardWithRewrite implementation:**

```go
// apps/osv/internal/gateway/proxy.go

// ForwardWithRewrite forward request đến upstream nhưng rewrite path trước
func (p *ReverseProxy) ForwardWithRewrite(target string, rewriteFn func(string) string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        newPath := rewriteFn(r.URL.Path)
        r = r.Clone(r.Context())
        r.URL.Path = newPath
        p.Forward(target).ServeHTTP(w, r)
    }
}
```

> **Note:** Nếu identity-service chưa có `/auth/totp/setup` và `/auth/totp/verify`, cần implement thêm theo `02-technical-design.md §10` TOTP pattern.

---

## Nhóm 2: CVE v2 Browse & DBInfo (BUG-V6-003, BUG-V6-004)

**Service:** `search-service` (:8083), `data-service` (:8082)  
**Phân tích:** `01-architecture.md §3.1` định nghĩa:
```
Sprint 4: /api/v2/cves/export, /api/v2/browse/* → search-service:8083
Public: /api/v1/dbinfo → data-service:8082
```

### Fix — Gateway Route Registration

```go
// apps/osv/internal/gateway/router.go

// BUG-V6-003: GET /api/v2/browse → search-service
// Vendor/product browse entrypoint (list all vendors)
mux.Handle("GET /api/v2/browse",
    authMiddleware(proxy.Forward(searchServiceAddr)))

// GET /api/v2/browse/{vendor} → search-service (nếu chưa có)
mux.Handle("GET /api/v2/browse/{vendor}",
    authMiddleware(proxy.Forward(searchServiceAddr)))

// GET /api/v2/browse/{vendor}/{product} → search-service (nếu chưa có)
mux.Handle("GET /api/v2/browse/{vendor}/{product}",
    authMiddleware(proxy.Forward(searchServiceAddr)))

// BUG-V6-004: GET /api/v2/dbinfo → data-service (public, no auth)
// Note: /api/v1/dbinfo đã có (public), nhưng /api/v2/dbinfo cần thêm
mux.Handle("GET /api/v2/dbinfo",
    proxy.Forward(dataServiceAddr))  // Public route
```

**Search-service browse handler** (nếu chưa có):

```go
// services/search-service/internal/delivery/http/browse_handler.go

// GET /api/v2/browse → list tất cả vendors có CVEs
func (h *BrowseHandler) ListVendors(w http.ResponseWriter, r *http.Request) {
    page, pageSize := parsePagination(r)
    
    vendors, total, err := h.browseUC.ListVendors(r.Context(), page, pageSize)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }
    
    writeJSON(w, http.StatusOK, map[string]interface{}{
        "vendors": vendors,
        "total":   total,
        "page":    page,
    })
}
```

---

## Nhóm 3: Products Grades (BUG-V6-005)

**Service:** `finding-service` (:8085)  
**Phân tích:** `02-technical-design.md §5.4` đã định nghĩa `GradingUseCase` và `ComputeGrade()`.  
Route `/api/v1/products/grades` cần được đăng ký trong gateway và finding-service.

### Fix — Gateway Registration

```go
// apps/osv/internal/gateway/router.go

// BUG-V6-005: GET /api/v1/products/grades → finding-service
// QUAN TRỌNG: Đăng ký TRƯỚC /api/v1/products/{id} để tránh conflict
mux.Handle("GET /api/v1/products/grades",
    authMiddleware(proxy.Forward(findingServiceAddr)))
```

### Fix — Finding-Service Router

```go
// services/finding-service/internal/delivery/http/router.go

// Đăng ký TRƯỚC wildcard routes
mux.Handle("GET /products/grades", authMiddleware(h.ListProductGrades))
mux.Handle("GET /products/types",  authMiddleware(h.ListProductTypes))   // nếu chưa có

// Sau đó mới đăng ký wildcard
mux.Handle("GET    /products/{id}", authMiddleware(h.GetProduct))
```

### Fix — Handler

```go
// services/finding-service/internal/delivery/http/product_handler.go

// GET /products/grades → tính grade cho tất cả products của user
func (h *ProductHandler) ListProductGrades(w http.ResponseWriter, r *http.Request) {
    // Filter theo query params
    productIDs := parseUUIDList(r.URL.Query().Get("product_ids"))

    grades, err := h.gradingUC.ComputeForAll(r.Context(), productIDs)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{
        "grades": grades,
        "total":  len(grades),
    })
}
```

**Response schema:**
```json
{
  "grades": [
    {
      "product_id": "uuid",
      "product_name": "My Web App",
      "grade": "B",
      "critical_count": 0,
      "high_count": 3,
      "total_active": 8,
      "computed_at": "2026-06-24T09:00:00Z"
    }
  ],
  "total": 1
}
```

---

## Nhóm 4: AI Insights (BUG-V6-006)

**Service:** `ai-service` (:9103)  
**Phân tích:** `01-architecture.md §3.11` liệt kê các AI endpoints, `GET /ai/insights` chưa được đề cập trong list → cần thêm.

### Fix — AI-Service Handler

```go
// services/ai-service/internal/delivery/http/insights_handler.go

type InsightsResponse struct {
    TotalEnriched     int       `json:"total_enriched"`
    PendingTriage     int       `json:"pending_triage"`
    AutoTriaged       int       `json:"auto_triaged"`
    HumanReviewed     int       `json:"human_reviewed"`
    AccuracyRate      float64   `json:"accuracy_rate"`
    TopVulnerableComponents []struct {
        Component string `json:"component"`
        Count     int    `json:"count"`
    } `json:"top_vulnerable_components"`
    LastEnrichmentAt  *time.Time `json:"last_enrichment_at"`
}

// GET /ai/insights → AI platform insights summary
func (h *AIHandler) GetInsights(w http.ResponseWriter, r *http.Request) {
    insights, err := h.insightsUC.Execute(r.Context())
    if err != nil {
        h.log.Error().Err(err).Msg("get insights failed")
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }
    writeJSON(w, http.StatusOK, insights)
}
```

### Fix — Gateway Registration

```go
// apps/osv/internal/gateway/router.go

// BUG-V6-006: GET /api/v1/ai/insights → ai-service:9103
mux.Handle("GET /api/v1/ai/insights",
    authMiddleware(proxy.Forward(aiServiceAddr)))
```

---

## Nhóm 5: JIRA Integration (BUG-V6-007, 008, 009, 010)

**Service:** `jira-service` (:8088)  
**Phân tích:** `01-architecture.md §3.9` và `§3.1`:
```
Sprint 3: /api/v1/jira/config → jira-service:8088 | Admin
```

Cần thêm: `GET /jira/config` (đã có route nhưng trả 404, nghĩa là route chưa đăng ký trong gateway), `POST /jira/config/test`, và `/integrations/jira` (alias routes).

### Fix — Gateway Registration

```go
// apps/osv/internal/gateway/router.go

// BUG-V6-007: GET /api/v1/jira/config
mux.Handle("GET  /api/v1/jira/config", adminOnly(proxy.Forward(jiraServiceAddr)))

// PATCH /api/v1/jira/config (nếu chưa có)
mux.Handle("PATCH /api/v1/jira/config", adminOnly(proxy.Forward(jiraServiceAddr)))

// BUG-V6-008: POST /api/v1/jira/config/test
mux.Handle("POST /api/v1/jira/config/test", adminOnly(proxy.Forward(jiraServiceAddr)))

// BUG-V6-009, 010: /integrations/jira → alias của /jira/config
// Dùng BFF rewrite pattern
mux.Handle("GET /api/v1/integrations/jira",
    adminOnly(proxy.ForwardWithRewrite(jiraServiceAddr,
        func(path string) string { return "/jira/config" })))

mux.Handle("PUT /api/v1/integrations/jira",
    adminOnly(proxy.ForwardWithRewrite(jiraServiceAddr,
        func(path string) string { return "/jira/config" })))
```

### Fix — JIRA-Service: Thêm Test Connection Handler

```go
// services/jira-service/internal/delivery/http/handler.go

// POST /jira/config/test → test kết nối JIRA
func (h *JiraHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
    // Lấy config hiện tại từ DB
    cfg, err := h.configRepo.Get(r.Context())
    if err != nil {
        if errors.Is(err, domain.ErrNotFound) {
            writeError(w, http.StatusBadRequest, "JIRA not configured")
            return
        }
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }

    start := time.Now()
    if err := h.jiraClient.TestConnection(r.Context(), cfg); err != nil {
        writeJSON(w, http.StatusOK, map[string]interface{}{
            "ok":         false,
            "error":      err.Error(),
            "latency_ms": time.Since(start).Milliseconds(),
        })
        return
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{
        "ok":         true,
        "latency_ms": time.Since(start).Milliseconds(),
    })
}
```

---

## Nhóm 6: Audit Log (BUG-V6-011)

**Service:** `audit-service` (:8090)  
**Phân tích:** `01-architecture.md §3.1`:
```
Sprint 3: /api/v1/audit-log → audit-service:8090 | Admin
```

Route đã được định nghĩa trong sprint plan nhưng chưa đăng ký trong gateway.

### Fix — Gateway Registration

```go
// apps/osv/internal/gateway/router.go

// BUG-V6-011: GET /api/v1/audit-log → audit-service:8090
mux.Handle("GET /api/v1/audit-log",
    adminOnly(proxy.Forward(auditServiceAddr)))

// Nếu cần export
mux.Handle("GET /api/v1/audit-log/export",
    adminOnly(rateLimiter.Limit("2/minute")(proxy.Forward(auditServiceAddr))))
```

### Audit-Service Response Schema

Theo `02-technical-design.md §9.1` và `§9.2`:

```go
// GET /audit-log handler phải return:
// {
//   "entries": [AuditEvent...],
//   "total": int,
//   "page": int,
//   "page_size": int
// }

type AuditEntryDTO struct {
    ID           string    `json:"id"`
    ActorID      string    `json:"actor_id"`
    ActorEmail   string    `json:"actor_email"`
    Action       string    `json:"action"`        // "finding.close", "product.create"
    ResourceType string    `json:"resource_type"`
    ResourceID   string    `json:"resource_id"`
    BeforeJSON   any       `json:"before,omitempty"`
    AfterJSON    any       `json:"after,omitempty"`
    CreatedAt    time.Time `json:"created_at"`
}
```

---

## Nhóm 7: Global Search (BUG-V6-012, BUG-V6-013)

**Service:** `search-service` (:8083) hoặc In-Gateway BFF  
**Phân tích:** Global search là feature cross-cutting — có thể là BFF aggregating từ nhiều services.

### Fix — Gateway + Search-Service

```go
// apps/osv/internal/gateway/router.go

// BUG-V6-012: GET /api/v1/search/recent → search-service (hoặc BFF từ Redis)
mux.Handle("GET /api/v1/search/recent",
    authMiddleware(proxy.Forward(searchServiceAddr)))

// BUG-V6-013: GET /api/v1/search/suggested → search-service
mux.Handle("GET /api/v1/search/suggested",
    authMiddleware(proxy.Forward(searchServiceAddr)))
```

**Search-service handlers:**

```go
// GET /api/v1/search/recent → recent search queries của current user
func (h *SearchHandler) GetRecentSearches(w http.ResponseWriter, r *http.Request) {
    userID := extractUserID(r)
    limit := parseInt(r.URL.Query().Get("limit"), 10)

    // Lưu trong Redis: sorted set "recent_searches:{user_id}"
    searches, err := h.searchHistoryRepo.GetRecent(r.Context(), userID, limit)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{
        "searches": searches,
    })
}

// GET /api/v1/search/suggested?q=CVE → autocomplete suggestions
func (h *SearchHandler) GetSuggested(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query().Get("q")
    if q == "" {
        writeJSON(w, http.StatusOK, map[string]interface{}{"suggestions": []string{}})
        return
    }

    // OpenSearch prefix query trên cve_id và description
    suggestions, err := h.searchUC.GetSuggestions(r.Context(), q, 10)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{
        "suggestions": suggestions,
    })
}
```

---

## Summary — All Gateway Route Additions

**File:** `apps/osv/internal/gateway/router.go`

```go
// ── Auth MFA Aliases (BFF path rewrite) ──────────────────────────────────
mux.Handle("GET  /api/v1/auth/mfa/setup",   authMiddleware(proxy.ForwardWithRewrite(identitySvc, mfaSetupRewrite)))
mux.Handle("POST /api/v1/auth/mfa/confirm", authMiddleware(proxy.ForwardWithRewrite(identitySvc, mfaConfirmRewrite)))

// ── CVE v2 Browse & DBInfo ───────────────────────────────────────────────
mux.Handle("GET /api/v2/browse",            authMiddleware(proxy.Forward(searchSvc)))
mux.Handle("GET /api/v2/dbinfo",            proxy.Forward(dataSvc))  // public

// ── Products Grades ──────────────────────────────────────────────────────
mux.Handle("GET /api/v1/products/grades",   authMiddleware(proxy.Forward(findingSvc)))

// ── AI Insights ──────────────────────────────────────────────────────────
mux.Handle("GET /api/v1/ai/insights",       authMiddleware(proxy.Forward(aiSvc)))

// ── JIRA Integration ─────────────────────────────────────────────────────
mux.Handle("GET  /api/v1/jira/config",      adminOnly(proxy.Forward(jiraSvc)))
mux.Handle("POST /api/v1/jira/config/test", adminOnly(proxy.Forward(jiraSvc)))
mux.Handle("GET  /api/v1/integrations/jira",adminOnly(proxy.ForwardWithRewrite(jiraSvc, func(p string) string { return "/jira/config" })))
mux.Handle("PUT  /api/v1/integrations/jira",adminOnly(proxy.ForwardWithRewrite(jiraSvc, func(p string) string { return "/jira/config" })))

// ── Audit Log ────────────────────────────────────────────────────────────
mux.Handle("GET /api/v1/audit-log",         adminOnly(proxy.Forward(auditSvc)))

// ── Global Search ─────────────────────────────────────────────────────────
mux.Handle("GET /api/v1/search/recent",     authMiddleware(proxy.Forward(searchSvc)))
mux.Handle("GET /api/v1/search/suggested",  authMiddleware(proxy.Forward(searchSvc)))
```

---

## Verification

```bash
# Test toàn bộ missing routes sau khi fix:
python3 tests/client/test_all_endpoints.py 2>&1 | grep -E "(FAIL|✗)" | grep -E "(mfa|browse|dbinfo|grades|insights|jira|audit|search)"

# Expected: tất cả dòng trên PASS (không còn trong FAIL list)
```
