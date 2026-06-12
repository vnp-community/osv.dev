> **✅ COMPLETED**

# T04 — Auth Service Runner (Goroutine)

## Thông tin
| | |
|---|---|
| **Phase** | 1 — Tier 1 goroutines |
| **Ước tính** | 3–4 giờ |
| **Depends on** | T00, T02, T03 |
| **Blocks** | T08, T09, T13 |

## Mục tiêu

Tạo `internal/runners/auth_runner.go` — goroutine chạy `auth-service` như một **gRPC server trên bufconn**.

Mọi service khác (API gateway, middleware) kết nối đến auth-service qua **gRPC client → authLis (bufconn)**.

> **Không thay đổi** bất kỳ code nào trong `services/auth-service/`.

---

## Step 1: Kiểm tra auth-service internal packages

```bash
# Xác nhận module name
grep "^module" services/auth-service/go.mod

# Liệt kê packages quan trọng
find services/auth-service/internal -name "*.go" | head -40

# Xác nhận delivery/grpc handler
ls services/auth-service/internal/delivery/grpc/
# hoặc
ls services/auth-service/internal/adapter/grpc/
```

**Ghi lại**:
- [x] Module name: `github.com/osv/auth-service` ✅
- [x] gRPC handler: `adapter/handler/grpc/auth_grpc_handler.go` — implement `internal/infra/auth/genproto/auth/v1` (internal proto)
- [x] Proto service: `AuthService` (ValidateToken, ValidateAPIKey)
- [x] Repo paths: `adapter/repository/postgres/{user_repo,session_repo,api_key_repo,oauth_account_repo}.go`
- [x] **Phát hiện**: `internal/` của auth-service không thể import từ module ngoài → dùng **Bridge Pattern**

---

## Step 2: Tạo `internal/runners/auth_runner.go`

```go
// internal/runners/auth_runner.go
package runners

import (
    "context"
    "fmt"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    "google.golang.org/grpc/health/grpc_health_v1"
    "google.golang.org/grpc/test/bufconn"

    // ↓ Điều chỉnh các import paths sau khi kiểm tra Step 1 ↓

    // Proto registration
    authpb "github.com/osv/shared/proto/gen/go/auth/v1"

    // auth-service internals (không thay đổi code)
    authgrpc    "github.com/osv/auth-service/internal/delivery/grpc"
    authlogin   "github.com/osv/auth-service/internal/usecase/login"
    authregister "github.com/osv/auth-service/internal/usecase/register"
    authlogout  "github.com/osv/auth-service/internal/usecase/logout"
    authrefresh "github.com/osv/auth-service/internal/usecase/refresh_token"
    authoauth   "github.com/osv/auth-service/internal/usecase/oauth"
    authtoken   "github.com/osv/auth-service/internal/usecase/validate_token"
    authapikey  "github.com/osv/auth-service/internal/usecase/manage_api_key"
    authjwt     "github.com/osv/auth-service/internal/infra/auth"
    authrepo    "github.com/osv/auth-service/internal/infra/repository"

    "github.com/osv/apps/openvulnscan/internal/transport"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
)

// AuthRunnerConfig chứa tất cả config cần thiết cho auth goroutine.
type AuthRunnerConfig struct {
    DBURL         string
    RedisURL      string
    JWTSecret     string
    JWTExpiry     time.Duration
    RefreshExpiry time.Duration
    GoogleClientID string
    GoogleSecret   string
    GoogleRedirect string
}

// AuthRunner implement ServiceRunner cho auth-service.
type AuthRunner struct {
    cfg    AuthRunnerConfig
    lis    *bufconn.Listener  // bufconn listener — các clients kết nối qua đây
    server *grpc.Server
}

func NewAuthRunner(cfg AuthRunnerConfig, lis *bufconn.Listener) *AuthRunner {
    return &AuthRunner{cfg: cfg, lis: lis}
}

func (r *AuthRunner) Name() string { return "auth-service" }

func (r *AuthRunner) Run(ctx context.Context) error {
    // 1. Kết nối DB (auth-service dùng pgx trực tiếp)
    db, err := pgxpool.New(ctx, r.cfg.DBURL)
    if err != nil {
        return fmt.Errorf("auth-runner: db connect: %w", err)
    }
    defer db.Close()

    // 2. Kết nối Redis (session store)
    redisOpt, err := redis.ParseURL(r.cfg.RedisURL)
    if err != nil {
        return fmt.Errorf("auth-runner: redis url: %w", err)
    }
    rdb := redis.NewClient(redisOpt)
    defer rdb.Close()

    // 3. Khởi tạo repos từ auth-service
    // TODO: điều chỉnh constructor theo Step 1
    userRepo    := authrepo.NewUserRepository(db)
    sessionRepo := authrepo.NewSessionRepository(rdb)
    apiKeyRepo  := authrepo.NewAPIKeyRepository(db)

    // 4. JWT signer
    jwtSigner := authjwt.NewJWTSigner(r.cfg.JWTSecret, r.cfg.JWTExpiry, r.cfg.RefreshExpiry)

    // 5. Usecases từ auth-service
    loginUC    := authlogin.New(userRepo, sessionRepo, jwtSigner)
    registerUC := authregister.New(userRepo, jwtSigner)
    logoutUC   := authlogout.New(sessionRepo)
    refreshUC  := authrefresh.New(sessionRepo, jwtSigner)
    validateUC := authtoken.New(sessionRepo, jwtSigner)
    apiKeyUC   := authapikey.New(apiKeyRepo)
    oauthUC    := authoauth.New(userRepo, sessionRepo, jwtSigner,
        r.cfg.GoogleClientID, r.cfg.GoogleSecret, r.cfg.GoogleRedirect)

    // 6. gRPC handler từ auth-service
    handler := authgrpc.NewHandler(loginUC, registerUC, logoutUC, refreshUC, validateUC, apiKeyUC, oauthUC)

    // 7. Start gRPC server trên bufconn listener
    r.server = grpc.NewServer(
        grpc.ChainUnaryInterceptor(
            grpcRecoveryInterceptor,
            grpcLoggingInterceptor,
        ),
    )
    authpb.RegisterAuthServiceServer(r.server, handler)
    grpc_health_v1.RegisterHealthServer(r.server, health.NewServer())

    errCh := make(chan error, 1)
    go func() { errCh <- r.server.Serve(r.lis) }()

    select {
    case <-ctx.Done():
        r.server.GracefulStop()
        return nil
    case err := <-errCh:
        return fmt.Errorf("auth-service gRPC: %w", err)
    }
}

// Health kiểm tra qua gRPC health check protocol.
func (r *AuthRunner) Health(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()
    conn, err := transport.DialBufConn(ctx, r.lis)
    if err != nil { return fmt.Errorf("auth health dial: %w", err) }
    defer conn.Close()

    hc := grpc_health_v1.NewHealthClient(conn)
    resp, err := hc.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
    if err != nil { return fmt.Errorf("auth health check: %w", err) }
    if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
        return fmt.Errorf("auth not serving: %s", resp.Status)
    }
    return nil
}
```

---

## Step 3: Thêm vào App wiring

Trong `internal/app/app.go`, thêm authLis và register runner:

```go
// Trong App struct
authLis *bufconn.Listener

// Trong New():
a.authLis = transport.NewBufConnListener()
a.registry.Register(runners.NewAuthRunner(runners.AuthRunnerConfig{
    DBURL:         cfg.Database.URL,
    RedisURL:      cfg.Redis.URL,
    JWTSecret:     cfg.Auth.JWTSecret,
    JWTExpiry:     cfg.Auth.JWTExpiry,
    RefreshExpiry:  cfg.Auth.RefreshExpiry,
    GoogleClientID: cfg.Auth.GoogleClientID,
    GoogleSecret:   cfg.Auth.GoogleSecret,
    GoogleRedirect: cfg.Auth.GoogleRedirectURL,
}, a.authLis))
```

---

## Step 4: gRPC client cho API Gateway

Sau khi Start(), tạo client để API Gateway dùng:

```go
// Trong connectClients():
authConn, err := transport.DialBufConn(ctx, a.authLis)
if err != nil { return fmt.Errorf("auth client: %w", err) }
a.authClient = authpb.NewAuthServiceClient(authConn)
```

---

## Step 5: JWT Middleware dùng authClient

```go
// internal/gateway/middleware.go
package gateway

func JWTMiddleware(authClient authpb.AuthServiceClient) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := r.Header.Get("Authorization")
            if token == "" {
                http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
                return
            }
            token = strings.TrimPrefix(token, "Bearer ")

            // Validate qua auth-service gRPC (in-process bufconn)
            resp, err := authClient.ValidateToken(r.Context(), &authpb.ValidateTokenRequest{
                Token: token,
            })
            if err != nil || !resp.Valid {
                http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
                return
            }

            // Inject user info vào context
            ctx := context.WithValue(r.Context(), "user_id", resp.UserId)
            ctx = context.WithValue(ctx, "user_role", resp.Role)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

---

## Output

- [x] `internal/runners/auth_runner.go` — AuthRunner: gRPC server trên bufconn
- [x] `internal/runners/auth_bridge.go` — authServiceBridge implement shared/proto, JWT validation trực tiếp
- [x] `App.authLis` được tạo và truyền vào runner ✔
- [x] `App.AuthClient` (authv1.AuthServiceClient) dùng shared/proto client
- [x] `internal/gateway/middleware.go` — JWT middleware (T13 — API Gateway) ✓ Implemented in internal/middleware/auth.go

## Trạng thái: ✅ HOÀN THÀNH
> Thực thi: 2026-06-09
> **Bridge Pattern**: AuthRunner không dùng auth-service/internal/; implement JWT validation trực tiếp trong auth_bridge.go
> `go build ./...` thành công

**Giải pháp Go internal restriction**:
- auth-service/internal/ không thể import từ module ngoài (Go rule)
- Thách thức: auth-service gRPC handler dùng internal proto
- Giải pháp: `auth_bridge.go` implement `sharedauthv1.AuthServiceServer` trực tiếp:
  - JWT validation dùng `golang-jwt/jwt/v5` trực tiếp (không qua auth-service/internal/jwt)
  - API key lookup: query trực tiếp vào `auth.api_keys` table
  - Redis JTI blacklist check trực tiếp

## Acceptance Criteria

```go
// auth-service goroutine khởi động và health check pass
runner := runners.NewAuthRunner(cfg, lis)
go runner.Run(ctx)
time.Sleep(200 * time.Millisecond)

err := runner.Health(ctx)
assert.NoError(t, err) // gRPC health check SERVING

// Login qua gRPC bufconn
conn, _ := transport.DialBufConn(ctx, lis)
client := authpb.NewAuthServiceClient(conn)
resp, err := client.Login(ctx, &authpb.LoginRequest{
    Email:    "admin@test.com",
    Password: "password",
})
assert.NoError(t, err)
assert.NotEmpty(t, resp.Token)
```

## Rủi ro

| Rủi ro | Xử lý |
|--------|-------|
| auth-service không có gRPC delivery | Tạo thin gRPC wrapper trong internal/runners/auth_runner.go |
| Proto file không match | Xác nhận proto path từ shared/proto |
| Constructor signatures khác | Đọc auth-service code trước khi implement |
