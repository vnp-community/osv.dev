# SOL-UI-001 — Auth Service Extension (CR-UI-001)

**Giải pháp cho:** [CR-UI-001 — Authentication & User API](../CR-UI-001-auth-api.md)  
**Ngày tạo:** 2026-06-16  
**Trạng thái:** ✅ Implemented — *Backend tasks BE-001 → BE-022 hoàn tất (2026-06-17)*  
**Service cần thay đổi:** `services/identity-service` (v2.2 existing)  
**Phụ thuộc kiến trúc:** `specs/01-architecture.md §3.10`, `specs/02-technical-design.md §10`

---

## 1. Tình trạng hiện tại

`identity-service` (:8081) hiện có:
- ✅ Auth chain: Local (bcrypt) + LDAP
- ✅ API Key management (prefix lookup + SHA-256 hash)
- ✅ JWT HS256 issuing
- ✅ Session token storage

**Thiếu hoàn toàn:**
- ❌ HTTP endpoint `POST /api/v1/auth/login` — hiện chỉ expose internal auth chain
- ❌ `POST /api/v1/auth/refresh` — refresh token mechanism (cookie-based)
- ❌ `GET /api/v1/auth/me` — current user info
- ❌ `POST /api/v1/auth/logout` — blacklist + clear cookie
- ❌ `GET /api/v1/admin/users*` — user CRUD via HTTP
- ❌ `GET /api/v1/admin/roles` — RBAC matrix endpoint
- ❌ `GET/PATCH /api/v1/profile` — user profile self-serve
- ❌ `GET/POST/DELETE /api/v1/api-keys` — API key HTTP CRUD

---

## 2. Giải Pháp

### 2.1 Kiến Trúc Giải Pháp

```
Gateway (:8080)
  │ POST /api/v1/auth/*   → identity-service (:8081)
  │ GET  /api/v1/admin/*  → identity-service (:8081)
  │ GET  /api/v1/profile  → identity-service (:8081)
  │ */api-keys*           → identity-service (:8081)
```

**Approach:** Thêm HTTP REST API handlers trực tiếp vào `identity-service`. Không tạo service mới.

> **Lý do:** identity-service đã có domain entities (User, APIKey, Session), auth chain usecase, và PostgreSQL repository. Chỉ cần thêm adapter layer HTTP handlers.

---

### 2.2 New Routes (identity-service internal router)

```go
// services/identity-service/internal/adapter/http/router.go

mux.HandleFunc("POST /auth/login",            h.Login)
mux.HandleFunc("POST /auth/refresh",          h.RefreshToken)
mux.HandleFunc("GET  /auth/me",               h.Me)
mux.HandleFunc("POST /auth/logout",           h.Logout)

mux.HandleFunc("GET  /admin/users",           h.ListUsers)
mux.HandleFunc("POST /admin/users/invite",    h.InviteUser)
mux.HandleFunc("PATCH /admin/users/{id}",     h.UpdateUser)
mux.HandleFunc("POST /admin/users/{id}/unlock",        h.UnlockUser)
mux.HandleFunc("POST /admin/users/{id}/reset-password", h.ResetPassword)
mux.HandleFunc("GET  /admin/roles",           h.GetRBACMatrix)

mux.HandleFunc("GET  /profile",               h.GetProfile)
mux.HandleFunc("PATCH /profile",              h.UpdateProfile)
mux.HandleFunc("POST /profile/change-password", h.ChangePassword)

mux.HandleFunc("GET  /api-keys",              h.ListAPIKeys)
mux.HandleFunc("POST /api-keys",              h.CreateAPIKey)
mux.HandleFunc("DELETE /api-keys/{id}",       h.RevokeAPIKey)
```

**Gateway route group** (thêm vào `apps/osv/internal/gateway/router.go`):
```go
// Auth routes — no auth required
mux.Handle("POST /api/v1/auth/login",   proxy.Forward("identity-service:8081"))
mux.Handle("POST /api/v1/auth/refresh", proxy.Forward("identity-service:8081"))

// Auth routes — JWT required
authMux := auth.Protect(mux)
authMux.Handle("GET  /api/v1/auth/me",    proxy.Forward("identity-service:8081"))
authMux.Handle("POST /api/v1/auth/logout", proxy.Forward("identity-service:8081"))
authMux.Handle("GET  /api/v1/profile",    proxy.Forward("identity-service:8081"))
authMux.Handle("PATCH /api/v1/profile",   proxy.Forward("identity-service:8081"))
authMux.Handle("POST /api/v1/profile/change-password", proxy.Forward("identity-service:8081"))
authMux.Handle("/api/v1/api-keys",        proxy.Forward("identity-service:8081"))
authMux.Handle("/api/v1/api-keys/",       proxy.Forward("identity-service:8081"))
authMux.Handle("/api/v1/admin/",          proxy.Forward("identity-service:8081")) // role:admin only
```

---

### 2.3 Login Handler

```go
// services/identity-service/internal/adapter/http/auth_handler.go

type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
    MFACode  string `json:"mfa_code,omitempty"`
}

type LoginResponse struct {
    AccessToken string   `json:"access_token"`
    ExpiresIn   int      `json:"expires_in"`
    User        *UserDTO `json:"user"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
    var req LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, 400, "VALIDATION_ERROR", "Invalid request body")
        return
    }
    
    // Delegate to existing auth chain usecase
    user, err := h.authChain.Authenticate(r.Context(), req.Email, req.Password)
    if err != nil {
        if errors.Is(err, ErrAccountLocked) {
            respondError(w, 423, "ACCOUNT_LOCKED", "Account locked after 5 failed attempts")
            return
        }
        respondError(w, 401, "INVALID_CREDENTIALS", "Invalid email or password")
        return
    }
    
    // MFA check
    if user.MFAEnabled && req.MFACode == "" {
        respondJSON(w, 200, map[string]interface{}{"mfa_required": true, "access_token": nil, "user": nil})
        return
    }
    if user.MFAEnabled {
        if !validateTOTP(user.MFASecret, req.MFACode) {
            respondError(w, 401, "INVALID_MFA_CODE", "Invalid TOTP code")
            return
        }
    }
    
    // Issue JWT (HS256, existing mechanism)
    token, expiresIn, err := h.tokenService.IssueJWT(user)
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", "Failed to issue token")
        return
    }
    
    // Issue refresh token → store hash in sessions table
    refreshToken, err := h.sessionService.CreateRefreshToken(r.Context(), user.ID)
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", "Failed to create session")
        return
    }
    
    // Set httpOnly cookie for refresh token
    http.SetCookie(w, &http.Cookie{
        Name:     "refresh_token",
        Value:    refreshToken,
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteStrictMode,
        Path:     "/api/v1/auth/refresh",
        MaxAge:   7 * 24 * 3600, // 7 days
    })
    
    respondJSON(w, 200, LoginResponse{
        AccessToken: token,
        ExpiresIn:   expiresIn,
        User:        toUserDTO(user),
    })
}
```

---

### 2.4 Refresh Token Handler

```go
func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
    cookie, err := r.Cookie("refresh_token")
    if err != nil {
        respondError(w, 401, "REFRESH_TOKEN_INVALID", "No refresh token cookie")
        return
    }
    
    // Validate refresh token from sessions table
    session, err := h.sessionService.ValidateRefreshToken(r.Context(), cookie.Value)
    if err != nil {
        if errors.Is(err, ErrTokenReused) {
            // Reuse detection: revoke entire token family
            h.sessionService.RevokeFamily(r.Context(), session.TokenFamily)
        }
        respondError(w, 401, "REFRESH_TOKEN_INVALID", "Invalid or expired refresh token")
        return
    }
    
    // Rotate: revoke old token, issue new
    user, _ := h.userRepo.FindByID(r.Context(), session.UserID)
    h.sessionService.RevokeToken(r.Context(), session.ID)
    
    newToken, expiresIn, _ := h.tokenService.IssueJWT(user)
    newRefreshToken, _ := h.sessionService.CreateRefreshToken(r.Context(), user.ID)
    
    // Rotate cookie
    http.SetCookie(w, &http.Cookie{
        Name: "refresh_token", Value: newRefreshToken,
        HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
        Path: "/api/v1/auth/refresh", MaxAge: 7 * 24 * 3600,
    })
    
    respondJSON(w, 200, map[string]interface{}{
        "access_token": newToken, "expires_in": expiresIn,
    })
}
```

---

### 2.5 User DTO (UserResponse với permissions[])

```go
// services/identity-service/internal/adapter/http/dto.go

type UserDTO struct {
    ID          string   `json:"id"`
    Email       string   `json:"email"`
    Name        string   `json:"name"`
    Role        string   `json:"role"`
    Permissions []string `json:"permissions"` // Computed from role — RBAC matrix
    MFAEnabled  bool     `json:"mfa_enabled"`
    AvatarURL   *string  `json:"avatar_url"`
    CreatedAt   string   `json:"created_at"`
}

func toUserDTO(u *domain.User) *UserDTO {
    return &UserDTO{
        ID:          u.ID.String(),
        Email:       u.Email,
        Name:        u.Name,
        Role:        string(u.Role),
        Permissions: domain.RolePermissions[u.Role], // from RBAC matrix
        MFAEnabled:  u.MFAEnabled,
        AvatarURL:   u.AvatarURL,
        CreatedAt:   u.CreatedAt.Format(time.RFC3339),
    }
}
```

**RBAC Matrix** (thêm vào domain):
```go
// services/identity-service/internal/domain/rbac.go

var RolePermissions = map[Role][]string{
    RoleAdmin:    {"scan:create", "scan:read", "asset:write", "asset:read",
                   "finding:write", "finding:read", "report:download",
                   "user:manage", "system:configure"},
    RoleUser:     {"scan:create", "scan:read", "asset:write", "asset:read",
                   "finding:write", "finding:read", "report:download"},
    RoleReadOnly: {"scan:read", "asset:read", "finding:read", "report:download"},
    RoleAgent:    {"asset:write", "agent:report"},
}
```

---

### 2.6 Session Table (Database Schema)

```sql
-- osv_identity schema — mới thêm
CREATE TABLE sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    refresh_token_hash VARCHAR(64) NOT NULL,  -- SHA-256 of refresh token
    token_family    UUID NOT NULL,             -- For reuse detection (revoke family)
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked         BOOLEAN DEFAULT FALSE,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_token_hash ON sessions(refresh_token_hash);
CREATE INDEX idx_sessions_family ON sessions(token_family);
```

---

### 2.7 Admin User CRUD Handler

```go
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
    // Middleware đã check X-User-Role == "admin"
    role := r.URL.Query().Get("role")
    isActive := r.URL.Query().Get("is_active")
    q := r.URL.Query().Get("q")
    page, pageSize := parsePagination(r)
    
    users, total, err := h.userRepo.List(r.Context(), UserFilter{
        Role: role, IsActive: isActive, Query: q,
        Page: page, PageSize: pageSize,
    })
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", err.Error())
        return
    }
    
    respondJSON(w, 200, map[string]interface{}{
        "users": mapUsers(users), "total": total, 
        "page": page, "page_size": pageSize,
    })
}
```

**Permission check trong Gateway middleware:**
```go
// apps/osv/internal/gateway/auth/middleware.go

func (m *authMiddleware) RequireRole(roles ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            userRole := r.Header.Get("X-User-Role")
            for _, required := range roles {
                if userRole == required {
                    next.ServeHTTP(w, r)
                    return
                }
            }
            http.Error(w, `{"error":"FORBIDDEN","message":"Insufficient permissions"}`, 403)
        })
    }
}

// Usage:
adminRoutes := m.RequireRole("admin")
mux.Handle("/api/v1/admin/", adminRoutes(proxy.Forward("identity-service:8081")))
```

---

### 2.8 API Keys HTTP CRUD

```go
// CREATE API Key — plaintext returned ONCE
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
    var req CreateAPIKeyRequest
    json.NewDecoder(r.Body).Decode(&req)
    userID := r.Header.Get("X-User-ID")
    
    // Generate key: ovs_ + base58(16 random bytes)
    rawKey := "ovs_" + generateRandomBase58(16)
    prefix := rawKey[:12]
    hashHex := sha256Hex(rawKey)
    
    key := &domain.APIKey{
        ID:         uuid.New(),
        UserID:     parseUUID(userID),
        Name:       req.Name,
        Prefix:     prefix,
        HashSHA256: hashHex,
        Scopes:     req.Permissions,
        ExpiresAt:  req.ExpiresAt,
    }
    h.apiKeyRepo.Create(r.Context(), key)
    
    respondJSON(w, 201, map[string]interface{}{
        "id":            key.ID,
        "name":          key.Name,
        "prefix":        prefix,
        "plaintext_key": rawKey, // ONLY TIME returned
        "permissions":   key.Scopes,
        "created_at":    time.Now().Format(time.RFC3339),
    })
}

// LIST — never return plaintext_key
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    keys, _ := h.apiKeyRepo.ListByUser(r.Context(), parseUUID(userID))
    
    dtos := make([]map[string]interface{}, len(keys))
    for i, k := range keys {
        dtos[i] = map[string]interface{}{
            "id": k.ID, "name": k.Name, "prefix": k.Prefix,
            "permissions": k.Scopes, "created_at": k.CreatedAt,
            "last_used_at": k.LastUsedAt, "expires_at": k.ExpiresAt,
            "is_active": !k.Revoked,
        }
    }
    respondJSON(w, 200, map[string]interface{}{"api_keys": dtos, "total": len(dtos)})
}
```

---

### 2.9 RBAC Matrix Endpoint

```go
func (h *Handler) GetRBACMatrix(w http.ResponseWriter, r *http.Request) {
    matrix := []map[string]interface{}{
        {"permission": "scan:create", "description": "Create and start scans",
         "roles": map[string]bool{"admin": true, "user": true, "readonly": false, "agent": false}},
        {"permission": "scan:read", "description": "View scan results",
         "roles": map[string]bool{"admin": true, "user": true, "readonly": true, "agent": true}},
        {"permission": "finding:write", "description": "Update finding status",
         "roles": map[string]bool{"admin": true, "user": true, "readonly": false, "agent": false}},
        {"permission": "finding:read", "description": "View findings",
         "roles": map[string]bool{"admin": true, "user": true, "readonly": true, "agent": false}},
        {"permission": "asset:write", "description": "Manage assets",
         "roles": map[string]bool{"admin": true, "user": true, "readonly": false, "agent": true}},
        {"permission": "asset:read", "description": "View assets",
         "roles": map[string]bool{"admin": true, "user": true, "readonly": true, "agent": false}},
        {"permission": "report:download", "description": "Download reports",
         "roles": map[string]bool{"admin": true, "user": true, "readonly": true, "agent": false}},
        {"permission": "user:manage", "description": "Manage users and roles",
         "roles": map[string]bool{"admin": true, "user": false, "readonly": false, "agent": false}},
        {"permission": "system:configure", "description": "Configure platform settings",
         "roles": map[string]bool{"admin": true, "user": false, "readonly": false, "agent": false}},
        {"permission": "agent:report", "description": "Submit agent scan results",
         "roles": map[string]bool{"admin": false, "user": false, "readonly": false, "agent": true}},
    }
    respondJSON(w, 200, map[string]interface{}{
        "roles": []string{"admin", "user", "readonly", "agent"},
        "permissions": matrix,
    })
}
```

---

## 3. Database Migrations

```sql
-- Migration: add sessions table
-- File: services/identity-service/db/migrations/003_add_sessions.sql

CREATE TABLE sessions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash  VARCHAR(64) NOT NULL UNIQUE,
    token_family        UUID NOT NULL,
    expires_at          TIMESTAMPTZ NOT NULL,
    revoked             BOOLEAN NOT NULL DEFAULT FALSE,
    revoked_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_token_hash ON sessions(refresh_token_hash);
CREATE INDEX idx_sessions_family ON sessions(token_family);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at) WHERE NOT revoked;

-- Cleanup cron: DELETE FROM sessions WHERE expires_at < NOW() - INTERVAL '1 day'
-- (Run via pg_cron or scheduled goroutine in identity-service)
```

---

## 4. Gateway Config Changes

```go
// apps/osv/internal/gateway/router.go — thêm new routes

// Public auth routes (no JWT required)
mux.Handle("POST /api/v1/auth/login",   p.Forward("identity-service:8081"))
mux.Handle("POST /api/v1/auth/refresh", p.ForwardWithTimeout("identity-service:8081", 5*time.Second))

// Protected auth routes
mux.Handle("GET /api/v1/auth/me",        am.Authenticate(p.Forward("identity-service:8081")))
mux.Handle("POST /api/v1/auth/logout",   am.Authenticate(p.Forward("identity-service:8081")))

// Profile routes
mux.Handle("GET /api/v1/profile",        am.Authenticate(p.Forward("identity-service:8081")))
mux.Handle("PATCH /api/v1/profile",      am.Authenticate(p.Forward("identity-service:8081")))
mux.Handle("POST /api/v1/profile/change-password", am.Authenticate(p.Forward("identity-service:8081")))

// API Keys routes
mux.Handle("GET /api/v1/api-keys",       am.Authenticate(p.Forward("identity-service:8081")))
mux.Handle("POST /api/v1/api-keys",      am.Authenticate(p.Forward("identity-service:8081")))
mux.Handle("DELETE /api/v1/api-keys/",   am.Authenticate(p.Forward("identity-service:8081")))

// Admin routes — role:admin check in gateway
adminHandler := am.Authenticate(am.RequireRole("admin")(p.Forward("identity-service:8081")))
mux.Handle("GET /api/v1/admin/users",    adminHandler)
mux.Handle("POST /api/v1/admin/users/",  adminHandler)
mux.Handle("PATCH /api/v1/admin/users/", adminHandler)
mux.Handle("GET /api/v1/admin/roles",    adminHandler)
```

---

## 5. File Structure (New Files)

```
services/identity-service/
├── internal/
│   ├── adapter/
│   │   └── http/
│   │       ├── auth_handler.go       [NEW] login, refresh, me, logout
│   │       ├── admin_handler.go      [NEW] user CRUD, roles matrix
│   │       ├── profile_handler.go    [NEW] profile get/update, change-password
│   │       ├── apikey_handler.go     [NEW] API key CRUD (HTTP)
│   │       ├── dto.go                [NEW] UserDTO, LoginRequest, etc.
│   │       └── router.go             [MODIFY] register new routes
│   ├── domain/
│   │   └── rbac.go                   [NEW] RolePermissions map
│   ├── usecase/
│   │   ├── session_usecase.go        [NEW] CreateRefreshToken, ValidateRefreshToken, RevokeFamily
│   │   └── auth_chain.go             [MODIFY] expose via HTTP adapter
│   └── infra/postgres/
│       └── session_repo.go           [NEW] sessions table repository
└── db/migrations/
    └── 003_add_sessions.sql          [NEW]
```

---

## 6. Acceptance Criteria

| Test | Expected |
|------|----------|
| `POST /api/v1/auth/login` valid credentials | 200 + `access_token` + `refresh_token` cookie (httpOnly) |
| `POST /api/v1/auth/login` wrong password (5x) | 401 `INVALID_CREDENTIALS`, 6th → 423 `ACCOUNT_LOCKED` |
| `POST /api/v1/auth/login` MFA enabled, no code | 200 `{"mfa_required":true}` |
| `POST /api/v1/auth/refresh` valid cookie | 200 + new token, old cookie cleared |
| `POST /api/v1/auth/refresh` reused token | 401, entire family revoked |
| `GET /api/v1/auth/me` valid Bearer | 200 user object với `permissions[]` |
| `POST /api/v1/auth/logout` | 200, JTI blacklisted, cookie cleared |
| `GET /api/v1/admin/users` as admin | 200 users list |
| `GET /api/v1/admin/users` as user | 403 FORBIDDEN |
| `POST /api/v1/api-keys` | 201 với `plaintext_key` |
| `GET /api/v1/api-keys` | 200 WITHOUT `plaintext_key`, chỉ `prefix` |
| `GET /api/v1/admin/roles` | 200 với 10 permissions × 4 roles |

---

## 7. Notes

> **v3.0 Extension:** MFA setup/confirm (§2.5-2.6 của CR-UI-001) và OAuth2 (§2.7-2.9) sẽ được implement theo CR-OVS-003 (`auth-service` với RS256, Argon2id, TOTP). Giải pháp hiện tại dùng HS256 vì identity-service đã có sẵn cơ chế đó.
>
> **Không break API Keys hiện tại:** `GET /api/v1/api-keys` endpoint mới này là UI-facing. API key validation logic trong gateway middleware (`X-Api-Key: ovs_...`) không thay đổi.
