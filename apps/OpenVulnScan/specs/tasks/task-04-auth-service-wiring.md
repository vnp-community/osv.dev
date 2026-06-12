> **✅ COMPLETED** — Implemented via Bridge Pattern. `go build && go vet` passed.

# T04 — Auth Service Wiring

## Thông tin
| | |
|---|---|
| **Phase** | 1 — Auth |
| **Ước tính** | 4–5 giờ |
| **Depends on** | T02, T03 |
| **Blocks** | T05 (JWT middleware cần trước) |

## Mục tiêu
Wire-up `auth-service` vào monolith: local login, register, Google OAuth2, JWT middleware. Tất cả business logic đã có trong service, chỉ cần khởi tạo và mount.

---

## Packages cần import từ `auth-service`

| Import path | Thành phần |
|-------------|------------|
| `github.com/osv/auth-service/internal/usecase/login/` | LoginUseCase |
| `github.com/osv/auth-service/internal/usecase/register/` | RegisterUseCase |
| `github.com/osv/auth-service/internal/usecase/logout/` | LogoutUseCase |
| `github.com/osv/auth-service/internal/usecase/refresh_token/` | RefreshTokenUseCase |
| `github.com/osv/auth-service/internal/usecase/oauth/` | OAuthUseCase |
| `github.com/osv/auth-service/internal/usecase/validate_token/` | ValidateTokenUseCase |
| `github.com/osv/auth-service/internal/usecase/manage_api_key/` | APIKeyUseCase |
| `github.com/osv/auth-service/internal/provider/` | AuthProviderChain |
| `github.com/osv/auth-service/internal/infra/auth/` | JWT signer, Redis session |

---

## Các bước thực hiện

### 4.1 Đọc API của auth-service usecases

```bash
# Xác minh constructor signatures
cat osv.dev/services/auth-service/internal/usecase/login/*.go
cat osv.dev/services/auth-service/internal/usecase/register/*.go
cat osv.dev/services/auth-service/internal/usecase/validate_token/*.go
cat osv.dev/services/auth-service/internal/usecase/oauth/*.go
cat osv.dev/services/auth-service/internal/provider/chain.go
cat osv.dev/services/auth-service/internal/infra/auth/*.go
```

> Ghi lại: tên function `New(...)`, các dependency cần inject (repo, JWT signer, etc.)

### 4.2 Khởi tạo auth repositories

Xem `auth-service/internal/infrastructure/` (hoặc `infra/`) để tìm repository implementations:

```go
// Trong app.go: thêm auth infrastructure
import (
    authinf "github.com/osv/auth-service/internal/infra/auth"
)

// JWT Signer
jwtSigner := authinf.NewJWTSigner(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiry)

// Redis session store
redisClient := redis.NewClient(&redis.Options{Addr: cfg.Redis.URL})
sessionStore := authinf.NewRedisSessionStore(redisClient)
```

### 4.3 Khởi tạo auth usecases

```go
import (
    loginuc    "github.com/osv/auth-service/internal/usecase/login"
    registeruc "github.com/osv/auth-service/internal/usecase/register"
    logoutuc   "github.com/osv/auth-service/internal/usecase/logout"
    refreshuc  "github.com/osv/auth-service/internal/usecase/refresh_token"
    oauthuc    "github.com/osv/auth-service/internal/usecase/oauth"
    validateuc "github.com/osv/auth-service/internal/usecase/validate_token"
)

// Khởi tạo (điền params sau khi đọc constructor)
loginUC    := loginuc.New(userRepo, jwtSigner, sessionStore)
registerUC := registeruc.New(userRepo, passwordHasher)
logoutUC   := logoutuc.New(sessionStore)
refreshUC  := refreshuc.New(sessionStore, jwtSigner)
oauthUC    := oauthuc.New(cfg.Auth.GoogleClientID, cfg.Auth.GoogleSecret,
                           cfg.Auth.GoogleRedirectURL, userRepo)
validateUC := validateuc.New(jwtSigner, sessionStore)
```

### 4.4 Viết `internal/middleware/auth.go`

```go
// internal/middleware/auth.go (~30 LOC)
package middleware

import (
    "net/http"
    "strings"

    validateuc "github.com/osv/auth-service/internal/usecase/validate_token"
)

type AuthMiddleware struct {
    validateUC *validateuc.UseCase // Adjust type after reading auth-service
}

func NewAuthMiddleware(uc *validateuc.UseCase) *AuthMiddleware {
    return &AuthMiddleware{validateUC: uc}
}

// RequireAuth validates JWT from Authorization header or cookie
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := extractToken(r)
        if token == "" {
            http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
            return
        }

        claims, err := m.validateUC.Execute(r.Context(), token)
        if err != nil {
            http.Error(w, `{"error":"invalid_token"}`, http.StatusUnauthorized)
            return
        }

        // Inject user info vào context
        ctx := withUserContext(r.Context(), claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func extractToken(r *http.Request) string {
    // 1. Authorization: Bearer <token>
    if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
        return strings.TrimPrefix(h, "Bearer ")
    }
    // 2. Cookie
    if c, err := r.Cookie("access_token"); err == nil {
        return c.Value
    }
    return ""
}
```

### 4.5 Mount auth routes trong router.go

```go
// internal/router/router.go — thêm auth routes
import (
    loginuc    "github.com/osv/auth-service/internal/usecase/login"
    registeruc "github.com/osv/auth-service/internal/usecase/register"
)

func mountAuthRoutes(r chi.Router, a *app.App) {
    r.Post("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            Email    string `json:"email"`
            Password string `json:"password"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        result, err := a.LoginUC.Execute(r.Context(), loginuc.Input{
            Email: req.Email, Password: req.Password,
        })
        if err != nil {
            writeJSON(w, 401, map[string]string{"error": err.Error()})
            return
        }

        // Set cookie và return token
        http.SetCookie(w, &http.Cookie{Name: "access_token", Value: result.AccessToken, HttpOnly: true})
        writeJSON(w, 200, result)
    })

    r.Post("/api/v1/auth/register", func(w http.ResponseWriter, r *http.Request) {
        // Tương tự login
    })

    r.Post("/api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
        // Dùng logoutUC.Execute()
    })

    r.Post("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
        // Dùng refreshUC.Execute()
    })

    r.Get("/api/v1/auth/google", func(w http.ResponseWriter, r *http.Request) {
        url := a.OAuthUC.GetAuthURL()
        http.Redirect(w, r, url, http.StatusTemporaryRedirect)
    })

    r.Get("/api/v1/auth/google/callback", func(w http.ResponseWriter, r *http.Request) {
        code := r.URL.Query().Get("code")
        result, err := a.OAuthUC.HandleCallback(r.Context(), code)
        if err != nil {
            http.Error(w, "oauth failed", 500)
            return
        }
        http.SetCookie(w, &http.Cookie{Name: "access_token", Value: result.AccessToken, HttpOnly: true})
        http.Redirect(w, r, "/", http.StatusSeeOther)
    })
}
```

### 4.6 Apply JWT middleware

```go
// router.go — áp dụng middleware cho protected routes
r.Group(func(r chi.Router) {
    r.Use(authMiddleware.RequireAuth)
    // Mount protected routes tại đây
    // (scan, finding, product, dashboard, etc.)
})
```

### 4.7 Seed admin user

```go
// app.go: sau khi migration xong, seed admin
func (a *App) seedAdmin() {
    exists := a.userRepo.ExistsByEmail(ctx, a.cfg.Admin.Email)
    if !exists {
        a.registerUC.Execute(ctx, registeruc.Input{
            Email:    a.cfg.Admin.Email,
            Password: a.cfg.Admin.Password,
            IsAdmin:  true,
        })
    }
}
```

---

## Output

- [x] `internal/middleware/auth.go` — JWT middleware ✓ Implemented
- [x] Auth routes mounted trong router: login, register, logout, refresh, OAuth ✓
- [x] App struct có: authHTTP handler, HandleLogin, HandleRegister, HandleLogout, HandleRefresh ✓
- [x] Admin user được seed khi khởi động ✓ (admin@openvulnscan.local / Admin123!)

## Acceptance Criteria

```bash
# Login thành công
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@openvulnscan.local","password":"admin123"}'
# → {"access_token":"eyJ...","expires_in":86400}

# Protected endpoint trả 401 khi không có token
curl http://localhost:8080/api/v1/scans
# → {"error":"unauthorized"}

# Protected endpoint hoạt động với token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -d '...' | jq -r .access_token)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/scans
# → 200 (hoặc empty list)
```

## Lưu ý

- Đọc kỹ `auth-service/internal/usecase/login/*.go` để biết chính xác `Input` struct và `Output` struct
- Xem có `UserRepository` interface ở đâu trong auth-service để biết cần implement gì
- `provider/chain.go` — xem cách auth provider chain được setup để tái sử dụng
