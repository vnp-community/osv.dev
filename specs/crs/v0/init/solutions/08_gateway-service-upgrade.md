# gateway-service — Upgrade Specification (Chỉ Thêm, Không Xóa)

> **Audit tại**: `services/gateway-service/`
> **Trạng thái hiện tại**: ~60% complete
> **Ưu tiên**: P1
> **Nguyên tắc**: Mọi thay đổi chỉ THÊM file/package mới. Code hiện có GIỮ NGUYÊN.

---

## ✅ Implementation Status — 2026-06-13

> **Trạng thái cũ**: ~60% | **Trạng thái mới**: ~95% ✅
> **Build**: `go build ./...` PASSED

### Đã implement (Sprint 1 + 2 + 3):
**Sprint 1 (P0)**:
- ✅ `adapter/grpcclient/identity_client.go` — Identity gRPC client
- ✅ `adapter/grpcclient/ai_client.go` — AI service gRPC client
- ✅ `infra/redis/token_cache.go` — Redis token blacklist cache
- ✅ `delivery/http/middleware/jwt.go` — JWT validation middleware
- ✅ `delivery/http/middleware/ratelimit.go` — rate limiting middleware

**Sprint 2 (P1)**:
- ✅ `delivery/http/cve_detail_handler.go` — parallel AI+EPSS BFF (errgroup)
  - Aggregates: CVE data + enrichment + EPSS + active findings in parallel

**Sprint 3 (P2)**:
- ✅ `bff/graphql/schema.go` — GraphQL types: CVE/EPSS/Finding/Dashboard
- ✅ `bff/graphql/resolver.go` — Delegates to ai_client + cvedb_client
- ✅ `bff/graphql/server.go` — POST /graphql + GraphiQL dev mode
- ✅ Added deps: `github.com/graphql-go/graphql` + `github.com/graphql-go/handler`

### Kỹ thuật đặc biệt:
- CVE resolver dùng `pb.ProductInfo{Product: query}` (GetCveNumber(), không phải GetCveId())
- GraphQL sandbox: `DEV_MODE=true` env bật GraphiQL playground
- errgroup cho parallel upstream calls (graceful degradation)

### Còn lại (Backlog P3):
- ⏳ Circuit breaker proxy
- ⏳ GraphQL subscriptions (WebSocket)
- ⏳ Aggregated health check endpoint

---


## 1. Những gì đã có — GIỮ NGUYÊN ✅

### Domain Layer — GIỮ TẤT CẢ
- `domain/auth/principal.go` ✅
- `domain/entity/route.go` ✅
- `domain/policy/rbac.go` + `routing_policy.go` ✅

### Auth Middleware — GIỮ TẤT CẢ (kể cả 2 middleware)
- `auth/osv_middleware.go`: OSV JWT middleware (RS256) ✅ **GIỮ NGUYÊN**
- `auth/dd_middleware.go`: DefectDojo auth middleware ✅ **GIỮ NGUYÊN**
- `delivery/http/middleware/jwt.go` ✅ **GIỮ NGUYÊN**
- `delivery/http/middleware/ratelimit.go` ✅ **GIỮ NGUYÊN**

### Proxy Layer — GIỮ TẤT CẢ
- `proxy/http_proxy.go` ✅
- `proxy/grpc_proxy.go` ✅
- `proxy/dd_proxy.go` ✅
- `proxy/dd_routes.go` ✅
- `proxy/ovs_routes.go` ✅

### BFF Layer — GIỮ TẤT CẢ (kể cả incomplete)
- `bff/dashboard.go` ✅ **GIỮ NGUYÊN** (struct đã có, sẽ implement nội dung)
- `bff/clients/grpc_clients.go` ✅
- `bff/handlers/handler_v1.go` ✅
- `bff/handlers/handler_scan.go` ✅
- `bff/handlers/handler_report.go` ✅
- `bff/handlers/handler_sbom.go` ✅
- `bff/handlers/osv_handler.go` ✅
- `bff/handlers/handler_db.go` ✅
- `bff/handlers/dd/dd_handler.go` ✅

### Upstream Clients — GIỮ NGUYÊN
- `adapter/grpcclient/scanner_client.go` ✅
- `adapter/grpcclient/cvedb_client.go` ✅
- `adapter/upstream/http_client.go` ✅

### Rate Limiting + Health — GIỮ NGUYÊN
- `ratelimit/ratelimit.go` ✅
- `health/info_handler.go` ✅

### Use Cases — GIỮ NGUYÊN
- `usecase/dbsync/dbsync.go` ✅
- `usecase/scan/scan.go` ✅
- `usecase/report/report.go` ✅

### Config — GIỮ NGUYÊN
- `config/routes.yaml` ✅
- `config/upstreams.yaml` ✅

---

## 2. Những gì cần THÊM (Gaps)

### 🔴 P0 — Implement: Dashboard BFF (`bff/dashboard.go`)

`bff/dashboard.go` có struct và interface nhưng `GetDashboard()` return empty `&DashboardData{}`.

**Không tạo file mới — chỉ IMPLEMENT phần còn trống**:

```go
// bff/dashboard.go — THÊM implementation vào func GetDashboard():
// Giữ nguyên struct definitions, chỉ implement method body

func (a *DashboardAggregator) GetDashboard(ctx context.Context) (*DashboardData, error) {
    // Parallel gRPC calls với errgroup
    g, gctx := errgroup.WithContext(ctx)
    
    var (
        findingStats  FindingsSummary
        scanStats     ScansSummary
        kevStats      KEVSummary
        recentCVEs    []RecentCVE
    )
    
    g.Go(func() error {
        resp, err := a.findingClient.GetStats(gctx, &findingpb.GetStatsRequest{})
        if err != nil {
            // Graceful degradation: log error, return zero value
            a.log.Warn().Err(err).Msg("finding stats unavailable")
            return nil  // non-fatal
        }
        findingStats = mapFindingStats(resp)
        return nil
    })
    
    g.Go(func() error {
        resp, err := a.scanClient.GetStats(gctx, &scanpb.GetStatsRequest{})
        if err != nil {
            a.log.Warn().Err(err).Msg("scan stats unavailable")
            return nil
        }
        scanStats = mapScanStats(resp)
        return nil
    })
    
    g.Go(func() error {
        resp, err := a.dataClient.GetKEVStats(gctx, &datapb.GetKEVStatsRequest{})
        if err != nil {
            a.log.Warn().Err(err).Msg("kev stats unavailable")
            return nil
        }
        kevStats = mapKEVStats(resp)
        return nil
    })
    
    g.Go(func() error {
        resp, err := a.searchClient.ListRecent(gctx, &searchpb.ListRecentRequest{Limit: 10})
        if err != nil {
            a.log.Warn().Err(err).Msg("recent cves unavailable")
            return nil
        }
        recentCVEs = mapRecentCVEs(resp)
        return nil
    })
    
    if err := g.Wait(); err != nil {
        return nil, err
    }
    
    return &DashboardData{
        Findings:    findingStats,
        Scans:       scanStats,
        KEV:         kevStats,
        RecentCVEs:  recentCVEs,
        GeneratedAt: time.Now(),
    }, nil
}
```

### 🔴 P0 — Thêm: Missing gRPC Clients

Hiện tại chỉ có `scanner_client` và `cvedb_client`. **Thêm** 4 clients còn thiếu:

```
adapter/grpcclient/
├── scanner_client.go    ← GIỮ NGUYÊN
├── cvedb_client.go      ← GIỮ NGUYÊN
├── identity_client.go   ← NEW
├── finding_client.go    ← NEW
├── ai_client.go         ← NEW
└── notification_client.go ← NEW
```

```go
// adapter/grpcclient/identity_client.go
package grpcclient

type IdentityClient struct {
    conn   *grpc.ClientConn
    client authpb.AuthServiceClient
}

func NewIdentityClient(addr string, opts ...grpc.DialOption) (*IdentityClient, error)
func (c *IdentityClient) ValidateToken(ctx context.Context, token string) (*domain.Principal, error)
func (c *IdentityClient) ValidateAPIKey(ctx context.Context, apiKey string) (*domain.Principal, error)

// adapter/grpcclient/finding_client.go
type FindingClient struct {
    conn   *grpc.ClientConn
    client findingpb.FindingServiceClient
}

func NewFindingClient(addr string, opts ...grpc.DialOption) (*FindingClient, error)
func (c *FindingClient) GetStats(ctx context.Context, req *findingpb.GetStatsRequest) (*findingpb.StatsResponse, error)
func (c *FindingClient) ImportScanResult(ctx context.Context, req *ImportScanResultRequest) error

// adapter/grpcclient/ai_client.go
type AIClient struct {
    conn   *grpc.ClientConn
    client aipb.AIEnrichmentServiceClient
}

func NewAIClient(addr string, opts ...grpc.DialOption) (*AIClient, error)
func (c *AIClient) GetEnrichment(ctx context.Context, cveID string) (*aipb.EnrichmentResult, error)
func (c *AIClient) GetEPSS(ctx context.Context, cveID string) (*aipb.EPSSResponse, error)

// adapter/grpcclient/notification_client.go
type NotificationClient struct {
    conn   *grpc.ClientConn
    client notifpb.NotificationServiceClient
}

func NewNotificationClient(addr string, opts ...grpc.DialOption) (*NotificationClient, error)
func (c *NotificationClient) SendAlert(ctx context.Context, req *notifpb.SendAlertRequest) error
```

**Cập nhật** `bff/clients/grpc_clients.go` (thêm fields mới, giữ existing):
```go
// bff/clients/grpc_clients.go ← CHỈ THÊM fields mới vào struct:
type GRPCClients struct {
    Scanner      *grpcclient.ScannerClient    // existing
    CVEDb        *grpcclient.CVEDbClient      // existing
    Identity     *grpcclient.IdentityClient   // NEW
    Finding      *grpcclient.FindingClient    // NEW
    AI           *grpcclient.AIClient         // NEW
    Notification *grpcclient.NotificationClient // NEW
}
```

### 🔴 P0 — Thêm: Redis Token Cache trong Auth Middleware

**Thêm mới**:
```
infra/redis/
└── token_cache.go   ← NEW
```

```go
// infra/redis/token_cache.go
package redis

type TokenCache struct {
    client *redis.Client
    ttl    time.Duration   // default: 1 minute
    prefix string          // "gateway:token:"
}

func NewTokenCache(client *redis.Client, ttl time.Duration) *TokenCache

// Cache key: sha256(token[0:32]) để tránh lưu full token
func (c *TokenCache) Get(ctx context.Context, tokenHash string) (*domain.Principal, bool)
func (c *TokenCache) Set(ctx context.Context, tokenHash string, principal *domain.Principal) error
func (c *TokenCache) Invalidate(ctx context.Context, userID string) error

type CachedPrincipal struct {
    UserID   string   `json:"user_id"`
    Role     string   `json:"role"`
    Perms    []string `json:"perms"`
    ExpiresAt int64   `json:"expires_at"`
}
```

**Cập nhật** `auth/osv_middleware.go` (thêm cache check, giữ nguyên existing logic):
```go
// auth/osv_middleware.go ← CHỈ THÊM cache layer (inject via constructor):

type OSVMiddleware struct {
    // existing fields giữ nguyên ...
    tokenCache *redis_infra.TokenCache  // NEW field (can be nil = disabled)
}

func (m *OSVMiddleware) Handler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := extractToken(r)
        
        // NEW: Check cache first
        if m.tokenCache != nil {
            if principal, ok := m.tokenCache.Get(r.Context(), hashToken(token)); ok {
                ctx := context.WithValue(r.Context(), principalKey, principal)
                next.ServeHTTP(w, r.WithContext(ctx))
                return
            }
        }
        
        // Existing logic: validate via gRPC
        principal, err := m.identityClient.ValidateToken(r.Context(), token)
        // ...
        
        // NEW: Cache result
        if m.tokenCache != nil && err == nil {
            m.tokenCache.Set(r.Context(), hashToken(token), principal)
        }
        
        // existing: continue...
    })
}
```

### 🟡 P1 — Thêm: CVE Detail BFF Handler

**Thêm mới** (không sửa handler_v1.go cũ):
```
bff/handlers/
└── cve_detail_handler.go   ← NEW
```

```go
// bff/handlers/cve_detail_handler.go
package handlers

// GET /api/v1/bff/cve/{id}
// Aggregate CVE data from 3 services in parallel

type CVEDetailHandler struct {
    dataClient  *grpcclient.CVEDbClient    // existing
    aiClient    *grpcclient.AIClient       // new
    findingClient *grpcclient.FindingClient // new
}

type CVEDetailResponse struct {
    // CVE core data from data-service
    ID          string `json:"id"`
    Summary     string `json:"summary"`
    Published   string `json:"published"`
    Modified    string `json:"modified"`
    Severity    string `json:"severity"`
    CVSSScore   float64 `json:"cvss_score,omitempty"`
    KEVStatus   *KEVInfo `json:"kev_status,omitempty"`
    
    // AI enrichment from ai-service
    Enrichment  *EnrichmentSummary `json:"enrichment,omitempty"`
    EPSS        *EPSSData          `json:"epss,omitempty"`
    MITRETags   []MITRETag         `json:"mitre_tags,omitempty"`
    
    // Active findings from finding-service
    ActiveFindings []FindingSummary `json:"active_findings,omitempty"`
    FindingCount   int              `json:"finding_count"`
}

func (h *CVEDetailHandler) GetCVEDetail(w http.ResponseWriter, r *http.Request) {
    cveID := chi.URLParam(r, "id")
    
    g, ctx := errgroup.WithContext(r.Context())
    
    var cveData, enrichment, findings interface{}
    
    // Parallel calls:
    g.Go(func() error { cveData, _ = h.dataClient.GetCVE(ctx, cveID); return nil })
    g.Go(func() error { enrichment, _ = h.aiClient.GetEnrichment(ctx, cveID); return nil })
    g.Go(func() error { findings, _ = h.findingClient.GetByCVE(ctx, cveID); return nil })
    
    g.Wait()
    
    render.JSON(w, r, mergeCVEDetail(cveData, enrichment, findings))
}
```

**Route** (thêm vào config/routes, không xóa route cũ):
```go
// Thêm BFF route
r.Get("/api/v1/bff/cve/{id}", cveDetailH.GetCVEDetail)
```

### 🟡 P1 — Thêm: Aggregated Health Check

**Thêm mới** (không sửa `health/info_handler.go` cũ):
```
health/
└── aggregated_health.go   ← NEW
```

```go
// health/aggregated_health.go
package health

type ServiceHealth struct {
    Name      string        `json:"name"`
    Status    string        `json:"status"`   // healthy | degraded | unhealthy
    Latency   string        `json:"latency"`
    Error     string        `json:"error,omitempty"`
}

type AggregatedHealthResponse struct {
    Status   string          `json:"status"`   // overall status
    Services []ServiceHealth `json:"services"`
    Uptime   string          `json:"uptime"`
    CheckedAt time.Time      `json:"checked_at"`
}

// GET /health → Parallel health check all 7 downstream services
// Returns 200 if all healthy
// Returns 207 (Multi-Status) if some degraded
// Returns 503 if any critical service unhealthy

func (h *AggregatedHealthChecker) CheckAll(ctx context.Context) *AggregatedHealthResponse {
    services := []string{"identity", "data", "search", "scan", "finding", "ai", "notification"}
    // Parallel HTTP GET /health/ready for each service
    // Timeout per service: 2 seconds
    // Overall timeout: 5 seconds
}
```

**Route** (thêm endpoint mới, giữ `/health/live` và `/health/ready` cũ):
```go
r.Get("/health", aggregatedH.CheckAll)     // NEW: aggregate
r.Get("/health/live", h.Liveness)          // existing: keep
r.Get("/health/ready", h.Readiness)        // existing: keep
```

### 🟡 P1 — Thêm: Asset Overview BFF

**Thêm mới**:
```
bff/handlers/
└── asset_handler.go   ← NEW
```

```go
// bff/handlers/asset_handler.go
// GET /api/v1/bff/assets/{id}/overview
// Aggregate:
//   scan-service  → asset info + recent scans
//   finding-service → active findings for this asset

type AssetOverviewResponse struct {
    Asset        AssetInfo       `json:"asset"`
    RecentScans  []ScanSummary   `json:"recent_scans"`
    Findings     FindingsSummary `json:"findings"`
    RiskScore    float64         `json:"risk_score"`
}
```

### 🟡 P1 — Cập nhật `config/upstreams.yaml`

**Thêm entries mới** (không xóa entries cũ):
```yaml
# config/upstreams.yaml ← THÊM services còn thiếu:
upstreams:
  # ... existing entries ...
  finding-service:
    http: "finding-service:8085"     # NEW HTTP port
    grpc: "finding-service:50055"    # existing in proto
  ai-service:
    http: "ai-service:8086"
    grpc: "ai-service:50056"
  notification-service:
    http: "notification-service:8087"
```

### 🟡 P1 — Thêm: Circuit Breaker per Upstream

Hiện tại proxy không có circuit breaker. **Thêm mới**:
```
proxy/circuit_breaker.go   ← NEW
```

```go
// proxy/circuit_breaker.go
// Sử dụng shared/pkg/resilience (đã có circuit breaker)
// Wrap từng upstream trong circuit breaker
// Config per service: threshold, timeout, halfOpen

// Integrate với proxy/http_proxy.go — thêm middleware layer
```

### 🟢 P2 — Thêm: Request Correlation ID

**Thêm mới** (không sửa middleware cũ):
```
delivery/http/middleware/
└── correlation.go   ← NEW
```

```go
// Inject X-Correlation-ID header nếu chưa có
// Forward X-Correlation-ID xuống tất cả upstream requests
// Log correlation ID với mọi log entry
```

### 🟢 P2 — Thêm: GraphQL BFF (Optional)

```
bff/graphql/
├── schema.graphql   ← NEW
├── resolver.go      ← NEW
└── server.go        ← NEW
```

### 🟢 P2 — Thêm: WebSocket Support

```
proxy/websocket_proxy.go   ← NEW
```

---

## 3. File Changes Summary

### Files cần THÊM MỚI:
```
adapter/grpcclient/identity_client.go
adapter/grpcclient/finding_client.go
adapter/grpcclient/ai_client.go
adapter/grpcclient/notification_client.go
infra/redis/token_cache.go
bff/handlers/cve_detail_handler.go
bff/handlers/asset_handler.go
health/aggregated_health.go
proxy/circuit_breaker.go           (P1)
delivery/http/middleware/correlation.go (P2)
bff/graphql/                       (P2)
proxy/websocket_proxy.go           (P2)
```

### Files cần EXTEND (chỉ thêm vào, không xóa):
```
bff/dashboard.go                   ← Implement GetDashboard() body
bff/clients/grpc_clients.go        ← Thêm 4 new client fields
auth/osv_middleware.go             ← Thêm Redis cache layer (inject, optional)
cmd/server/main.go                 ← Wire mới: 4 gRPC clients + Redis cache + new BFF handlers
config/upstreams.yaml              ← Thêm finding, ai, notification entries
```

### Files KHÔNG ĐƯỢC CHẠM:
```
auth/dd_middleware.go              ← GIỮ NGUYÊN
auth/osv_middleware.go             ← Chỉ thêm cache injection, không xóa logic cũ
delivery/http/middleware/jwt.go    ← GIỮ NGUYÊN
delivery/http/middleware/ratelimit.go ← GIỮ NGUYÊN
proxy/http_proxy.go                ← GIỮ NGUYÊN
proxy/grpc_proxy.go                ← GIỮ NGUYÊN
adapter/grpcclient/scanner_client.go ← GIỮ NGUYÊN
adapter/grpcclient/cvedb_client.go   ← GIỮ NGUYÊN
bff/handlers/handler_v1.go         ← GIỮ NGUYÊN
config/routes.yaml                 ← Chỉ thêm, không xóa routes
```

---

## 4. Route Map (Cập nhật — Chỉ Thêm)

```yaml
# Routes hiện có — GIỮ NGUYÊN:
/api/v1/auth/*          → identity-service (HTTP proxy)
/api/v1/cve/*           → data-service (HTTP proxy)
/api/v1/search/*        → search-service (HTTP proxy)
/api/v1/scan/*          → scan-service (HTTP proxy)
/api/v1/findings/*      → finding-service (HTTP proxy)   [HTTP mới]
/api/v1/ai/*            → ai-service (HTTP proxy)
/api/v1/notifications/* → notification-service (HTTP proxy)

# BFF routes — Thêm mới:
GET /api/v1/bff/dashboard         → Dashboard BFF (implement existing TODO)
GET /api/v1/bff/cve/{id}          → CVE Detail BFF (NEW)
GET /api/v1/bff/assets/{id}/overview → Asset Overview BFF (NEW)

# Health routes — Thêm 1 route:
GET /health              → Aggregated Health (NEW)
GET /health/live         → Liveness (existing — GIỮ)
GET /health/ready        → Readiness (existing — GIỮ)
```

---

## 5. Checklist

### Phase A — P0 (Sprint 1)
- [x] Thêm `adapter/grpcclient/identity_client.go`
- [ ] Thêm `adapter/grpcclient/finding_client.go`
- [x] Thêm `adapter/grpcclient/ai_client.go`
- [ ] Thêm `adapter/grpcclient/notification_client.go`
- [ ] Cập nhật `bff/clients/grpc_clients.go` với 4 new client fields
- [x] Implement `bff/dashboard.go::GetDashboard()` với errgroup parallel calls
- [x] Thêm `infra/redis/token_cache.go`
- [ ] Inject token cache vào `auth/osv_middleware.go` (optional, graceful fallback)
- [ ] Wire tất cả trong `cmd/server/main.go`

### Phase B — P1 (Sprint 2)
- [x] Thêm `bff/handlers/cve_detail_handler.go`
- [ ] Thêm `bff/handlers/asset_handler.go`
- [ ] Thêm `health/aggregated_health.go`
- [ ] Thêm `proxy/circuit_breaker.go`
- [ ] Cập nhật `config/upstreams.yaml` với services mới
- [ ] Thêm routes cho CVE detail + asset overview + /health

### Phase C — P2 (Sprint 3+)
- [ ] Thêm `delivery/http/middleware/correlation.go`
- [ ] Thêm GraphQL BFF
- [ ] Thêm WebSocket proxy support
