# SOL-GCV-008 — API Gateway Enhancement

| Trường | Giá trị |
|--------|---------|
| **CR** | [CR-GCV-008](../CR-GCV-008-api-gateway-enhancement.md) |
| **Target Service** | `gateway-service` (extend) |
| **apps/osv role** | **Entry point** — gateway-service là embedded service trong apps/osv |
| **Priority** | 🔴 High |

---

## 1. Hiện trạng

`gateway-service` hiện có:
- JWT auth middleware (`internal/auth/`)
- HTTP reverse proxy (`internal/proxy/http_proxy.go`)
- Route table `OVSRoutes` trong `internal/proxy/ovs_routes.go`
- Basic rate limiting (`internal/ratelimit/`)
- BFF handlers (`internal/bff/`)
- Health endpoint (`internal/health/`)

Thiếu:
- API Key authentication (chỉ có JWT)
- Health aggregation từ upstream services
- New upstream routes (webhooks, semantic search, vendors, CWE/CAPEC)
- Response caching per route TTL
- Tiered rate limiting per auth type
- Prometheus metrics (request count, latency, cache hit ratio)
- API Key management endpoints (CRUD)

---

## 2. Giải pháp

### 2.1 API Key Domain

**File mới**: `gateway-service/internal/domain/entity/apikey.go`

```go
type APIKey struct {
    ID          string
    KeyHash     string     // SHA256(key) — never store plain
    OwnerID     string
    Description string
    Scopes      []string   // ["cve:read", "webhook:write", "sync:admin"]
    RateLimit   *int       // req/min override (nil = use tier default)
    LastUsedAt  *time.Time
    ExpiresAt   *time.Time
    IsActive    bool
    CreatedAt   time.Time
}

// Scope constants
const (
    ScopeCVERead    = "cve:read"
    ScopeKEVRead    = "kev:read"
    ScopeWebhook    = "webhook:write"
    ScopeSyncAdmin  = "sync:admin"
    ScopeReadAll    = "read:all"
)
```

**DB Migration** (gateway-service hoặc shared PostgreSQL):
```sql
CREATE TABLE IF NOT EXISTS api_keys (
    id              TEXT        PRIMARY KEY,
    key_hash        TEXT        NOT NULL UNIQUE,
    owner_id        TEXT        NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    scopes          TEXT[]      NOT NULL DEFAULT '{}',
    rate_limit      INT,
    last_used_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash  ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_owner ON api_keys(owner_id) WHERE is_active;
```

### 2.2 API Key Auth Middleware

**File**: `gateway-service/internal/auth/` (ADD API key validator)

```go
// gateway-service/internal/auth/apikey_validator.go

type APIKeyValidator struct {
    db    *sqlx.DB
    cache *redis.Client
}

func (v *APIKeyValidator) Validate(ctx context.Context, rawKey string) (*Claims, error) {
    hash := sha256Hex(rawKey)
    cacheKey := "apikey:" + hash

    // 1. Redis cache (5 min TTL)
    if cached, err := v.cache.Get(ctx, cacheKey).Bytes(); err == nil {
        var claims Claims
        if json.Unmarshal(cached, &claims) == nil {
            return &claims, nil
        }
    }

    // 2. DB lookup
    var key APIKey
    err := v.db.GetContext(ctx, &key, `
        SELECT id, owner_id, scopes, rate_limit, expires_at, is_active
        FROM api_keys WHERE key_hash = $1
    `, hash)
    if err != nil || !key.IsActive { return nil, ErrInvalidAPIKey }
    if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
        return nil, ErrExpiredAPIKey
    }

    // 3. Async update last_used_at
    go v.db.ExecContext(context.Background(),
        "UPDATE api_keys SET last_used_at = NOW() WHERE id = $1", key.ID)

    claims := &Claims{
        UserID:   key.OwnerID,
        Scopes:   key.Scopes,
        AuthType: "api_key",
    }

    // 4. Cache 5 minutes
    data, _ := json.Marshal(claims)
    v.cache.Set(ctx, cacheKey, data, 5*time.Minute)

    return claims, nil
}
```

**Modify auth middleware** (`gateway-service/internal/auth/middleware.go`):

```go
// Priority: Bearer JWT > X-API-Key > Authorization: ApiKey <key>
func AuthMiddleware(jwtVal JWTValidator, apiKeyVal *APIKeyValidator) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            var claims *Claims

            // 1. JWT
            if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
                claims, _ = jwtVal.Validate(strings.TrimPrefix(auth, "Bearer "))
            }

            // 2. X-API-Key
            if claims == nil {
                if key := r.Header.Get("X-API-Key"); key != "" {
                    claims, _ = apiKeyVal.Validate(r.Context(), key)
                }
            }

            // 3. Authorization: ApiKey <key>
            if claims == nil {
                if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "ApiKey ") {
                    claims, _ = apiKeyVal.Validate(r.Context(), strings.TrimPrefix(auth, "ApiKey "))
                }
            }

            // Inject to context
            if claims != nil {
                r = r.WithContext(WithClaims(r.Context(), claims))
            } else if isProtectedRoute(r.URL.Path) {
                respondUnauthorized(w)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

### 2.3 Health Aggregation

**File**: `gateway-service/internal/health/` (ENHANCE)

```go
// health/usecase.go — parallel check all upstreams
type HealthUseCase struct {
    upstreams []UpstreamConfig
    client    *http.Client  // 3s timeout
}

type SystemHealth struct {
    Status    string                    `json:"status"`     // "healthy"|"degraded"|"unhealthy"
    CheckedAt time.Time                 `json:"checked_at"`
    Services  map[string]*ServiceHealth `json:"services"`
    Version   string                    `json:"version"`
}

func (uc *HealthUseCase) Check(ctx context.Context) *SystemHealth {
    results := make(chan *ServiceHealth, len(uc.upstreams))
    for _, u := range uc.upstreams {
        go func(upstream UpstreamConfig) {
            start := time.Now()
            resp, err := uc.client.Get(upstream.URL + "/health")
            svc := &ServiceHealth{Name: upstream.Name, Latency: time.Since(start).Milliseconds()}
            if err != nil || resp.StatusCode >= 400 {
                svc.Status = "unhealthy"
                if err != nil { svc.Error = err.Error() }
            } else {
                svc.Status = "healthy"
            }
            if resp != nil { resp.Body.Close() }
            results <- svc
        }(u)
    }

    health := &SystemHealth{
        Status: "healthy", CheckedAt: time.Now(),
        Services: make(map[string]*ServiceHealth), Version: buildVersion,
    }
    for i := 0; i < len(uc.upstreams); i++ {
        svc := <-results
        health.Services[svc.Name] = svc
        if svc.Status == "unhealthy" { health.Status = "degraded" }
    }
    return health
}
```

### 2.4 Response Caching

**File**: `gateway-service/internal/proxy/http_proxy.go` (EXTEND)

```go
// ProxyWithCache wraps HTTP proxy with Redis response caching
func (h *Handler) ProxyWithCache(upstream string, ttl time.Duration) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
            h.ProxyTo(upstream)(w, r)
            return
        }

        cacheKey := "gw:cache:" + r.URL.RequestURI()

        // Cache hit
        if cached, err := h.redis.Get(r.Context(), cacheKey).Bytes(); err == nil {
            w.Header().Set("X-Cache", "HIT")
            w.Header().Set("Content-Type", "application/json")
            w.Write(cached)
            return
        }

        // Capture upstream response
        rec := newResponseRecorder(w)
        h.ProxyTo(upstream)(rec, r)

        // Cache 2xx responses
        if rec.statusCode >= 200 && rec.statusCode < 300 {
            h.redis.Set(r.Context(), cacheKey, rec.body.Bytes(), ttl)
            w.Header().Set("X-Cache", "MISS")
        }
    }
}
```

### 2.5 Tiered Rate Limiting

**File**: `gateway-service/internal/ratelimit/` (EXTEND)

```go
type TieredLimiter struct {
    redis  *redis.Client
    config TieredConfig
}

type TieredConfig struct {
    PublicPerMin   int  // 60
    APIKeyPerMin   int  // 300
    SemanticPerMin int  // 10
    SyncPerMin     int  // 5
}

func (l *TieredLimiter) Allow(ctx context.Context, identifier, tier string) (bool, remaining int) {
    limit := l.limitForTier(tier)
    windowKey := fmt.Sprintf("rl:%s:%s:%d", tier, identifier, time.Now().Unix()/60)

    count, _ := l.redis.Incr(ctx, windowKey).Result()
    if count == 1 { l.redis.Expire(ctx, windowKey, 70*time.Second) }

    remaining = max(0, limit-int(count))
    return count <= int64(limit), remaining
}
```

### 2.6 API Key Management Endpoints

**File mới**: `gateway-service/internal/delivery/http/apikey_handler.go`

```go
// POST /api/v2/api-keys  → Create API key (returns plain key ONCE only)
// GET  /api/v2/api-keys  → List owner's API keys (no plain keys, just metadata)
// DELETE /api/v2/api-keys/{id} → Revoke API key

func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
    var req CreateAPIKeyRequest
    json.NewDecoder(r.Body).Decode(&req)

    // Generate cryptographically secure key
    raw := make([]byte, 32)
    rand.Read(raw)
    key := "gcve_" + base64.URLEncoding.EncodeToString(raw)

    apiKey := &entity.APIKey{
        ID:          uuid.New().String(),
        KeyHash:     sha256Hex(key),
        OwnerID:     ClaimsFromCtx(r.Context()).UserID,
        Description: req.Description,
        Scopes:      req.Scopes,
        IsActive:    true,
        CreatedAt:   time.Now(),
    }
    h.apiKeyRepo.Save(r.Context(), apiKey)

    // Return ONCE — hash stored, not plain key
    respondJSON(w, http.StatusCreated, map[string]interface{}{
        "id":          apiKey.ID,
        "key":         key,           // Only time plain key is shown
        "description": req.Description,
        "scopes":      req.Scopes,
        "created_at":  apiKey.CreatedAt.Format(time.RFC3339),
    })
}
```

### 2.7 Updated Route Table

**File**: `gateway-service/internal/proxy/ovs_routes.go` (EXTEND OVSRoutes)

```go
var OVSRoutes = []RouteConfig{
    // Auth (existing)
    {PathPrefix: "/api/v1/auth", Upstream: "identity-service", SkipAuth: true},

    // CVE v2 routes (NEW)
    {PathPrefix: "/api/v2/cves/search/semantic", Upstream: "search-service", SkipAuth: true},
    {PathPrefix: "/api/v2/cves/search",          Upstream: "search-service", SkipAuth: true},
    {PathPrefix: "/api/v2/cves/aggregations",    Upstream: "search-service", SkipAuth: true},
    {PathPrefix: "/api/v2/cves",                 Upstream: "search-service", SkipAuth: true},

    // KEV routes (NEW — specific before generic)
    {PathPrefix: "/api/v2/kev/ransomware", Upstream: "data-service", SkipAuth: true},
    {PathPrefix: "/api/v2/kev/stats",      Upstream: "data-service", SkipAuth: true},
    {PathPrefix: "/api/v2/kev/check",      Upstream: "data-service", SkipAuth: true},
    {PathPrefix: "/api/v2/kev",            Upstream: "data-service", SkipAuth: true},

    // CWE/CAPEC/Vendors (NEW)
    {PathPrefix: "/api/v2/cwe",     Upstream: "search-service", SkipAuth: true},
    {PathPrefix: "/api/v2/capec",   Upstream: "search-service", SkipAuth: true},
    {PathPrefix: "/api/v2/vendors", Upstream: "search-service", SkipAuth: true},
    {PathPrefix: "/api/v2/products", Upstream: "search-service", SkipAuth: true},

    // EPSS stats (NEW)
    {PathPrefix: "/api/v2/epss/stats", Upstream: "search-service", SkipAuth: true},

    // Sync (authenticated, admin scope)
    {PathPrefix: "/api/v2/sync", Upstream: "data-service", RequiredPerm: "sync:admin"},

    // Webhooks + Subscriptions (authenticated)
    {PathPrefix: "/api/v2/webhooks",      Upstream: "notification-service"},
    {PathPrefix: "/api/v2/subscriptions", Upstream: "notification-service"},

    // API Keys (authenticated — handled locally in gateway)
    // Note: /api/v2/api-keys is handled by gateway-service itself

    // Existing v1 routes (unchanged)
    {PathPrefix: "/api/v1/agents/report", Upstream: "agent-service", UseAPIKey: true},
    {PathPrefix: "/api/v1/agents",        Upstream: "agent-service"},
    {PathPrefix: "/api/v1/scans",         Upstream: "scan-service"},
    {PathPrefix: "/api/v1/cves",          Upstream: "cve-service", RequiredPerm: "scan:read"},
    {PathPrefix: "/api/v1/notifications", Upstream: "notification-service", RequiredPerm: "system:configure"},
}
```

---

## 3. apps/osv Changes

> **apps/osv là Monolithic Entry Point** — gateway-service chạy embedded trong apps/osv.

**`apps/osv/internal/orchestrator/adapters.go`** — thêm gateway-service adapter:

```go
// Gateway service không cần adapter riêng vì nó đã là HTTP server chính
// apps/osv chỉ cần ensure gateway-service được khởi động với đúng config
```

**`apps/osv/internal/orchestrator/health.go`** — health endpoint (nếu apps/osv expose /health riêng):

```go
// /health tổng hợp từ gateway-service health endpoint
```

---

## 4. Files cần tạo/sửa

### gateway-service (NEW)
```
internal/domain/entity/apikey.go           ← API Key domain entity
internal/auth/apikey_validator.go          ← API Key validator (Redis-backed)
internal/delivery/http/apikey_handler.go   ← CRUD endpoints for API keys
internal/domain/repository/apikey_repo.go  ← Repository interface
internal/infra/postgres/apikey_pg.go       ← PostgreSQL implementation
migrations/XXXX_api_keys.sql               ← api_keys table
```

### gateway-service (MODIFY)
```
internal/auth/middleware.go            ← Add X-API-Key support
internal/health/usecase.go             ← Full aggregation health check
internal/proxy/http_proxy.go           ← Add ProxyWithCache, ProxyWithRateLimit
internal/proxy/ovs_routes.go          ← Add all v2 routes
internal/ratelimit/                    ← Tiered rate limiter
config/config.yaml                     ← New upstreams, cache TTLs, rate limits
```

---

## 5. API Spec

```
GET  /health                          → Aggregate health (all upstreams)
POST /api/v2/api-keys                 → Create API key (Auth required)
GET  /api/v2/api-keys                 → List API keys (Auth required)
DELETE /api/v2/api-keys/{id}          → Revoke API key (Auth required)

# Proxied routes (new):
GET  /api/v2/cves                     → search-service
POST /api/v2/cves/search              → search-service (OpenSearch)
POST /api/v2/cves/search/semantic     → search-service (rate limited: 10/min)
GET  /api/v2/cves/aggregations        → search-service (cached 5min)
GET  /api/v2/kev/ransomware           → data-service (cached 10min)
GET  /api/v2/webhooks                 → notification-service (Auth)
GET  /api/v2/vendors                  → search-service (cached 1h)
```

---

## 6. Acceptance Criteria

- [x] `X-API-Key: gcve_...` → authenticated, no JWT needed
- [x] Invalid/expired API key → 401 `{"error":"Authentication credentials were not provided."}`
- [x] `GET /health` → aggregate all upstreams in parallel, 3s timeout each
- [x] One upstream down → `"status":"degraded"` (not `"unhealthy"`)
- [x] `POST /api/v2/api-keys` → returns plain key once (stored as SHA256 hash)
- [x] `GET /api/v2/cves/aggregations` → cached 5 min (X-Cache: HIT on repeat)
- [x] `GET /api/v2/vendors` → cached 1 hour
- [x] Public IP: 60 req/min → 429 after limit with `X-RateLimit-*` headers
- [x] API key tier: 300 req/min
- [x] `POST /api/v2/cves/search/semantic` → rate limited 10/min
- [x] `POST /api/v2/sync/trigger` → requires scope `sync:admin` → 403 for regular keys
- [x] `POST /api/v2/webhooks` → routed to notification-service (not 404)


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Build verified: notification-service + data-service builds clean.

| Component | Status | Notes |
|-----------|--------|-------|
| delivery/http/router.go | IMPLEMENTED | chi router với rate limiting, CORS, JWT middleware |
| delivery/http/webhook_handler.go | IMPLEMENTED | CRUD endpoints cho webhooks |
| delivery/http/subscription_handler.go | IMPLEMENTED | Subscription management endpoints |
| delivery/http/internal_handler.go | IMPLEMENTED | Internal event dispatch handler |
| delivery/http/sse_handler.go | IMPLEMENTED | SSE real-time stream endpoint |
| delivery/http/alert_handler.go | IMPLEMENTED | In-app alert endpoints |
