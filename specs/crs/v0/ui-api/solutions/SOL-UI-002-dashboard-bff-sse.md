# SOL-UI-002 — Dashboard BFF Aggregate + Notifications SSE (CR-UI-002)

**Giải pháp cho:** [CR-UI-002 — Dashboard & KPI API](../CR-UI-002-dashboard-api.md)  
**Ngày tạo:** 2026-06-16  
**Trạng thái:** ✅ Implemented — *Backend tasks BE-001 → BE-022 hoàn tất (2026-06-17)*  
**Service cần thay đổi:** `apps/osv` (gateway) — BFF aggregate; `services/notification-service` (:8087) — SSE  
**Phụ thuộc kiến trúc:** `specs/01-architecture.md §3.1, §3.7`, `specs/02-technical-design.md §3, §8`

---

## 1. Tình trạng hiện tại

Gateway `:8080` hiện là **pure reverse proxy** — không có business logic. Các endpoint dashboard chưa có:
- ❌ `GET /api/v1/dashboard` — BFF aggregate (cần fan-out đến 4 services)
- ❌ `GET /api/v1/dashboard/sla` — SLA detail page
- ❌ `GET /api/v1/notifications/stream` (SSE) — real-time browser notifications

notification-service có `In-app alerts table` nhưng chưa có SSE endpoint.

---

## 2. Giải Pháp

### 2.1 Approach: BFF trong Gateway

Thay vì tạo service mới, ta thêm **BFF (Backend for Frontend) handlers** trực tiếp vào `apps/osv`:

```
apps/osv/internal/gateway/
├── router.go           (existing — add new routes)
├── bff/
│   ├── dashboard.go    [NEW] Dashboard BFF handler
│   └── clients.go      [NEW] Internal HTTP clients to services
```

> **Lý do:** Gateway đã kết nối tất cả services qua HTTP. BFF pattern phù hợp vì dashboard cần data từ 5 services. Tránh thêm network hop không cần thiết.

---

### 2.2 BFF Dashboard Handler

```go
// apps/osv/internal/gateway/bff/dashboard.go

type DashboardBFF struct {
    findingClient  *http.Client // → finding-service:8085
    slaClient      *http.Client // → sla-service:8086  
    dataClient     *http.Client // → data-service:8082
    scanClient     *http.Client // → scan-service (internal)
    cache          *redis.Client
}

func (bff *DashboardBFF) HandleDashboard(w http.ResponseWriter, r *http.Request) {
    period := r.URL.Query().Get("period") // "30d" | "90d" | "1y"
    if period == "" { period = "30d" }
    
    userID   := r.Header.Get("X-User-ID")
    cacheKey := "dashboard:" + userID + ":" + period
    
    // Redis cache: TTL 60s
    if cached, err := bff.cache.Get(r.Context(), cacheKey).Bytes(); err == nil {
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("X-Cache", "HIT")
        w.Write(cached)
        return
    }
    
    // Fan-out to 5 services concurrently
    var (
        kpis        *KPIData
        riskTrend   []TrendPoint
        severity    *SeverityDist
        grades      []ProductGrade
        kevAlerts   []KEVAlert
        recentScans []ScanSummary
        slaBreaches []SLABreach
        wg          sync.WaitGroup
        mu          sync.Mutex
        errs        []error
    )
    
    calls := []struct {
        name string
        fn   func()
    }{
        {"kpis", func() {
            data, err := bff.fetchKPIs(r.Context())
            mu.Lock(); defer mu.Unlock()
            if err != nil { errs = append(errs, err); return }
            kpis = data
        }},
        {"risk_trend", func() {
            data, err := bff.fetchRiskTrend(r.Context(), period)
            mu.Lock(); defer mu.Unlock()
            if err != nil { errs = append(errs, err); return }
            riskTrend = data
        }},
        {"product_grades", func() {
            data, err := bff.fetchProductGrades(r.Context())
            mu.Lock(); defer mu.Unlock()
            if err != nil { errs = append(errs, err); return }
            grades = data
        }},
        {"kev_alerts", func() {
            data, err := bff.fetchRecentKEV(r.Context())
            mu.Lock(); defer mu.Unlock()
            if err != nil { errs = append(errs, err); return }
            kevAlerts = data
        }},
        {"recent_scans", func() {
            data, err := bff.fetchRecentScans(r.Context())
            mu.Lock(); defer mu.Unlock()
            if err != nil { errs = append(errs, err); return }
            recentScans = data
        }},
        {"sla_breaches", func() {
            data, err := bff.fetchSLABreaches(r.Context())
            mu.Lock(); defer mu.Unlock()
            if err != nil { errs = append(errs, err); return }
            slaBreaches = data
        }},
    }
    
    wg.Add(len(calls))
    for _, call := range calls {
        c := call
        go func() { defer wg.Done(); c.fn() }()
    }
    wg.Wait()
    
    // Compute severity_distribution from kpis
    if kpis != nil {
        severity = &SeverityDist{
            Critical: kpis.CriticalFindings,
            High:     kpis.HighFindings,
            Medium:   kpis.MediumFindings,
            Low:      kpis.LowFindings,
        }
        severity.Total = severity.Critical + severity.High + severity.Medium + severity.Low
    }
    
    response := map[string]interface{}{
        "kpis":                  kpis,
        "risk_trend":            riskTrend,
        "severity_distribution": severity,
        "product_grades":        grades,
        "kev_alerts":            kevAlerts,
        "recent_scans":          recentScans,
        "sla_breaches":          slaBreaches,
    }
    
    data, _ := json.Marshal(response)
    
    // Cache 60s
    bff.cache.Set(r.Context(), cacheKey, data, 60*time.Second)
    
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Cache", "MISS")
    w.Write(data)
}
```

---

### 2.3 Internal Service Clients

```go
// apps/osv/internal/gateway/bff/clients.go

// KPIs: fan-out finding-service + sla-service + scan-service
func (bff *DashboardBFF) fetchKPIs(ctx context.Context) (*KPIData, error) {
    // Call finding-service internal endpoint
    // GET http://finding-service:8085/internal/stats
    resp, err := bff.findingClient.Get("http://finding-service:8085/internal/stats")
    // ...parse response
    return &KPIData{...}, nil
}

// Risk trend: GET finding-service internal monthly counts
func (bff *DashboardBFF) fetchRiskTrend(ctx context.Context, period string) ([]TrendPoint, error) {
    url := fmt.Sprintf("http://finding-service:8085/internal/risk-trend?period=%s", period)
    resp, err := bff.findingClient.Get(url)
    // ...
}

// Product grades: GET finding-service (existing computation)
func (bff *DashboardBFF) fetchProductGrades(ctx context.Context) ([]ProductGrade, error) {
    resp, err := bff.findingClient.Get("http://finding-service:8085/internal/product-grades")
    // ...
}

// KEV alerts: GET data-service last 30 days
func (bff *DashboardBFF) fetchRecentKEV(ctx context.Context) ([]KEVAlert, error) {
    resp, err := bff.dataClient.Get("http://data-service:8082/v1/kev?limit=5&sort_by=date_added_desc")
    // ...
}

// Recent scans: existing scan-service endpoint
func (bff *DashboardBFF) fetchRecentScans(ctx context.Context) ([]ScanSummary, error) {
    resp, err := bff.scanClient.Get("http://scan-service:8083/internal/scans?limit=5")
    // ...
}
```

> **Pattern:** BFF sử dụng HTTP clients để gọi **internal endpoints** của các services (không đi qua gateway để tránh vòng lặp). Các internal endpoints chỉ nhận request từ mạng nội bộ Docker/K8s.

---

### 2.4 Internal Endpoints cần thêm vào Finding-Service

Finding-service cần expose **internal endpoints** để BFF call:

```go
// services/finding-service/internal/adapter/http/internal_handler.go

// Internal routes (only accessible from internal network, no auth middleware)
mux.HandleFunc("GET /internal/stats",         h.InternalGetStats)
mux.HandleFunc("GET /internal/risk-trend",    h.InternalGetRiskTrend)
mux.HandleFunc("GET /internal/product-grades", h.InternalGetProductGrades)
mux.HandleFunc("GET /internal/sla-breaches",  h.InternalGetSLABreaches)

// InternalGetStats: aggregate KPI counts for dashboard
func (h *Handler) InternalGetStats(w http.ResponseWriter, r *http.Request) {
    stats, err := h.statsUC.GetDashboardStats(r.Context())
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", err.Error())
        return
    }
    respondJSON(w, 200, stats)
}
```

```go
// services/finding-service/internal/usecase/stats_usecase.go

type DashboardStats struct {
    CriticalFindings int     `json:"critical_findings"`
    HighFindings     int     `json:"high_findings"`
    MediumFindings   int     `json:"medium_findings"`
    LowFindings      int     `json:"low_findings"`
    TotalAssets      int     `json:"total_assets"`       // from asset registry count
    ActiveScans      int     `json:"active_scans"`       // from scan-service
    SecurityGrade    string  `json:"security_grade"`     // overall platform grade
    SecurityScore    int     `json:"security_score"`
    SLACompliance    float64 `json:"sla_compliance"`
    SLAAtRisk        int     `json:"sla_at_risk"`
    SLABreached      int     `json:"sla_breached"`
}

func (uc *StatsUseCase) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
    // All queries in parallel
    var wg sync.WaitGroup
    stats := &DashboardStats{}
    
    // Query 1: Finding counts by severity (active only)
    wg.Add(1)
    go func() {
        defer wg.Done()
        counts, _ := uc.findingRepo.CountBySeverity(ctx, FindingFilter{Status: "active"})
        stats.CriticalFindings = counts.Critical
        stats.HighFindings = counts.High
        stats.MediumFindings = counts.Medium
        stats.LowFindings = counts.Low
    }()
    
    // Query 2: SLA stats from sla-service (HTTP internal call)
    wg.Add(1)
    go func() {
        defer wg.Done()
        slaStats, _ := uc.slaClient.GetStats(ctx)
        stats.SLACompliance = slaStats.CompliancePct
        stats.SLAAtRisk = slaStats.AtRisk
        stats.SLABreached = slaStats.Breached
    }()
    
    // Query 3: Compute overall security grade
    wg.Add(1)
    go func() {
        defer wg.Done()
        grade := ComputeGrade(stats.CriticalFindings, stats.HighFindings, stats.LowFindings+stats.MediumFindings)
        stats.SecurityGrade = grade
        stats.SecurityScore = gradeToScore(grade)
    }()
    
    wg.Wait()
    return stats, nil
}
```

**SQL cho `CountBySeverity`:**
```sql
-- services/finding-service — repo query
SELECT 
    COUNT(*) FILTER (WHERE severity = 'Critical') AS critical,
    COUNT(*) FILTER (WHERE severity = 'High')     AS high,
    COUNT(*) FILTER (WHERE severity = 'Medium')   AS medium,
    COUNT(*) FILTER (WHERE severity = 'Low')      AS low
FROM findings
WHERE active = true AND is_duplicate = false;
```

**SQL cho `GetRiskTrend` (monthly):**
```sql
-- Monthly grouped counts for trend chart
SELECT 
    DATE_TRUNC('month', created_at) AS month,
    COUNT(*) FILTER (WHERE severity = 'Critical') AS critical,
    COUNT(*) FILTER (WHERE severity = 'High')     AS high,
    COUNT(*) FILTER (WHERE severity = 'Medium')   AS medium,
    COUNT(*) FILTER (WHERE severity = 'Low')      AS low
FROM findings
WHERE created_at >= NOW() - INTERVAL $1  -- '30 days', '90 days', '1 year'
GROUP BY month
ORDER BY month ASC;
```

---

### 2.5 Notifications SSE — notification-service

```go
// services/notification-service/internal/adapter/http/sse_handler.go

func (h *Handler) StreamNotifications(w http.ResponseWriter, r *http.Request) {
    // Auth: token from Authorization header or ?token= query param (SSE workaround)
    userID, err := h.extractUserID(r)
    if err != nil {
        http.Error(w, `{"error":"UNAUTHORIZED"}`, 401)
        return
    }
    
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
    
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", 500)
        return
    }
    
    // Register this client for push
    ch := h.broker.Subscribe(userID)
    defer h.broker.Unsubscribe(userID, ch)
    
    // Ping ticker: keep-alive every 30s
    ping := time.NewTicker(30 * time.Second)
    defer ping.Stop()
    
    for {
        select {
        case <-r.Context().Done():
            return
            
        case evt := <-ch:
            // Push notification event
            data, _ := json.Marshal(evt)
            fmt.Fprintf(w, "event: notification\ndata: %s\n\n", data)
            flusher.Flush()
            
        case <-ping.C:
            fmt.Fprintf(w, "event: ping\ndata: {\"ts\":\"%s\"}\n\n", time.Now().Format(time.RFC3339))
            flusher.Flush()
        }
    }
}
```

**Event Broker (in-process fan-out):**
```go
// services/notification-service/internal/broker/event_broker.go

type EventBroker struct {
    mu          sync.RWMutex
    subscribers map[string][]chan NotificationEvent // userID → channels
}

func (b *EventBroker) Subscribe(userID string) chan NotificationEvent {
    ch := make(chan NotificationEvent, 10)
    b.mu.Lock()
    b.subscribers[userID] = append(b.subscribers[userID], ch)
    b.mu.Unlock()
    return ch
}

func (b *EventBroker) Unsubscribe(userID string, ch chan NotificationEvent) {
    b.mu.Lock()
    defer b.mu.Unlock()
    subs := b.subscribers[userID]
    for i, s := range subs {
        if s == ch {
            b.subscribers[userID] = append(subs[:i], subs[i+1:]...)
            close(ch)
            return
        }
    }
}

// Push: called by NATS subscriber when event arrives
func (b *EventBroker) Push(userID string, evt NotificationEvent) {
    b.mu.RLock()
    defer b.mu.RUnlock()
    for _, ch := range b.subscribers[userID] {
        select {
        case ch <- evt:
        default: // drop if channel full (non-blocking)
        }
    }
}

// PushAll: broadcast to all connected users
func (b *EventBroker) PushAll(evt NotificationEvent) {
    b.mu.RLock()
    defer b.mu.RUnlock()
    for _, channels := range b.subscribers {
        for _, ch := range channels {
            select { case ch <- evt: default: }
        }
    }
}
```

**NATS → SSE Bridge:**
```go
// services/notification-service/internal/adapter/nats/subscriber.go

// Existing NATS subscribers đã có — chỉ cần thêm broker.Push call

func (s *NATSSubscriber) onKEVNew(msg *nats.Msg) {
    var event KEVNewEvent
    json.Unmarshal(msg.Data, &event)
    
    // Existing: store in alerts table + dispatch channels
    notif := NotificationEvent{
        Type:       "kev.new",
        Title:      fmt.Sprintf("New KEV: %s %s (%s)", event.Vendor, event.Product, event.CveID),
        Message:    "CVE added to CISA KEV catalog",
        Severity:   "Critical",
        EntityType: "cve",
        EntityID:   event.CveID,
    }
    s.alertStore.Store(context.Background(), notif)
    
    // NEW: push to all SSE-connected clients (broadcast)
    s.broker.PushAll(notif) // [NEW LINE]
}

func (s *NATSSubscriber) onSLABreached(msg *nats.Msg) {
    var event SLABreachedEvent
    json.Unmarshal(msg.Data, &event)
    
    notif := NotificationEvent{Type: "finding.sla.breached", ...}
    s.alertStore.Store(context.Background(), notif)
    
    // NEW: push to finding's product owners
    s.broker.Push(event.AssignedUserID, notif) // [NEW LINE]
}
```

---

### 2.6 Notification REST API (stored alerts)

```go
// services/notification-service/internal/adapter/http/alerts_handler.go

// GET /api/v2/notification-alerts  (existing path — expose new path)
// Expose new path: GET /api/v1/notifications
func (h *Handler) ListNotifications(w http.ResponseWriter, r *http.Request) {
    userID   := r.Header.Get("X-User-ID")
    isRead   := r.URL.Query().Get("is_read")
    page, ps := parsePagination(r)
    
    alerts, total, unread, _ := h.alertRepo.ListByUser(r.Context(), userID, isRead, page, ps)
    
    respondJSON(w, 200, map[string]interface{}{
        "notifications": mapAlerts(alerts),
        "total":         total,
        "unread_count":  unread,
        "page": page, "page_size": ps,
    })
}

func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    userID := r.Header.Get("X-User-ID")
    
    if err := h.alertRepo.MarkRead(r.Context(), id, userID); err != nil {
        respondError(w, 404, "NOT_FOUND", "Notification not found")
        return
    }
    respondJSON(w, 200, map[string]interface{}{"id": id, "is_read": true})
}

func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    count, _ := h.alertRepo.MarkAllRead(r.Context(), userID)
    respondJSON(w, 200, map[string]interface{}{"marked_count": count})
}

func (h *Handler) UnreadCount(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    count, _ := h.alertRepo.CountUnread(r.Context(), userID)
    respondJSON(w, 200, map[string]interface{}{"unread_count": count})
}
```

---

### 2.7 New Routes — Gateway Registration

```go
// apps/osv/internal/gateway/router.go additions

// BFF Dashboard (handled in gateway process)
mux.Handle("GET /api/v1/dashboard",     am.Authenticate(bff.HandleDashboard))
mux.Handle("GET /api/v1/dashboard/sla", am.Authenticate(bff.HandleDashboardSLA))

// Notifications SSE + REST (notification-service:8087)
mux.Handle("GET /api/v1/notifications/stream",     am.Authenticate(p.ForwardSSE("notification-service:8087")))
mux.Handle("GET /api/v1/notifications",            am.Authenticate(p.Forward("notification-service:8087")))
mux.Handle("PATCH /api/v1/notifications/",         am.Authenticate(p.Forward("notification-service:8087")))
mux.Handle("POST /api/v1/notifications/mark-all-read", am.Authenticate(p.Forward("notification-service:8087")))
mux.Handle("GET /api/v1/notifications/unread-count",   am.Authenticate(p.Forward("notification-service:8087")))
```

**SSE Forward proxy (buffering disabled):**
```go
// apps/osv/internal/gateway/proxy.go

func (p *ReverseProxy) ForwardSSE(target string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Accel-Buffering", "no") // nginx: disable buffering
        
        upstreamURL := "http://" + target + r.URL.RequestURI()
        req, _ := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, r.Body)
        req.Header = r.Header.Clone()
        
        // Use transport with no response body timeout
        transport := &http.Transport{
            ResponseHeaderTimeout: 5 * time.Second,
            // No idle timeout: SSE connections are long-lived
        }
        client := &http.Client{Transport: transport, Timeout: 0} // no timeout
        
        resp, err := client.Do(req)
        if err != nil {
            http.Error(w, "upstream unavailable", 503)
            return
        }
        defer resp.Body.Close()
        
        // Copy headers
        for k, vv := range resp.Header {
            for _, v := range vv { w.Header().Add(k, v) }
        }
        w.WriteHeader(resp.StatusCode)
        
        // Stream body (flush immediately)
        flusher := w.(http.Flusher)
        buf := make([]byte, 1024)
        for {
            n, err := resp.Body.Read(buf)
            if n > 0 {
                w.Write(buf[:n])
                flusher.Flush()
            }
            if err != nil { return }
        }
    }
}
```

---

### 2.8 SLA Dashboard Endpoint (sla-service)

```go
// services/sla-service/internal/adapter/http/handler.go — new endpoint

func (h *Handler) GetSLADashboard(w http.ResponseWriter, r *http.Request) {
    productID := r.URL.Query().Get("product_id")
    page, ps   := parsePagination(r)
    
    // Aggregate SLA data
    summary, _ := h.slaUC.GetComplianceSummary(r.Context(), productID)
    trend, _   := h.slaUC.GetComplianceTrend(r.Context(), 6) // last 6 months
    breached, _ := h.slaUC.GetBreachedFindings(r.Context(), productID, page, ps)
    atRisk, _  := h.slaUC.GetAtRiskFindings(r.Context(), productID, page, ps)
    byProduct, _ := h.slaUC.GetByProduct(r.Context())
    
    respondJSON(w, 200, map[string]interface{}{
        "summary":          summary,
        "compliance_trend": trend,
        "breached_findings": breached,
        "at_risk_findings": atRisk,
        "by_product":       byProduct,
        "total_breached":   summary.Breached,
        "total_at_risk":    summary.AtRisk,
        "page": page, "page_size": ps,
    })
}
```

**SLA Compliance Trend query:**
```sql
-- sla-service monthly compliance trend
SELECT 
    DATE_TRUNC('month', checked_at) AS month,
    AVG(compliance_pct) AS compliance_percent
FROM sla_snapshots
WHERE checked_at >= NOW() - INTERVAL '6 months'
  AND (product_id = $1 OR $1 IS NULL)
GROUP BY month
ORDER BY month ASC;
```

---

## 3. File Structure (New Files)

```
apps/osv/internal/gateway/
├── bff/
│   ├── dashboard.go      [NEW] DashboardBFF handler (fan-out logic)
│   ├── clients.go        [NEW] Internal HTTP clients to services
│   └── types.go          [NEW] KPIData, TrendPoint, etc.
└── proxy.go              [MODIFY] add ForwardSSE method

services/finding-service/internal/
└── adapter/http/
    └── internal_handler.go [NEW] /internal/* endpoints for BFF

services/notification-service/internal/
├── broker/
│   └── event_broker.go   [NEW] In-memory SSE broker
├── adapter/http/
│   ├── sse_handler.go     [NEW] StreamNotifications handler
│   └── alerts_handler.go  [MODIFY] add ListNotifications, MarkRead, MarkAllRead
└── adapter/nats/
    └── subscriber.go      [MODIFY] add broker.Push calls
```

---

## 4. Performance & Caching Strategy

| Data | Cache TTL | Source |
|------|-----------|--------|
| Dashboard KPIs | 60s Redis | finding-service aggregate |
| Risk trend | 5min Redis | PostgreSQL monthly GROUP BY |
| Product grades | 5min Redis | finding-service ComputeGrade |
| KEV alerts | 30min Redis | data-service (rarely changes) |
| Recent scans | 30s Redis | scan-service |
| SLA breaches | 60s Redis | sla-service |

**Performance target:** `GET /api/v1/dashboard` < 500ms P95 với Redis cache HIT < 50ms.

---

## 5. Acceptance Criteria

| Test | Expected |
|------|----------|
| `GET /api/v1/dashboard` | 200, tất cả 7 top-level keys có data |
| `GET /api/v1/dashboard?period=90d` | 200, `risk_trend` có 90 ngày data |
| `GET /api/v1/dashboard` 2nd call | response time < 50ms (Redis HIT) |
| `GET /api/v1/notifications/stream` | 200 `text/event-stream`, connection established |
| SSE ping event | received every 30s |
| `kev.new` NATS event | SSE event pushed đến browser < 2s |
| `GET /api/v1/notifications` | 200 với `unread_count` |
| `PATCH /api/v1/notifications/{id}/read` | 200 `{"is_read":true}` |
| `POST /api/v1/notifications/mark-all-read` | 200 `{"marked_count":N}` |
