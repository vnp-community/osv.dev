# Solution 007: KEV Stats Schema, SSE Token Auth, API Key Response

**Status**: Proposed  
**Target Service**: `apps/osv` (Gateway auth middleware), `data-service`  
**Related CR**: [CR-007-kev-notifications-sla-gaps.md](../CR-007-kev-notifications-sla-gaps.md)

## 1. SSE Token Auth — Gateway Middleware

Giải pháp quan trọng nhất: cho phép authenticate SSE qua `?token` query param.

```go
// apps/osv/internal/gateway/auth/middleware.go

// AuthenticateSSE validates JWT từ query param "token" (fallback to Authorization header)
func (m *AuthMiddleware) AuthenticateSSE(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        tokenStr := r.URL.Query().Get("token")
        if tokenStr == "" {
            tokenStr = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
        }
        if tokenStr == "" {
            http.Error(w, `{"error":"UNAUTHORIZED"}`, http.StatusUnauthorized)
            return
        }
        
        claims, err := m.validator.Validate(tokenStr)
        if err != nil {
            http.Error(w, `{"error":"INVALID_TOKEN"}`, http.StatusUnauthorized)
            return
        }
        
        ctx := context.WithValue(r.Context(), claimsKey, claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

```go
// router.go — dùng sseAuth cho SSE endpoints
sseAuth := chain(authMW.AuthenticateSSE, transform.InjectUserHeaders)

// Cập nhật SSE routes
mux.Handle("GET /api/v1/notifications/stream",
    sseAuth(proxy.ForwardSSE("notification-service:8087")))
mux.Handle("GET /api/v1/scans/{id}/stream",
    sseAuth(proxy.ForwardSSE("scan-service:8084")))
```

## 2. KEV Stats Response Schema — data-service

Cập nhật handler `GetStats` trong data-service để trả về đầy đủ `KEVStatsResponse`:

```go
// services/data-service/internal/delivery/http/kev_handler.go
type KEVStatsResponse struct {
    Stats           KEVStats    `json:"stats"`
    ByVendor        []struct{
        Vendor string `json:"vendor"`
        Count  int    `json:"count"`
    } `json:"by_vendor"`
    RecentAdditions []KEVEntry  `json:"recent_additions"`
}

func (h *KEVHandler) GetStats(w http.ResponseWriter, r *http.Request) {
    stats, _    := h.repo.GetStats(r.Context())
    byVendor, _ := h.repo.GetStatsByVendor(r.Context(), 10) // top 10
    recent, _   := h.repo.GetRecent(r.Context(), 10)        // last 10 added
    
    json.NewEncoder(w).Encode(KEVStatsResponse{
        Stats:           stats,
        ByVendor:        byVendor,
        RecentAdditions: recent,
    })
}
```

## 3. KEV List Filter Params — data-service

Handler `ListKEV` cần parse thêm query params:

```go
func (h *KEVHandler) ListKEV(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    
    filter := KEVFilter{
        RansomwareOnly: q.Get("ransomware_only") == "true",
        Vendor:         q.Get("vendor"),
        DateFrom:       q.Get("date_from"),
        DateTo:         q.Get("date_to"),
        SortBy:         q.Get("sort_by"), // date_added_desc|date_added_asc|vendor_asc
        Page:           parseIntDefault(q.Get("page"), 1),
        PageSize:       parseIntDefault(q.Get("page_size"), 20),
    }
    
    entries, total, stats, _ := h.repo.List(r.Context(), filter)
    
    json.NewEncoder(w).Encode(map[string]interface{}{
        "data":      entries,
        "total":     total,
        "page":      filter.Page,
        "page_size": filter.PageSize,
        "stats":     stats,
    })
}
```
