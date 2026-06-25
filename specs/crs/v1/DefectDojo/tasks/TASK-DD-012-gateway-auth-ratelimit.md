# ✅ COMPLETED — TASK-DD-012 — Gateway Auth (JWT + API Key) + Rate Limiting

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-012 |
| **Service** | `apps/osv` |
| **CR** | CR-DD-011 |
| **Phase** | 1 — Foundation |
| **Priority** | 🔴 High |
| **Prerequisites** | — (độc lập) |
| **Estimated effort** | 1.5 ngày |

## Context

Mở rộng `apps/osv` với HTTP gateway layer. Implement: Auth middleware (JWT Bearer + `Token <api_key>`), Rate Limiting (Redis sliding window), Request header injection. `apps/osv` hiện chỉ có gRPC server — cần thêm HTTP gateway layer chạy song song.

## Reference

- Solution: [`sol-gateway.md`](../solutions/sol-gateway.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/
```

## Files to Create

```
internal/gateway/
├── server.go                   # HTTP server setup (port 8080)
├── auth/
│   ├── middleware.go            # Auth middleware (JWT + API Key)
│   ├── jwt.go                   # JWT validation logic
│   ├── apikey.go                # Token <key> validation via identity gRPC
│   └── claims.go                # AuthClaims struct
├── ratelimit/
│   └── middleware.go            # Redis-backed sliding window rate limiter
└── transform/
    └── headers.go               # Inject X-User-ID, X-User-Email, X-User-Roles
```

## Implementation Spec

### `internal/gateway/auth/claims.go`

```go
package auth

type AuthClaims struct {
    UserID   string
    Email    string
    Roles    []string
    AuthType string  // "jwt" | "api_key"
    IsAdmin  bool
}

type contextKey int
const claimsKey contextKey = iota

func SetClaims(ctx context.Context, claims *AuthClaims) context.Context {
    return context.WithValue(ctx, claimsKey, claims)
}
func GetClaims(ctx context.Context) *AuthClaims {
    v, _ := ctx.Value(claimsKey).(*AuthClaims)
    return v
}
```

### `internal/gateway/auth/jwt.go`

```go
package auth

import (
    "fmt"
    "net/http"
    "strings"
    "github.com/golang-jwt/jwt/v5"
)

type JWTValidator struct {
    publicKey interface{}  // RSA or HMAC key
    issuer    string
}

func (v *JWTValidator) Validate(tokenStr string) (*AuthClaims, error) {
    token, err := jwt.ParseWithClaims(tokenStr, &jwt.MapClaims{}, func(t *jwt.Token) (interface{}, error) {
        return v.publicKey, nil
    })
    if err != nil || !token.Valid {
        return nil, fmt.Errorf("invalid JWT: %w", err)
    }
    claims := token.Claims.(*jwt.MapClaims)
    return &AuthClaims{
        UserID:   (*claims)["sub"].(string),
        Email:    (*claims)["email"].(string),
        Roles:    toStringSlice((*claims)["roles"]),
        AuthType: "jwt",
        IsAdmin:  hasRole(toStringSlice((*claims)["roles"]), "Admin"),
    }, nil
}
```

### `internal/gateway/auth/apikey.go`

```go
package auth

import (
    "strings"
    "net/http"
    identityv1 "github.com/osv/pkg/clients/identity/v1"
)

type APIKeyProvider struct {
    identityClient identityv1.IdentityServiceClient
}

// Authenticate tries API Key first, then JWT
func (p *APIKeyProvider) Authenticate(r *http.Request) (*AuthClaims, error) {
    authHeader := r.Header.Get("Authorization")

    switch {
    case strings.HasPrefix(authHeader, "Token "):
        // DefectDojo-style API key: "Authorization: Token <key>"
        apiKey := strings.TrimPrefix(authHeader, "Token ")
        resp, err := p.identityClient.ValidateAPIKey(r.Context(), &identityv1.ValidateAPIKeyRequest{
            ApiKey: apiKey,
        })
        if err != nil || !resp.Valid {
            return nil, ErrInvalidAPIKey
        }
        return &AuthClaims{
            UserID:   resp.UserId,
            Email:    resp.Email,
            Roles:    resp.Roles,
            AuthType: "api_key",
            IsAdmin:  hasRole(resp.Roles, "Admin"),
        }, nil

    case strings.HasPrefix(authHeader, "Bearer "):
        token := strings.TrimPrefix(authHeader, "Bearer ")
        return p.jwtValidator.Validate(token)

    default:
        return nil, ErrNoCredentials
    }
}
```

### `internal/gateway/auth/middleware.go`

```go
package auth

import (
    "encoding/json"
    "net/http"
)

var (
    ErrInvalidAPIKey = errors.New("invalid API key")
    ErrNoCredentials = errors.New("no credentials provided")
)

// Authenticate is HTTP middleware that validates auth and sets claims in context
func (p *APIKeyProvider) Authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims, err := p.authenticate(r)
        if err != nil {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusUnauthorized)
            json.NewEncoder(w).Encode(map[string]string{
                "detail": "Authentication credentials were not provided.",
            })
            return
        }
        r = r.WithContext(SetClaims(r.Context(), claims))
        next.ServeHTTP(w, r)
    })
}
```

### `internal/gateway/ratelimit/middleware.go`

```go
package ratelimit

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "strings"
    "time"

    "github.com/osv/apps/osv/internal/gateway/auth"
    "github.com/redis/go-redis/v9"
)

type RateLimiter struct {
    redis *redis.Client
}

// Limit returns middleware that enforces a rate limit.
// spec format: "<N>/minute" | "<N>/hour" | "<N>/second"
func (rl *RateLimiter) Limit(spec string) func(http.Handler) http.Handler {
    maxRequests, window := parseSpec(spec)

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims := auth.GetClaims(r.Context())
            var userKey string
            if claims != nil {
                userKey = claims.UserID
            } else {
                userKey = r.RemoteAddr
            }

            key := fmt.Sprintf("rate:%s:%s", userKey, r.URL.Path)
            count, err := rl.redis.Incr(context.Background(), key).Result()
            if err != nil {
                // Redis unavailable — fail open (allow request)
                next.ServeHTTP(w, r)
                return
            }
            if count == 1 {
                rl.redis.Expire(context.Background(), key, window)
            }

            w.Header().Set("X-RateLimit-Limit", strconv.Itoa(maxRequests))
            remaining := maxRequests - int(count)
            if remaining < 0 { remaining = 0 }
            w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

            if int(count) > maxRequests {
                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusTooManyRequests)
                json.NewEncoder(w).Encode(map[string]string{
                    "detail": fmt.Sprintf("Request was throttled. Expected available in %s.", window),
                })
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

func parseSpec(spec string) (int, time.Duration) {
    parts := strings.Split(spec, "/")
    n, _ := strconv.Atoi(parts[0])
    switch parts[1] {
    case "second": return n, time.Second
    case "hour":   return n, time.Hour
    default:       return n, time.Minute // default: per minute
    }
}
```

### `internal/gateway/transform/headers.go`

```go
package transform

import (
    "net/http"
    "strings"
    "github.com/osv/apps/osv/internal/gateway/auth"
)

// InjectUserHeaders adds user context headers to downstream requests
func InjectUserHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims := auth.GetClaims(r.Context())
        if claims != nil {
            r.Header.Set("X-User-ID", claims.UserID)
            r.Header.Set("X-User-Email", claims.Email)
            r.Header.Set("X-User-Roles", strings.Join(claims.Roles, ","))
            r.Header.Set("X-Auth-Type", claims.AuthType)
        }
        next.ServeHTTP(w, r)
    })
}

// UserScopeFilter injects _user_id query param for scoped list endpoints
func UserScopeFilter(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims := auth.GetClaims(r.Context())
        if claims != nil && !claims.IsAdmin {
            q := r.URL.Query()
            q.Set("_user_id", claims.UserID)
            r.URL.RawQuery = q.Encode()
        }
        next.ServeHTTP(w, r)
    })
}
```

### `internal/gateway/server.go`

```go
package gateway

import (
    "context"
    "fmt"
    "net/http"
    "time"
)

type Server struct {
    httpServer *http.Server
}

func NewServer(handler http.Handler, port int) *Server {
    return &Server{
        httpServer: &http.Server{
            Addr:         fmt.Sprintf(":%d", port),
            Handler:      handler,
            ReadTimeout:  15 * time.Second,
            WriteTimeout: 30 * time.Second,
            IdleTimeout:  60 * time.Second,
        },
    }
}

func (s *Server) Start(ctx context.Context) error {
    go func() {
        <-ctx.Done()
        shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        s.httpServer.Shutdown(shutCtx)
    }()
    if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
        return err
    }
    return nil
}
```

## Wire into apps/osv Supervisor

```go
// apps/osv/cmd/server/main.go — add gateway to supervisor
gatewayServer := gateway.NewServer(
    gateway.SetupRouter(cfg, authProvider, rateLimiter),
    envPort("GATEWAY_HTTP_PORT", 8080),
)
supervisor.New(
    existingGRPCService,  // existing
    gatewayServer,        // new
).Run(ctx)
```

## Config (env vars)

```
GATEWAY_HTTP_PORT=8080
OSV_JWT_PUBLIC_KEY_BASE64=...   # RSA public key for JWT validation
OSV_IDENTITY_GRPC_ADDR=identity-service:9001  # for API key validation
REDIS_ADDR=redis:6379           # for rate limiting
```

## Acceptance Criteria

- [x] `Authorization: Bearer <valid_jwt>` → request passes auth middleware
- [x] `Authorization: Token <valid_api_key>` → request passes auth middleware
- [x] `Authorization: Bearer <invalid>` → 401 `{"detail": "Authentication credentials were not provided."}`
- [x] No auth header → 401
- [x] Rate limit 30/minute: 31st request → 429 `{"detail": "Request was throttled..."}`
- [x] X-RateLimit-Limit header present on all rate-limited routes
- [x] X-User-ID header injected into downstream requests
- [x] X-User-Roles header injected (comma-separated)
- [x] `go build ./...` thành công
- [x] Redis unavailable → auth still works (fail-open for rate limiting only)

## Implementation Status: ✅ DONE

> `apps/osv/internal/gateway/auth/middleware.go` — AuthClaims, JWT parser, APIKey validator, Provider.Authenticate() middleware
> `apps/osv/internal/gateway/ratelimit/middleware.go` — Redis sliding window, X-RateLimit-* headers, fail-open
> `apps/osv/internal/gateway/transform/headers.go` — InjectUserHeaders, UserScopeFilter
