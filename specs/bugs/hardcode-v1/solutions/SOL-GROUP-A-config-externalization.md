# SOL-GROUP-A — Config & Network Hardcode Externalization

> **Fixes**: BUG-001, BUG-003, BUG-004, BUG-005  
> **Services**: `gateway-service`, `asset-service`, `finding-service`, `notification-service`, `search-service`, `data-service`  
> **Priority**: 🔴 High — phải fix trước khi deploy lên container/K8s

---

## BUG-001 — Gateway: Hardcoded `search-service` URL

### Root Cause

`EmbeddedConfig` struct thiếu field `SearchAddr`, nên `search-service` URL không thể
được override và luôn trỏ `localhost:8083`.

### Files Cần Sửa

- `services/gateway-service/embedded.go`

### Solution

**Bước 1**: Thêm `SearchAddr` (và các field còn thiếu) vào `EmbeddedConfig`:

```go
// services/gateway-service/embedded.go

type EmbeddedConfig struct {
    JWTSecret        string
    IdentityAddr     string
    DataAddr         string
    SearchAddr       string  // [ADD] — was missing, caused BUG-001
    FindingAddr      string
    ScanAddr         string
    NotificationAddr string
    AIAddr           string
    RankingAddr      string
    AssetAddr        string
    ProductAddr      string
    SLAAddr          string
    JiraAddr         string  // [ADD] — was proxied incorrectly via FindingAddr
    AuditAddr        string  // [ADD] — for completeness
}
```

**Bước 2**: Gán biến `searchHTTP` từ config với fallback qua env var:

```go
// services/gateway-service/embedded.go — hàm WireEmbedded()

import "github.com/osv/shared/pkg/config"

func WireEmbedded(ctx context.Context, cfg EmbeddedConfig, ...) error {
    // Dùng config.Coalesce: ưu tiên EmbeddedConfig → env var → localhost
    identityHTTP     := config.Coalesce(cfg.IdentityAddr,     os.Getenv("IDENTITY_SERVICE_HTTP"),     "http://localhost:8081")
    dataHTTP         := config.Coalesce(cfg.DataAddr,         os.Getenv("DATA_SERVICE_HTTP"),         "http://localhost:8082")
    searchHTTP        := config.Coalesce(cfg.SearchAddr,       os.Getenv("SEARCH_SERVICE_HTTP"),       "http://localhost:8083") // [FIX BUG-001]
    findingHTTP      := config.Coalesce(cfg.FindingAddr,      os.Getenv("FINDING_SERVICE_HTTP"),      "http://localhost:8085")
    scanHTTP         := config.Coalesce(cfg.ScanAddr,         os.Getenv("SCAN_SERVICE_HTTP"),         "http://localhost:8084")
    notificationHTTP := config.Coalesce(cfg.NotificationAddr, os.Getenv("NOTIFICATION_SERVICE_HTTP"), "http://localhost:8087")
    assetHTTP        := config.Coalesce(cfg.AssetAddr,        os.Getenv("ASSET_SERVICE_HTTP"),        "http://localhost:8091")
    productHTTP      := config.Coalesce(cfg.ProductAddr,      os.Getenv("PRODUCT_SERVICE_HTTP"),      "http://localhost:8089")
    slaHTTP          := config.Coalesce(cfg.SLAAddr,          os.Getenv("SLA_SERVICE_HTTP"),          "http://localhost:8086")
    aiHTTP           := config.Coalesce(cfg.AIAddr,           os.Getenv("AI_SERVICE_HTTP"),           "http://localhost:9103")
    jiraHTTP         := config.Coalesce(cfg.JiraAddr,         os.Getenv("JIRA_SERVICE_HTTP"),         "http://localhost:8088")
    auditHTTP        := config.Coalesce(cfg.AuditAddr,        os.Getenv("AUDIT_SERVICE_HTTP"),        "http://localhost:8090")

    // Log warning nếu dùng localhost fallback
    warnIfLocalhost("IDENTITY_SERVICE_HTTP", identityHTTP)
    warnIfLocalhost("DATA_SERVICE_HTTP", dataHTTP)
    warnIfLocalhost("SEARCH_SERVICE_HTTP", searchHTTP)   // BUG-001 fix
    // ... tương tự các service khác

    upstreamURLs := map[string]string{
        "identity-service":     identityHTTP,
        "data-service":         dataHTTP,
        "search-service":       searchHTTP,  // [FIX] was "http://localhost:8083" hardcoded
        "search":               searchHTTP,  // [FIX] was "http://localhost:8083" hardcoded
        "finding-service":      findingHTTP,
        "scan-service":         scanHTTP,
        "notification-service": notificationHTTP,
        "asset-service":        assetHTTP,
        "product-service":      productHTTP,
        "sla-service":          slaHTTP,
        "ai-service":           aiHTTP,
        "jira-service":         jiraHTTP,
        "audit-service":        auditHTTP,
    }

    // Truyền searchHTTP vào NewUIAPIHandler (line 145)
    uiHandler := NewUIAPIHandler(ctx, UIAPIHandlerConfig{
        // ...
        SearchServiceURL:  searchHTTP,  // [FIX] was "http://localhost:8083" hardcoded
        ProductServiceURL: productHTTP,
        IdentityServiceURL: identityHTTP,
        // ...
    })
    
    return nil
}

// Helper: log warning khi service URL trỏ về localhost
func warnIfLocalhost(envKey, addr string) {
    if strings.Contains(addr, "localhost") || strings.Contains(addr, "127.0.0.1") {
        log.Warn().
            Str("env_key", envKey).
            Str("addr", addr).
            Msg("service addr points to localhost — set env var for container/K8s deployment")
    }
}
```

### Env Vars Cần Set Trong Docker Compose / K8s

```yaml
# docker-compose.yml / K8s ConfigMap
environment:
  IDENTITY_SERVICE_HTTP:     http://identity-service:8081
  DATA_SERVICE_HTTP:         http://data-service:8082
  SEARCH_SERVICE_HTTP:       http://search-service:8083    # BUG-001 fix
  FINDING_SERVICE_HTTP:      http://finding-service:8085
  SCAN_SERVICE_HTTP:         http://scan-service:8084
  NOTIFICATION_SERVICE_HTTP: http://notification-service:8087
  ASSET_SERVICE_HTTP:        http://asset-service:8091
  PRODUCT_SERVICE_HTTP:      http://product-service:8089
  SLA_SERVICE_HTTP:          http://sla-service:8086
  AI_SERVICE_HTTP:           http://ai-service:9103
  JIRA_SERVICE_HTTP:         http://jira-service:8088
  AUDIT_SERVICE_HTTP:        http://audit-service:8090
```

---

## BUG-003 — OSV Handler: Hardcoded gRPC/HTTP Fallback + Non-configurable Timeout

### Files Cần Sửa

- `services/gateway-service/internal/proxy/osv_handler.go`

### Solution

```go
// services/gateway-service/internal/proxy/osv_handler.go

import (
    "github.com/osv/shared/pkg/config"
)

// OSVHandlerConfig giữ tất cả cấu hình cho OSV handler.
type OSVHandlerConfig struct {
    DataServiceAddr   string        // gRPC address của data-service
    SearchServiceHTTP string        // HTTP address của search-service
    HTTPTimeoutSec    int           // timeout cho HTTP client (seconds)
}

// OSVHandlerConfigFromEnv load config từ env vars với fallback + warning log.
func OSVHandlerConfigFromEnv() OSVHandlerConfig {
    dataAddr := config.ServiceAddr("DATA_SERVICE_ADDR", "localhost", 50053)
    // [FIX] Thêm warning log — trước đây silent fallback
    // config.ServiceAddr đã tự log WARN khi dùng fallback

    searchHTTP := config.HTTPServiceAddr("SEARCH_SERVICE_HTTP", "localhost", 8083)
    // [FIX] Thêm warning log

    timeoutSec := config.Int("OSV_HANDLER_TIMEOUT_SEC", 15)
    // [FIX] Timeout nay configurable qua env var

    return OSVHandlerConfig{
        DataServiceAddr:   dataAddr,
        SearchServiceHTTP: searchHTTP,
        HTTPTimeoutSec:    timeoutSec,
    }
}

// OSVV1Router dùng config để khởi tạo HTTP client với timeout có thể cấu hình.
func NewOSVV1Router(cfg OSVHandlerConfig) *OSVV1Router {
    return &OSVV1Router{
        dataServiceAddr:   cfg.DataServiceAddr,
        searchServiceHTTP: cfg.SearchServiceHTTP,
        httpClient: &http.Client{
            Timeout: time.Duration(cfg.HTTPTimeoutSec) * time.Second, // [FIX] was hardcoded 15s
        },
    }
}
```

---

## BUG-004 — Asset Service & Multi-Service: Hardcoded gRPC Targets + Dev Credentials

### Files Cần Sửa

- `services/asset-service/embedded.go`
- `services/finding-service/embedded.go`
- `services/notification-service/cmd/server/main.go`
- `services/search-service/cmd/server/main.go`

### Solution: asset-service/embedded.go

```go
// services/asset-service/embedded.go

import "github.com/osv/shared/pkg/config"

func WireEmbedded(ctx context.Context, ...) error {
    // [FIX] Thay vì:
    //   grpcTarget := os.Getenv("FINDING_SERVICE_GRPC")
    //   if grpcTarget == "" { grpcTarget = "localhost:50060" }
    //
    // Dùng:
    grpcTarget := config.ServiceAddr("FINDING_SERVICE_GRPC", "localhost", 50060)
    // → tự động log WARN nếu dùng fallback localhost

    natsURL := config.Str("NATS_URL", "nats://localhost:4222")
    
    // ...
}
```

### Solution: notification-service/cmd/server/main.go

```go
// services/notification-service/cmd/server/main.go

import (
    "github.com/osv/shared/pkg/config"
)

func main() {
    // [FIX] Xóa credentials ra khỏi source code
    // TRƯỚC (BUG):
    //   postgresDSN := envOr("POSTGRES_DSN", "postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable")
    //
    // SAU (FIX): Không có credentials trong default — fail fast nếu thiếu
    infraCfg := config.LoadInfraConfig()
    // LoadInfraConfig() sẽ:
    //   1. Đọc POSTGRES_DSN trước
    //   2. Nếu thiếu, build từ POSTGRES_HOST + POSTGRES_USER + POSTGRES_PASSWORD
    //   3. log.Fatal() nếu không có credentials nào

    redisURL := config.Str("REDIS_URL", "redis://localhost:6379/0")
    natsURL  := config.Str("NATS_URL", "nats://localhost:4222")

    // ...
}
```

### Solution: search-service/cmd/server/main.go

```go
// services/search-service/cmd/server/main.go

// [FIX] Thay:
//   redisAddr := envOr("REDIS_ADDR", "localhost:6379")
// Dùng:
    redisAddr := config.ServiceAddr("REDIS_ADDR", "localhost", 6379)
```

### Cập Nhật `.env.example` / Docker Compose

```yaml
# deploy/dev/docker-compose.yml — notification-service

notification-service:
  environment:
    POSTGRES_DSN: postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/osvdb?sslmode=disable
    # Hoặc dùng riêng lẻ:
    # POSTGRES_USER: osv
    # POSTGRES_PASSWORD: ${SECRET_POSTGRES_PASSWORD}
    # POSTGRES_HOST: postgres
    REDIS_URL: redis://redis:6379/0
    NATS_URL: nats://nats:4222
    FINDING_SERVICE_GRPC: finding-service:50060
```

> ⚠️ **SECURITY**: Xóa `osv_dev` password khỏi source code. Sử dụng Docker secrets
> hoặc K8s Secrets để inject credentials. Không commit `.env` vào git.

---

## BUG-005 — Prometheus Metrics Port Hardcode

### Files Cần Sửa

- `services/gateway-service/cmd/server/main.go` (port 9090)
- `services/search-service/cmd/server/main.go` (port 9091)
- `services/data-service/cmd/server/main.go` (port 9092)
- `services/notification-service/cmd/server/main.go` (port 9094)

### Solution

Mỗi service đọc `METRICS_PORT` từ env với port mặc định khác nhau:

```go
// gateway-service/cmd/server/main.go
metricsPort := config.Int("METRICS_PORT", 9090)
observability.StartMetricsServer(metricsPort)  // [FIX] was: observability.StartMetricsServer(9090)

// search-service/cmd/server/main.go
metricsPort := config.Int("METRICS_PORT", 9091)
observability.StartMetricsServer(metricsPort)  // [FIX] was: observability.StartMetricsServer(9091)

// data-service/cmd/server/main.go
metricsPort := config.Int("METRICS_PORT", 9092)
observability.StartMetricsServer(metricsPort)  // [FIX] was: observability.StartMetricsServer(9092)

// notification-service/cmd/server/main.go
metricsPort := config.Int("METRICS_PORT", 9094)
observability.StartMetricsServer(metricsPort)  // [FIX] was: observability.StartMetricsServer(9094)
```

### Cập Nhật Signature `StartMetricsServer` (Optional Enhancement)

Nếu muốn refactor deeper, cập nhật `shared/pkg/observability`:

```go
// shared/pkg/observability/metrics.go

// StartMetricsServer khởi động Prometheus HTTP server.
// port = 0 sẽ đọc từ env var METRICS_PORT, fallback về defaultPort.
func StartMetricsServer(defaultPort int) {
    port := config.Int("METRICS_PORT", defaultPort)
    addr := fmt.Sprintf(":%d", port)
    go func() {
        mux := http.NewServeMux()
        mux.Handle("/metrics", promhttp.Handler())
        log.Info().Int("port", port).Msg("metrics server started")
        if err := http.ListenAndServe(addr, mux); err != nil {
            log.Error().Err(err).Msg("metrics server failed")
        }
    }()
}
```

### Prometheus scrape config (prometheus.yml)

```yaml
scrape_configs:
  - job_name: gateway-service
    static_configs:
      - targets: ['gateway-service:9090']
  - job_name: search-service
    static_configs:
      - targets: ['search-service:9091']
  - job_name: data-service
    static_configs:
      - targets: ['data-service:9092']
  - job_name: notification-service
    static_configs:
      - targets: ['notification-service:9094']
```

---

## Tóm Tắt Thay Đổi

| Bug | File | Thay Đổi Chính |
|-----|------|----------------|
| BUG-001 | `gateway-service/embedded.go` | Thêm `SearchAddr` vào `EmbeddedConfig`; dùng `coalesce(cfg, env, localhost)` |
| BUG-003 | `proxy/osv_handler.go` | Dùng `config.ServiceAddr()` + `config.Int()` cho timeout |
| BUG-004 | `asset-service/embedded.go` | Dùng `config.ServiceAddr()` cho gRPC target |
| BUG-004 | `notification-service/main.go` | Xóa credentials; dùng `config.LoadInfraConfig()` |
| BUG-004 | `search-service/main.go` | Dùng `config.ServiceAddr()` cho Redis |
| BUG-005 | 4 service main.go | Dùng `config.Int("METRICS_PORT", defaultPort)` |

## Test Verification

```bash
# Verify BUG-001: search-service URL có thể override
SEARCH_SERVICE_HTTP=http://search-service:8083 go run ./services/gateway-service/cmd/server/
# → Log không có WARN về SEARCH_SERVICE_HTTP

# Verify BUG-004: service fail nếu thiếu credentials
# (bỏ qua POSTGRES_DSN, POSTGRES_USER, POSTGRES_PASSWORD)
go run ./services/notification-service/cmd/server/
# → log.Fatal(): "required env var not set..."

# Verify BUG-005: metrics port có thể thay đổi
METRICS_PORT=19090 go run ./services/gateway-service/cmd/server/
# → metrics server starts on :19090
```
