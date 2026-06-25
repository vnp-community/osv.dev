# Solution 006: Reports v1 Alias, SLA Overview BFF, Jira PUT

**Status**: Proposed  
**Target Service**: `apps/osv` (Gateway), `finding-service`, `sla-service`, `jira-service`  
**Related CR**: [CR-006-reports-sla-jira-gaps.md](../CR-006-reports-sla-jira-gaps.md)

## 1. Reports v1 — Gateway Path Rewrite Middleware

Thay vì thêm handler mới trong finding-service, dùng path rewrite tại gateway:

```go
// apps/osv/internal/gateway/transform/path_rewrite.go
package transform

// RewriteV1ToV2 rewrites /api/v1/<path> to /api/v2/<path> before proxying
func RewriteV1ToV2(prefix string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            r.URL.Path = strings.Replace(r.URL.Path, "/api/v1/"+prefix, "/api/v2/"+prefix, 1)
            next.ServeHTTP(w, r)
        })
    }
}
```

```go
// router.go
rewriteReports := transform.RewriteV1ToV2("reports")

mux.Handle("GET /api/v1/reports",
    protected(rewriteReports(transform.UserScopeFilter(proxy.Forward("finding-service:8085")))))
mux.Handle("POST /api/v1/reports",
    protected(rewriteReports(rl.Limit("5/minute")(proxy.Forward("finding-service:8085")))))
mux.Handle("GET /api/v1/reports/{id}",
    protected(rewriteReports(proxy.Forward("finding-service:8085"))))
mux.Handle("DELETE /api/v1/reports/{id}",
    protected(rewriteReports(proxy.Forward("finding-service:8085"))))
mux.Handle("GET /api/v1/reports/{id}/download",
    protected(rewriteReports(proxy.ForwardWithTimeout("finding-service:8085", 30*time.Second))))
```

## 2. SLA Overview — In-Gateway BFF

Tạo BFF handler đơn giản aggregate từ sla-service:

```go
// apps/osv/internal/gateway/bff/sla_overview.go
package bff

type SLAOverviewBFF struct {
    slaServiceAddr string
    httpClient     *http.Client
}

func (b *SLAOverviewBFF) HandleOverview(w http.ResponseWriter, r *http.Request) {
    // Gọi sla-service dashboard endpoint
    resp, err := b.httpClient.Get("http://" + b.slaServiceAddr + "/api/v2/sla-dashboard")
    if err != nil {
        http.Error(w, "upstream error", 502)
        return
    }
    defer resp.Body.Close()
    
    var dashboard SLADashboardData
    json.NewDecoder(resp.Body).Decode(&dashboard)
    
    // Extract summary
    summary := dashboard.Summary
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(summary)
}
```

```go
// router.go
slaOverviewBFF := bff.NewSLAOverviewBFF("sla-service:8086")
mux.Handle("GET /api/v1/sla/overview",
    protected(http.HandlerFunc(slaOverviewBFF.HandleOverview)))
```

## 3. Jira Config PUT — Gateway Alias

Jira-service dùng upsert logic trong POST handler. Gateway chỉ cần thêm PUT alias:

```go
// router.go — thêm sau GET và POST /jira/config
mux.Handle("PUT /api/v1/jira/config", adminOnly(proxy.Forward("jira-service:8088")))
```

Jira-service handler cần xử lý cả `PUT /jira/config` (hoặc có thể map PUT → POST nội bộ trong jira-service):

```go
// services/jira-service/internal/delivery/http/router.go
r.Put("/jira/config", h.UpsertConfig)    // Thêm mới
r.Post("/jira/config", h.UpsertConfig)   // Giữ nguyên
```
