# TASK-GCV-013 — Gateway Route Table v2

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-013 |
| **Service** | `gateway-service` |
| **CR** | CR-GCV-008 |
| **Phase** | 1 — Core Pipeline |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-GCV-011 |

## Context

Cập nhật `OVSRoutes` (hoặc tương đương) trong `gateway-service` để thêm tất cả routes v2 mới: semantic search, KEV ransomware, CWE/CAPEC, vendors, EPSS stats, webhooks, subscriptions. **Thứ tự routes quan trọng** — specific paths phải đứng trước generic prefix paths.

## Reference

- Solution: [SOL-GCV-008](../solutions/SOL-GCV-008-api-gateway-enhancement.md) §2.7
- CR: [CR-GCV-008](../CR-GCV-008-api-gateway-enhancement.md) §7

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/proxy/ovs_routes.go
```

**Đọc trước**: File `ovs_routes.go` và `osv_handler.go` để hiểu cấu trúc `RouteConfig` struct và cách routes được match.

## Implementation Spec

### ovs_routes.go — EXTEND OVSRoutes

Đọc cấu trúc `RouteConfig` hiện tại. Sau đó thêm các routes mới vào `OVSRoutes` slice theo đúng thứ tự ưu tiên:

```go
// Thứ tự quan trọng: specific trước generic!
// Thêm các routes mới (INSERT trước các routes generic hiện có):

var OVSRoutes = []RouteConfig{
    // ─── Auth (unchanged) ───────────────────────────────────────────────────
    {PathPrefix: "/api/v1/auth", Upstream: "identity-service", SkipAuth: true},

    // ─── CVE v2 routes (NEW — specific before generic) ──────────────────────
    {PathPrefix: "/api/v2/cves/search/semantic",  Upstream: "search-service",  SkipAuth: true},
    {PathPrefix: "/api/v2/cves/search",           Upstream: "search-service",  SkipAuth: true},
    {PathPrefix: "/api/v2/cves/aggregations",     Upstream: "search-service",  SkipAuth: true},
    {PathPrefix: "/api/v2/cves/export",           Upstream: "search-service",  SkipAuth: true},
    {PathPrefix: "/api/v2/cves",                  Upstream: "search-service",  SkipAuth: true},

    // ─── EPSS stats (NEW) ────────────────────────────────────────────────────
    {PathPrefix: "/api/v2/epss/stats",            Upstream: "search-service",  SkipAuth: true},

    // ─── KEV routes (NEW — specific before /api/v2/kev generic) ────────────
    {PathPrefix: "/api/v2/kev/ransomware",        Upstream: "data-service",    SkipAuth: true},
    {PathPrefix: "/api/v2/kev/stats",             Upstream: "data-service",    SkipAuth: true},
    {PathPrefix: "/api/v2/kev/check",             Upstream: "data-service",    SkipAuth: true},
    {PathPrefix: "/api/v2/kev",                   Upstream: "data-service",    SkipAuth: true},

    // ─── Taxonomy & Catalog (NEW) ────────────────────────────────────────────
    {PathPrefix: "/api/v2/cwe",                   Upstream: "search-service",  SkipAuth: true},
    {PathPrefix: "/api/v2/capec",                 Upstream: "search-service",  SkipAuth: true},
    {PathPrefix: "/api/v2/vendors",               Upstream: "search-service",  SkipAuth: true},
    {PathPrefix: "/api/v2/products",              Upstream: "search-service",  SkipAuth: true},

    // ─── Stats & Dashboard (NEW) ─────────────────────────────────────────────
    {PathPrefix: "/api/v2/stats/dashboard",       Upstream: "search-service",  SkipAuth: true},

    // ─── Admin / Sync (authenticated, admin scope) ───────────────────────────
    {PathPrefix: "/api/v2/sync",                  Upstream: "data-service",    RequiredPerm: "sync:admin"},

    // ─── Webhooks + Subscriptions (authenticated) ─────────────────────────────
    {PathPrefix: "/api/v2/webhooks",              Upstream: "notification-service"},
    {PathPrefix: "/api/v2/subscriptions",         Upstream: "notification-service"},

    // ─── API Keys (handled locally in gateway — NOT proxied) ─────────────────
    // Note: /api/v2/api-keys routes are handled by APIKeyHandler in gateway router
    //       Do NOT add them here as proxied routes.

    // ─── Existing v1 routes (unchanged) ──────────────────────────────────────
    // (giữ nguyên toàn bộ routes hiện có phía dưới)
    // ...
}
```

### Per-Route Cache TTL Config

Thêm TTL config cho các routes có cache (trong `RouteConfig` struct hoặc gateway config file):

| Route | Cache TTL |
|-------|----------|
| `/api/v2/cves/aggregations` | 5 phút (300s) |
| `/api/v2/kev/stats` | 10 phút (600s) |
| `/api/v2/kev/ransomware` | 10 phút (600s) |
| `/api/v2/vendors` | 1 giờ (3600s) |
| `/api/v2/products` | 1 giờ (3600s) |
| `/api/v2/cwe` | 1 giờ (3600s) |
| `/api/v2/capec` | 1 giờ (3600s) |
| `/api/v2/stats/dashboard` | 5 phút (300s) |
| `/api/v2/epss/stats` | 30 phút (1800s) |

Nếu `RouteConfig` chưa có `CacheTTL` field, thêm vào struct:

```go
type RouteConfig struct {
    PathPrefix   string
    Upstream     string
    SkipAuth     bool
    RequiredPerm string
    UseAPIKey    bool
    CacheTTL     time.Duration // 0 = no cache
}
```

## Acceptance Criteria

- [x] `GET /api/v2/cves/search/semantic` → routed tới `search-service` (không phải `/api/v2/cves`)
- [x] `GET /api/v2/kev/ransomware` → routed tới `data-service` (không phải bị catch bởi `/api/v2/kev`)
- [x] `GET /api/v2/webhooks` → routed tới `notification-service`
- [x] `POST /api/v2/webhooks` → routed tới `notification-service` (POST không bị block)
- [x] `POST /api/v2/sync/trigger` (unauthenticated) → 403 (requires `sync:admin` scope)
- [x] `GET /api/v2/api-keys` → handled locally bởi `gateway-service` (không proxy)
- [x] `GET /api/v2/cwe` → routed tới `search-service`, cached 1h
- [x] Routes v1 cũ vẫn hoạt động bình thường (không bị ảnh hưởng)
- [x] `go build ./...` pass không lỗi
