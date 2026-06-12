> **✅ COMPLETED** — Bridge Pattern, go build && go vet passed.

# T13 — API Gateway (Goroutine)

## Thông tin
| | |
|---|---|
| **Phase** | 3 — API Gateway |
| **Ước tính** | 4–5 giờ |
| **Depends on** | T04–T12 (tất cả service runners) |
| **Blocks** | T15, T16, T17, T18 |

## Mục tiêu

Tạo **API Gateway goroutine** — HTTP server lắng nghe `:8080`, nhận requests từ client và chuyển sang gRPC calls đến các service goroutines qua bufconn.

```
Client (HTTP)
    │
    ▼
API Gateway (HTTP :8080) — goroutine
    │
    ├── JWT middleware → gRPC call → auth-service (bufconn)
    │
    ├── /api/v1/scans      → gRPC → scan-service (bufconn)
    ├── /api/v1/findings   → gRPC → finding-service (bufconn)
    ├── /api/v1/products   → gRPC → product-service (bufconn)
    ├── /api/v1/cve        → gRPC → vuln-service (bufconn)
    ├── /api/v1/reports    → gRPC → report-service (bufconn)
    ├── /api/v1/dashboard  → gRPC → query-service (bufconn)
    ├── /api/v1/auth/*     → gRPC → auth-service (bufconn)
    └── /agent/*           → REST → agent handlers (NATS publish)
```

---

## Step 1: Tạo Gateway gRPC Clients

```go
// internal/app/clients.go
package app

import (
    "context"
    "google.golang.org/grpc"

    authpb    "github.com/osv/shared/proto/gen/go/auth/v1"
    scanpb    "github.com/osv/shared/proto/gen/go/scan/v1"
    findingpb "github.com/osv/shared/proto/gen/go/finding/v1"
    productpb "github.com/osv/shared/proto/gen/go/product/v1"
    vulnpb    "github.com/osv/shared/proto/gen/go/vulnerability/v1"
    reportpb  "github.com/osv/shared/proto/gen/go/report/v1"
    querypb   "github.com/osv/shared/proto/gen/go/query/v1"

    "github.com/osv/apps/openvulnscan/internal/transport"
)

// Clients chứa tất cả gRPC clients đến service goroutines.
type Clients struct {
    Auth    authpb.AuthServiceClient
    Scan    scanpb.ScanServiceClient
    Finding findingpb.FindingServiceClient
    Product productpb.ProductServiceClient
    Vuln    vulnpb.VulnerabilityServiceClient
    Report  reportpb.ReportServiceClient
    Query   querypb.QueryServiceClient

    // connections (để close khi shutdown)
    conns []*grpc.ClientConn
}

// NewClients tạo gRPC clients đến tất cả service goroutines.
// Phải gọi sau khi tất cả runners đã Start().
func NewClients(ctx context.Context, a *App) (*Clients, error) {
    c := &Clients{}

    type dialTarget struct {
        lis  *bufconn.Listener
        name string
    }
    // Dial all services
    conns := map[string]*grpc.ClientConn{}

    for name, lis := range map[string]*bufconn.Listener{
        "auth":    a.authLis,
        "scan":    a.scanLis,
        "finding": a.findingLis,
        "product": a.productLis,
        "vuln":    a.vulnLis,
        "report":  a.reportLis,
        "query":   a.queryLis,
    } {
        conn, err := transport.DialBufConn(ctx, lis)
        if err != nil { return nil, fmt.Errorf("dial %s: %w", name, err) }
        conns[name] = conn
        c.conns = append(c.conns, conn)
    }

    c.Auth    = authpb.NewAuthServiceClient(conns["auth"])
    c.Scan    = scanpb.NewScanServiceClient(conns["scan"])
    c.Finding = findingpb.NewFindingServiceClient(conns["finding"])
    c.Product = productpb.NewProductServiceClient(conns["product"])
    c.Vuln    = vulnpb.NewVulnerabilityServiceClient(conns["vuln"])
    c.Report  = reportpb.NewReportServiceClient(conns["report"])
    c.Query   = querypb.NewQueryServiceClient(conns["query"])

    return c, nil
}

func (c *Clients) Close() {
    for _, conn := range c.conns { conn.Close() }
}
```

---

## Step 2: Tạo `internal/gateway/router.go`

```go
// internal/gateway/router.go
package gateway

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/cors"

    "github.com/osv/apps/openvulnscan/internal/app"
    "github.com/osv/apps/openvulnscan/internal/gateway/handlers"
)

func NewRouter(clients *app.Clients, nc *nats.Conn) http.Handler {
    r := chi.NewRouter()

    // ── Global Middleware ──
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Recoverer)
    r.Use(requestLogger)
    r.Use(cors.Handler(cors.Options{
        AllowedOrigins:   []string{"*"},
        AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
        AllowCredentials: true,
    }))

    // ── Public routes ──
    r.Get("/healthz", healthHandler(clients))
    r.Get("/healthz/ready", readyHandler(clients))
    r.Post("/api/v1/auth/login",    handlers.NewAuthHandler(clients.Auth).Login)
    r.Post("/api/v1/auth/register", handlers.NewAuthHandler(clients.Auth).Register)
    r.Post("/api/v1/auth/refresh",  handlers.NewAuthHandler(clients.Auth).RefreshToken)
    r.Get("/api/v1/auth/google",           handlers.NewAuthHandler(clients.Auth).GoogleOAuth)
    r.Get("/api/v1/auth/google/callback",  handlers.NewAuthHandler(clients.Auth).GoogleCallback)

    // Agent endpoints (API key auth)
    r.Group(func(r chi.Router) {
        r.Use(APIKeyMiddleware(clients.Auth))
        r.Get("/agent/download", handlers.NewAgentHandler(nc).Download)
        r.Post("/agent/report",  handlers.NewAgentHandler(nc).SubmitReport)
    })

    // ── Protected routes (JWT) ──
    r.Group(func(r chi.Router) {
        r.Use(JWTMiddleware(clients.Auth))

        // Auth
        r.Post("/api/v1/auth/logout", handlers.NewAuthHandler(clients.Auth).Logout)
        r.Get("/api/v1/users/me",     handlers.NewAuthHandler(clients.Auth).Me)
        r.Route("/api/v1/api-keys", func(r chi.Router) {
            r.Post("/",        handlers.NewAuthHandler(clients.Auth).CreateAPIKey)
            r.Get("/",         handlers.NewAuthHandler(clients.Auth).ListAPIKeys)
            r.Delete("/{id}", handlers.NewAuthHandler(clients.Auth).DeleteAPIKey)
        })

        // Scans
        scanH := handlers.NewScanHandler(clients.Scan)
        r.Route("/api/v1/scans", func(r chi.Router) {
            r.Get("/",         scanH.List)
            r.Post("/",        scanH.Create)
            r.Get("/{id}",     scanH.Get)
            r.Delete("/{id}", scanH.Cancel)
            r.Get("/{id}/status", scanH.Status)
            r.Post("/{id}/schedule", scanH.Schedule)
        })

        // Findings
        findingH := handlers.NewFindingHandler(clients.Finding)
        r.Route("/api/v1/findings", func(r chi.Router) {
            r.Get("/",         findingH.List)
            r.Get("/{id}",     findingH.Get)
            r.Patch("/{id}/status", findingH.UpdateStatus)
        })

        // Products / Assets
        productH := handlers.NewProductHandler(clients.Product)
        r.Route("/api/v1/products", func(r chi.Router) {
            r.Get("/",       productH.List)
            r.Post("/",      productH.Create)
            r.Get("/{id}",   productH.Get)
            r.Put("/{id}",   productH.Update)
            r.Delete("/{id}", productH.Delete)
        })
        r.Route("/api/v1/assets", func(r chi.Router) {
            r.Get("/",     productH.ListAssets)
            r.Post("/",    productH.UpsertAsset)
            r.Get("/{id}", productH.GetAsset)
        })

        // CVE / Vulnerabilities
        vulnH := handlers.NewVulnHandler(clients.Vuln)
        r.Route("/api/v1/cve", func(r chi.Router) {
            r.Get("/",    vulnH.Search)
            r.Get("/{id}", vulnH.Get)
        })

        // Reports
        reportH := handlers.NewReportHandler(clients.Report)
        r.Route("/api/v1/reports", func(r chi.Router) {
            r.Post("/",    reportH.Generate)
            r.Get("/{id}", reportH.Download)
        })

        // Dashboard
        dashH := handlers.NewDashboardHandler(clients.Query, clients.Scan)
        r.Get("/api/v1/dashboard", dashH.Summary)
    })

    return r
}
```

---

## Step 3: Tạo `internal/gateway/middleware.go`

```go
// internal/gateway/middleware.go
package gateway

import (
    "context"
    "net/http"
    "strings"

    authpb "github.com/osv/shared/proto/gen/go/auth/v1"
)

type ctxKey string

const (
    ctxUserID   ctxKey = "user_id"
    ctxUserRole ctxKey = "user_role"
)

// JWTMiddleware validates Bearer token qua auth-service gRPC (bufconn).
func JWTMiddleware(authClient authpb.AuthServiceClient) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := extractBearerToken(r)
            if token == "" {
                jsonError(w, http.StatusUnauthorized, "missing authorization header")
                return
            }

            resp, err := authClient.ValidateToken(r.Context(), &authpb.ValidateTokenRequest{Token: token})
            if err != nil || !resp.Valid {
                jsonError(w, http.StatusUnauthorized, "invalid or expired token")
                return
            }

            ctx := context.WithValue(r.Context(), ctxUserID, resp.UserId)
            ctx = context.WithValue(ctx, ctxUserRole, resp.Role)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// APIKeyMiddleware validates API key qua auth-service gRPC.
func APIKeyMiddleware(authClient authpb.AuthServiceClient) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            apiKey := r.Header.Get("X-API-Key")
            if apiKey == "" {
                apiKey = r.URL.Query().Get("api_key")
            }
            if apiKey == "" {
                jsonError(w, http.StatusUnauthorized, "missing API key")
                return
            }

            resp, err := authClient.ValidateAPIKey(r.Context(), &authpb.ValidateAPIKeyRequest{ApiKey: apiKey})
            if err != nil || !resp.Valid {
                jsonError(w, http.StatusUnauthorized, "invalid API key")
                return
            }

            ctx := context.WithValue(r.Context(), ctxUserID, resp.UserId)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func extractBearerToken(r *http.Request) string {
    auth := r.Header.Get("Authorization")
    if strings.HasPrefix(auth, "Bearer ") {
        return strings.TrimPrefix(auth, "Bearer ")
    }
    return ""
}
```

---

## Step 4: Gateway Runner

```go
// internal/runners/gateway_runner.go
package runners

import (
    "context"
    "net/http"

    "github.com/osv/apps/openvulnscan/internal/app"
    "github.com/osv/apps/openvulnscan/internal/gateway"
    "github.com/nats-io/nats.go"
)

type GatewayRunner struct {
    httpAddr string
    clients  *app.Clients
    nc       *nats.Conn
    server   *http.Server
}

func NewGatewayRunner(httpAddr string, clients *app.Clients, nc *nats.Conn) *GatewayRunner {
    return &GatewayRunner{httpAddr: httpAddr, clients: clients, nc: nc}
}

func (r *GatewayRunner) Name() string { return "api-gateway" }

func (r *GatewayRunner) Run(ctx context.Context) error {
    handler := gateway.NewRouter(r.clients, r.nc)
    r.server = &http.Server{
        Addr:         r.httpAddr,
        Handler:      handler,
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 30 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    errCh := make(chan error, 1)
    go func() {
        if err := r.server.ListenAndServe(); err != http.ErrServerClosed {
            errCh <- err
        }
    }()

    select {
    case <-ctx.Done():
        shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        return r.server.Shutdown(shutCtx)
    case err := <-errCh:
        return fmt.Errorf("api-gateway: %w", err)
    }
}

func (r *GatewayRunner) Health(ctx context.Context) error {
    resp, err := http.Get("http://" + r.httpAddr + "/healthz")
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("healthz returned %d", resp.StatusCode)
    }
    return nil
}
```

---

## Step 5: Health Handler

```go
// internal/gateway/handlers/health.go
func healthHandler(clients *app.Clients) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
        defer cancel()

        results := clients.HealthAll(ctx)
        allOK := true
        for _, err := range results {
            if err != nil { allOK = false; break }
        }

        status := http.StatusOK
        if !allOK { status = http.StatusServiceUnavailable }
        jsonResponse(w, status, map[string]interface{}{
            "status":   map[bool]string{true: "ok", false: "degraded"}[allOK],
            "services": results,
        })
    }
}
```

---

## Output

- [x] `internal/app/clients.go` ✓ (connectClients() in app.go — AuthClient, FindingClient, ProductClient)
- [x] `internal/gateway/router.go` ✓ (internal/router/router.go — chi.Router tất cả routes)
- [x] `internal/gateway/middleware.go` ✓ (internal/middleware/auth.go — JWT + APIKey middleware)
- [x] `internal/gateway/handlers/auth.go` ✓ (HandleLogin, HandleRegister, HandleLogout, HandleRefresh in app.go)
- [x] `internal/gateway/handlers/scan.go` ✓ (ScanRunner.HTTPHandler mounted via router)
- [x] `internal/gateway/handlers/finding.go` ✓ (HandleListFindings, HandleGetFinding, HandleUpdateFindingStatus)
- [x] `internal/gateway/handlers/product.go` ✓ (ProductRunner.HTTPHandler mounted)
- [x] `internal/gateway/handlers/vuln.go` ✓ (VulnRunner.HTTPHandler: /cve/*)
- [x] `internal/gateway/handlers/report.go` ✓ (ReportRunner.HTTPHandler mounted: /api/v1/reports, scans/{id}/report)
- [x] `internal/gateway/handlers/dashboard.go` ✓ (internal/router/dashboard_routes.go: mountDashboardRoutes)
- [x] `internal/gateway/handlers/health.go` ✓ (router.go: /health, /ready endpoints)
- [x] `internal/runners/gateway_runner.go` ✓ (không cần u2014 gateway được implement trong internal/router/router.go)

## Acceptance Criteria

```bash
# Server start
curl http://localhost:8080/healthz
# → {"status":"ok","services":{"auth-service":null,"scan-service":null,...}}

# Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"password"}'
# → {"token":"eyJ...","refresh_token":"..."}

# Authenticated request
curl http://localhost:8080/api/v1/scans \
  -H "Authorization: Bearer eyJ..."
# → {"data":[...],"total":0}

# Create scan
curl -X POST http://localhost:8080/api/v1/scans \
  -H "Authorization: Bearer eyJ..." \
  -H "Content-Type: application/json" \
  -d '{"target":"192.168.1.1","scan_type":"full"}'
# → {"id":"...","status":"pending"}
```

## Rủi ro

| Rủi ro | Xử lý |
|--------|-------|
| Proto clients chưa có file | Tạo wrapper nếu proto chưa đủ |
| Service goroutine chưa ready khi gateway start | Thêm retry/wait logic trong NewClients |
| CORS issue với frontend | Điều chỉnh AllowedOrigins |
