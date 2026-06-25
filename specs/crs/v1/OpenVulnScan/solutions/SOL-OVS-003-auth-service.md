# SOL-OVS-003 — Giải Pháp: Auth Service (JWT RS256, RBAC, MFA, API Keys)

| Trường | Giá trị |
|--------|---------|
| **Solution ID** | SOL-OVS-003 |
| **CR tham chiếu** | CR-OVS-003 |
| **Tiêu đề** | Auth Service — JWT RS256 Authentication, RBAC, TOTP MFA, API Keys (ovs_ prefix), Google/GitHub OAuth2 |
| **Ngày tạo** | 2026-06-16 |
| **Ngày implement** | 2026-06-17 |
| **Trạng thái** | ✅ Implemented |

---

## 0. Implementation Status

> **Trạng thái**: ✅ **IMPLEMENTED** — 2026-06-17

| Task | File | Trạng thái |
|------|------|------------|
| T-AUTH-001 | `identity-service/internal/domain/entity/` | ✅ Done |
| T-AUTH-002 | `identity-service/internal/crypto/` | ✅ Done |
| T-AUTH-003 | `identity-service/internal/cache/redis/jti_cache.go` | ✅ Done |
| T-AUTH-004 | `identity-service/internal/usecase/register/` + `login/` | ✅ Done |
| T-AUTH-005 | `identity-service/internal/usecase/refresh/` + `logout/` | ✅ Done |
| T-AUTH-006 | `identity-service/internal/delivery/grpc/` | ✅ Done |
| T-AUTH-007 | `identity-service/internal/usecase/apikey/usecase.go` | ✅ Done |
| T-AUTH-008 | `identity-service/internal/usecase/totp/usecase.go` | ✅ Done |
| T-AUTH-009 | `identity-service/internal/usecase/oauth2/usecase.go` | ✅ Done |
| T-AUTH-010 | `identity-service/internal/delivery/http/handlers.go` | ✅ Done |

**Chi tiết implementation**:
- **Argon2id**: memory=64MB, iterations=3, parallelism=2, PHC format
- **JWT RS256**: `kid` header, 15min access token, JWKS endpoint
- **Redis JTI**: `auth:jti:` prefix, pipeline-based Store/Revoke/IsRevoked, `O(1)` lookup
- **Token Family**: refresh token rotation + reuse detection → revoke entire family
- **API Key**: `ovs_` prefix, SHA-256 stored, constant-time compare, prefix-indexed lookup
- **TOTP**: RFC 6238, ±1 step clock skew, AES-256-GCM encrypted secrets
- **OAuth2**: Google + GitHub, upsert user by email, CSRF state parameter
- **Account Lockout**: 5 failed → lock, 15min auto-unlock background job

---

## 1. Tổng Quan Giải Pháp

### 1.1 Bối Cảnh

`auth-service` là **foundation** của toàn bộ OpenVulnScan. Mọi request đến unified-gateway đều phải đi qua auth-service để validate. Đây là service có yêu cầu **performance khắt khe nhất**: `ValidateToken` gRPC phải hoàn thành trong < 1ms.

### 1.2 Design Decisions Quan Trọng

| Decision | Rationale |
|----------|-----------|
| **RS256 thay vì HS256** | Asymmetric signing: services chỉ cần public key để verify, private key chỉ ở auth-service |
| **15min access token** | Ngắn để limit exposure nếu token bị leak; refresh token bù lại UX |
| **Argon2id thay vì bcrypt** | Memory-hard, OWASP recommended cho 2024+, chống GPU brute force tốt hơn |
| **Redis cho JTI** | O(1) lookup, không DB call. ValidateToken = crypto verify + Redis GET |
| **Token family tracking** | Phát hiện refresh token reuse attack; revoke toàn bộ family khi phát hiện |
| **API key prefix `ovs_`** | User-friendly identification (như GitHub's `ghp_`, Stripe's `sk_`) |

---

## 2. Kiến Trúc

### 2.1 Auth Flow Overview

```
Client ──▶ unified-gateway
                │
                │ (every request)
                ▼ gRPC call
           auth-service.ValidateToken()
                │
                ├─ Step 1: Parse JWT (RS256, crypto verify) — ~0.1ms
                ├─ Step 2: Check JTI in Redis — ~0.5ms  
                └─ Return: {user_id, role, permissions}

Total: < 1ms target (Redis in same DC = ~0.1-0.3ms RTT)
```

### 2.2 Cấu Trúc Thư Mục

```
services/auth-service/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── entity/
│   │   │   ├── user.go            # User, Role, Permission constants
│   │   │   ├── session.go         # Session (refresh token)
│   │   │   └── api_key.go         # APIKey entity
│   │   └── port/
│   │       ├── user_repository.go
│   │       ├── session_repository.go
│   │       └── api_key_repository.go
│   ├── usecase/
│   │   ├── register/usecase.go
│   │   ├── login/usecase.go
│   │   ├── refresh_token/usecase.go
│   │   ├── logout/usecase.go
│   │   ├── validate_token/usecase.go   # HOT PATH: Redis only
│   │   ├── manage_api_key/
│   │   │   ├── create.go
│   │   │   ├── list.go
│   │   │   └── revoke.go
│   │   ├── mfa/
│   │   │   ├── setup.go
│   │   │   └── confirm.go
│   │   └── oauth2/
│   │       ├── google.go
│   │       └── github.go
│   ├── adapter/
│   │   ├── repository/postgres/
│   │   │   ├── user_repo.go
│   │   │   ├── session_repo.go
│   │   │   └── api_key_repo.go
│   │   └── cache/redis/
│   │       └── jwt_cache.go
│   └── delivery/
│       ├── http/
│       │   ├── auth_handler.go
│       │   ├── mfa_handler.go
│       │   ├── api_key_handler.go
│       │   └── oauth2_handler.go
│       └── grpc/
│           └── auth_server.go          # ValidateToken, ValidateAPIKey
├── migrations/
│   └── 001_create_auth_tables.sql
├── crypto/
│   ├── argon2.go
│   └── jwt.go
└── config/config.yaml
```

---

## 3. JWT Design Chi Tiết

### 3.1 Token Structure

```json
// JWT Header
{
  "alg": "RS256",
  "typ": "JWT",
  "kid": "key-2026-06"  // Key ID for rotation
}

// JWT Claims
{
  "sub": "user-uuid-here",          // User ID
  "iss": "openvulnscan-auth",       // Issuer
  "aud": ["openvulnscan-api"],      // Audience
  "exp": 1718535600,                // 15 minutes from issue
  "iat": 1718534700,                // Issued at
  "jti": "uuid-v4-jti",            // JWT ID (for blacklist)
  "role": "user",                   // User role
  "permissions": [                  // Resolved permissions
    "scan:create", "scan:read",
    "finding:write", "finding:read",
    "report:download"
  ]
}
```

### 3.2 JTI Redis Management

```go
// auth-service/internal/adapter/cache/redis/jwt_cache.go

const (
    jtiKeyPrefix    = "auth:jwt:"
    jtiTTLBuffer    = 60 * time.Second  // Extra buffer for clock skew
)

type JWTCache struct {
    client *redis.Client
}

// SetJTI — called on login/token generation
func (c *JWTCache) SetJTI(ctx context.Context, jti, userID string, ttl time.Duration) error {
    return c.client.Set(ctx, jtiKeyPrefix+jti, userID, ttl+jtiTTLBuffer).Err()
}

// CheckJTI — called on every request validation (hot path)
// Returns (exists, error) — must be O(1)
func (c *JWTCache) CheckJTI(ctx context.Context, jti string) (bool, error) {
    result, err := c.client.Exists(ctx, jtiKeyPrefix+jti).Result()
    if err != nil { return false, err }
    return result > 0, nil
}

// RevokeJTI — called on logout
func (c *JWTCache) RevokeJTI(ctx context.Context, jti string) error {
    return c.client.Del(ctx, jtiKeyPrefix+jti).Err()
}
```

### 3.3 RS256 Key Management

```go
// crypto/jwt.go

// Private key: loaded once at startup from /secrets/jwt_private.pem
// Public key: shared with all services via /secrets/jwt_public.pem
// OR: served via JWKS endpoint (/.well-known/jwks.json)

type JWTKeyManager struct {
    privateKey *rsa.PrivateKey
    publicKey  *rsa.PublicKey
    keyID      string  // "key-YYYY-MM" for rotation
}

func (m *JWTKeyManager) Sign(claims jwt.Claims) (string, error) {
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    token.Header["kid"] = m.keyID
    return token.SignedString(m.privateKey)
}

func (m *JWTKeyManager) Parse(tokenString string) (*JWTClaims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{},
        func(token *jwt.Token) (interface{}, error) {
            if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
            }
            return m.publicKey, nil
        })
    if err != nil { return nil, err }
    return token.Claims.(*JWTClaims), nil
}

// JWKS endpoint for public key distribution
func (m *JWTKeyManager) JWKS() *jose.JSONWebKeySet {
    return &jose.JSONWebKeySet{
        Keys: []jose.JSONWebKey{{
            Key:       m.publicKey,
            KeyID:     m.keyID,
            Algorithm: "RS256",
            Use:       "sig",
        }},
    }
}
```

---

## 4. Refresh Token Rotation

### 4.1 Token Family Security

```
Initial Login:
  access_token_1 + refresh_token_1
  session: {id=S1, refresh_hash=H(RT1), family=F1, revoked_at=nil}

Refresh (normal):
  Input: refresh_token_1
  → Find session by H(RT1): S1, family=F1, not revoked
  → Revoke S1 (revoked_at=now)
  → Create S2: {refresh_hash=H(RT2), family=F1, revoked_at=nil}
  → Issue access_token_2 + refresh_token_2

REUSE ATTACK (refresh_token_1 used again after rotation):
  Input: refresh_token_1
  → Find session by H(RT1): S1, REVOKED (revoked_at != nil)
  → 🚨 SECURITY: Revoke ALL sessions in family F1 (S2 also revoked)
  → Log: token.refresh.reuse_detected
  → Return: 401 ErrRefreshTokenReuse
  → User must re-login
```

---

## 5. Argon2id Configuration

```go
// crypto/argon2.go

// Argon2id parameters (OWASP recommended 2024)
const (
    Memory      = 64 * 1024  // 64 MB
    Iterations  = 3
    Parallelism = 2
    SaltLength  = 16
    KeyLength   = 32
)

// Timing: ~200ms on average hardware
// This is intentional — makes brute force attacks 200ms/attempt

type Argon2Hasher struct{}

func (h *Argon2Hasher) Hash(password string) (string, error) {
    salt := make([]byte, SaltLength)
    if _, err := rand.Read(salt); err != nil { return "", err }
    
    hash := argon2.IDKey([]byte(password), salt, Iterations, Memory, Parallelism, KeyLength)
    
    // PHC format: $argon2id$v=19$m=65536,t=3,p=2$<salt_b64>$<hash_b64>
    b64Salt := base64.RawStdEncoding.EncodeToString(salt)
    b64Hash := base64.RawStdEncoding.EncodeToString(hash)
    
    return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
        argon2.Version, Memory, Iterations, Parallelism, b64Salt, b64Hash), nil
}

func (h *Argon2Hasher) Verify(password, encoded string) (bool, error) {
    // Parse PHC format and verify
    params, salt, hash, err := decodeArgon2Hash(encoded)
    if err != nil { return false, err }
    
    computed := argon2.IDKey([]byte(password), salt,
        params.iterations, params.memory, params.parallelism, params.keyLength)
    
    return subtle.ConstantTimeCompare(hash, computed) == 1, nil
}
```

---

## 6. Account Lockout Design

```go
// Lockout policy:
// - 5 failed attempts within a session → lock account (is_active=false)
// - Auto-unlock sau 15 phút (background job) OR manual admin unlock
// - Reset counter on successful login

// Background job: auto-unlock every 15 minutes
func (uc *AutoUnlockUseCase) Execute(ctx context.Context) error {
    // Find users locked > 15 minutes ago
    // (Track locked_at timestamp separately)
    users, _ := uc.userRepo.FindLockedBefore(ctx, time.Now().Add(-15*time.Minute))
    
    for _, u := range users {
        u.IsActive = true
        u.FailedLoginAttempts = 0
        uc.userRepo.Update(ctx, u)
        
        uc.auditRepo.Log(ctx, &AuditEntry{
            UserID: u.ID,
            Event:  "user.account.auto_unlocked",
        })
    }
    return nil
}
```

---

## 7. API Key Design

### 7.1 Key Format

```
Full key (shown once): ovs_4xKmNpQvR8sT7wYzA2bCdEfGhIjK
                        ─┬─ ──────────────────────────────
                         │  32 bytes base58 encoded ≈ 43 chars
                         └─ prefix "ovs_"

Prefix for display: ovs_4xKmNpQv (first 12 chars including "ovs_")
Stored: SHA-256(full_key) — never store plain key

Lookup: client sends full key → 
        server takes first 12 chars as prefix → 
        finds api_keys WHERE prefix = "ovs_4xKmNpQv" →
        constant-time compare SHA-256(input) == stored_key_hash
```

### 7.2 Scoped Permissions

```go
// API key permissions = subset of user's permissions
// Can NOT escalate beyond user's role

// Valid API key creation for a "user" role:
// User has: [scan:create, scan:read, finding:write, ...]
// API key can have: [scan:read]  (subset) ✅
// API key CANNOT have: [user:manage]  (not in user's permissions) ❌

func validateKeyPermissions(userPerms, requestedPerms []string) error {
    userPermSet := make(map[string]bool)
    for _, p := range userPerms {
        userPermSet[p] = true
    }
    
    for _, p := range requestedPerms {
        if !userPermSet[p] {
            return fmt.Errorf("permission escalation: %s not in user's permissions", p)
        }
    }
    return nil
}
```

---

## 8. TOTP MFA Design

### 8.1 Setup Flow

```
Step 1: POST /auth/mfa/setup
  → Generate TOTP secret (20 bytes random)
  → AES-256-GCM encrypt secret
  → Store encrypted secret in user.mfa_totp_secret
  → Return: {secret, qr_url (otpauth://...), backup_codes[8]}
  → mfa_enabled = FALSE (not yet confirmed)

Step 2: User scans QR code in authenticator app

Step 3: POST /auth/mfa/confirm  {code: "123456"}
  → Decrypt stored secret
  → TOTP.Validate(code, secret, window=1)  // ±30s tolerance
  → If valid: mfa_enabled = TRUE
  → Store hashed backup codes (one-time use)

Step 4: Future logins require MFA code
  POST /auth/login {email, password, mfa_code}
```

### 8.2 Backup Codes

```go
func generateBackupCodes() []string {
    codes := make([]string, 8)
    for i := range codes {
        b := make([]byte, 5)
        rand.Read(b)
        // Format: XXXXX-XXXXX (10 chars, hyphen-separated)
        codes[i] = fmt.Sprintf("%05X-%05X", 
            binary.BigEndian.Uint32(b[:4]), 
            binary.BigEndian.Uint16(b[3:5]))
    }
    return codes
}

// Backup codes stored as SHA-256 hashes, one per row
// Marked as used after consumption (one-time use)
```

---

## 9. OAuth2 Integration

### 9.1 Google OAuth2 Flow

```
1. GET /auth/oauth/google
   → Redirect to: https://accounts.google.com/o/oauth2/v2/auth
     ?client_id=...&redirect_uri=...&scope=email,profile
     &state=<random_csrf_token>  (stored in session)

2. Google → Callback: GET /auth/oauth/google/callback?code=...&state=...
   → Verify state (CSRF protection)
   → Exchange code for access_token
   → GET https://www.googleapis.com/oauth2/v3/userinfo
   → Upsert user: {email, username, auth_provider=google}
   → Create session
   → Return: {access_token, refresh_token}
```

### 9.2 Account Linking Strategy

```go
// Upsert strategy:
// 1. Find user by email (from OAuth provider)
// 2. If not found: create new user (IsVerified=true for OAuth)
// 3. If found with different auth_provider:
//    → Link OAuth account to existing user (oauth_accounts table)
//    → User can now login via multiple providers
// 4. Update oauth_accounts with refreshed access_token

func upsertOAuthUser(ctx context.Context, provider, email, providerID, name string) (*User, error) {
    // Try find by provider+provider_id first
    account, _ := oauthRepo.FindByProvider(ctx, provider, providerID)
    if account != nil {
        return userRepo.FindByID(ctx, account.UserID)
    }
    
    // Try find by email
    user, err := userRepo.FindByEmail(ctx, email)
    if err != nil {
        // Create new user
        user = &User{
            Email:        email,
            Username:     sanitizeUsername(name),
            Role:         RoleUser,
            AuthProvider: AuthProvider(provider),
            IsActive:     true,
            IsVerified:   true,  // OAuth verified email
        }
        userRepo.Save(ctx, user)
    }
    
    // Link OAuth account
    oauthRepo.Upsert(ctx, &OAuthAccount{
        UserID:     user.ID,
        Provider:   provider,
        ProviderID: providerID,
        Email:      email,
    })
    
    return user, nil
}
```

---

## 10. gRPC Hot Path Optimization

```go
// auth-service/internal/delivery/grpc/auth_server.go

// ValidateToken — must complete in < 1ms
func (s *AuthServer) ValidateToken(ctx context.Context, 
    req *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
    
    start := time.Now()
    defer func() {
        // Track latency metric
        s.metrics.ValidateTokenDuration.Observe(time.Since(start).Seconds())
    }()
    
    // Step 1: Parse JWT — CPU only, no I/O
    claims, err := s.jwtManager.Parse(req.Token)
    if err != nil {
        return &authpb.ValidateTokenResponse{Valid: false}, nil
    }
    
    // Step 2: Check expiry (in claims, no I/O)
    if !claims.ExpiresAt.After(time.Now()) {
        return &authpb.ValidateTokenResponse{Valid: false}, nil
    }
    
    // Step 3: Redis JTI check (O(1), single network call)
    exists, err := s.jwtCache.CheckJTI(ctx, claims.ID)
    if err != nil || !exists {
        return &authpb.ValidateTokenResponse{Valid: false}, nil
    }
    
    return &authpb.ValidateTokenResponse{
        Valid:       true,
        UserId:      claims.Subject,
        Role:        claims.Role,
        Permissions: claims.Permissions,
        ExpiresAt:   claims.ExpiresAt.Unix(),
    }, nil
}

// ValidateAPIKey — slightly slower (DB lookup for prefix)
func (s *AuthServer) ValidateAPIKey(ctx context.Context,
    req *authpb.ValidateAPIKeyRequest) (*authpb.ValidateAPIKeyResponse, error) {
    
    key := req.Key
    if !strings.HasPrefix(key, "ovs_") {
        return &authpb.ValidateAPIKeyResponse{Valid: false}, nil
    }
    
    prefix := key[:12]  // "ovs_" + 8 chars
    
    // DB lookup by prefix (indexed)
    apiKey, err := s.apiKeyRepo.FindByPrefix(ctx, prefix)
    if err != nil { return &authpb.ValidateAPIKeyResponse{Valid: false}, nil }
    
    // Check revocation + expiry
    if apiKey.RevokedAt != nil { return &authpb.ValidateAPIKeyResponse{Valid: false}, nil }
    if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
        return &authpb.ValidateAPIKeyResponse{Valid: false}, nil
    }
    
    // Constant-time compare (prevent timing attacks)
    inputHash := sha256Hex(key)
    if subtle.ConstantTimeCompare([]byte(inputHash), []byte(apiKey.KeyHash)) != 1 {
        return &authpb.ValidateAPIKeyResponse{Valid: false}, nil
    }
    
    // Update last_used_at (async, non-blocking)
    go s.apiKeyRepo.UpdateLastUsed(context.Background(), apiKey.ID)
    
    return &authpb.ValidateAPIKeyResponse{
        Valid:       true,
        UserId:      apiKey.UserID.String(),
        Permissions: apiKey.Permissions,
        KeyId:       apiKey.ID.String(),
    }, nil
}
```

---

## 11. Database Schema

```sql
-- migrations/001_create_auth_tables.sql

CREATE TABLE users (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email                 VARCHAR(255) UNIQUE NOT NULL,
    username              VARCHAR(50) UNIQUE NOT NULL,
    hashed_password       VARCHAR(255),        -- NULL for OAuth-only
    role                  VARCHAR(20) NOT NULL DEFAULT 'user'
                          CHECK (role IN ('admin','user','readonly','agent')),
    auth_provider         VARCHAR(20) NOT NULL DEFAULT 'local'
                          CHECK (auth_provider IN ('local','google','github')),
    mfa_enabled           BOOLEAN NOT NULL DEFAULT FALSE,
    mfa_totp_secret       TEXT,               -- AES-256-GCM encrypted
    mfa_backup_codes      TEXT[],             -- SHA-256 hashed backup codes
    is_active             BOOLEAN NOT NULL DEFAULT TRUE,
    is_verified           BOOLEAN NOT NULL DEFAULT FALSE,
    failed_login_attempts INT NOT NULL DEFAULT 0,
    locked_at             TIMESTAMPTZ,        -- When account was locked
    last_login_at         TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email    ON users(email);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_locked   ON users(locked_at) WHERE locked_at IS NOT NULL;

CREATE TABLE sessions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(64) NOT NULL UNIQUE,  -- SHA-256
    token_family       UUID NOT NULL,
    ip_address         INET,
    user_agent         TEXT,
    expires_at         TIMESTAMPTZ NOT NULL,
    revoked_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user   ON sessions(user_id);
CREATE INDEX idx_sessions_family ON sessions(token_family);
CREATE INDEX idx_sessions_hash   ON sessions(refresh_token_hash);

CREATE TABLE oauth_accounts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider      VARCHAR(20) NOT NULL CHECK (provider IN ('google','github')),
    provider_id   VARCHAR(255) NOT NULL,
    email         VARCHAR(255),
    access_token  TEXT,         -- AES-256-GCM encrypted
    refresh_token TEXT,         -- AES-256-GCM encrypted
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, provider_id)
);

CREATE INDEX idx_oauth_user ON oauth_accounts(user_id);

CREATE TABLE api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         VARCHAR(100) NOT NULL,
    key_hash     VARCHAR(64) NOT NULL UNIQUE,   -- SHA-256 of full key
    prefix       VARCHAR(12) NOT NULL,           -- "ovs_" + 8 chars
    permissions  TEXT[] NOT NULL DEFAULT '{}',
    last_used_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_keys_prefix ON api_keys(prefix);  -- Lookup path
CREATE INDEX idx_api_keys_user   ON api_keys(user_id);

CREATE TABLE audit_log (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id),
    event      VARCHAR(50) NOT NULL,
    -- Events: user.registered, user.login.success, user.login.failure,
    --         user.logout, user.locked, user.unlocked,
    --         token.refresh, token.refresh.reuse_detected,
    --         api_key.created, api_key.revoked,
    --         mfa.enabled, mfa.disabled, oauth.login
    ip_address INET,
    user_agent TEXT,
    metadata   JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_user  ON audit_log(user_id, created_at DESC);
CREATE INDEX idx_audit_event ON audit_log(event, created_at DESC);
```

---

## 12. Configuration

```yaml
# config/config.yaml
server:
  grpc_port: 50051
  http_port: 8051

database:
  host: "${DB_HOST}"
  port: 5432
  name: "auth_service"
  user: "${DB_USER}"
  password: "${DB_PASSWORD}"

redis:
  url: "${REDIS_URL}"
  jwt_ttl_buffer: "60s"  # Extra TTL buffer for clock skew

jwt:
  algorithm: "RS256"
  access_token_ttl: "15m"
  private_key_path: "/secrets/jwt_private.pem"
  public_key_path: "/secrets/jwt_public.pem"
  issuer: "openvulnscan-auth"
  audience: "openvulnscan-api"

argon2id:
  memory: 65536      # 64MB
  iterations: 3
  parallelism: 2
  salt_length: 16
  key_length: 32

account_lockout:
  max_failed_attempts: 5
  auto_unlock_duration: "15m"

mfa:
  totp_issuer: "OpenVulnScan"
  totp_window: 1       # ±1 period tolerance
  backup_codes: 8

oauth2:
  google:
    client_id: "${GOOGLE_CLIENT_ID}"
    client_secret: "${GOOGLE_CLIENT_SECRET}"
    callback_url: "${BASE_URL}/auth/oauth/google/callback"
  github:
    client_id: "${GITHUB_CLIENT_ID}"
    client_secret: "${GITHUB_CLIENT_SECRET}"
    callback_url: "${BASE_URL}/auth/oauth/github/callback"

encryption:
  totp_key: "${TOTP_ENCRYPTION_KEY}"    # 32-byte AES key (hex)
  oauth_token_key: "${OAUTH_ENC_KEY}"   # 32-byte AES key (hex)

nats:
  url: "${NATS_URL}"

logging:
  level: "info"
  format: "json"
```

---

## 13. Security Hardening Checklist

| Concern | Solution |
|---------|---------|
| Timing attack on password verify | `subtle.ConstantTimeCompare` in Argon2 verify |
| Timing attack on API key lookup | Constant-time compare after prefix DB lookup |
| Token replay | JTI in Redis with exact TTL |
| Refresh token theft | Token family revocation on reuse detection |
| TOTP secret leak | AES-256-GCM encryption before storage |
| OAuth CSRF | Random state parameter, verified in callback |
| Brute force | Account lockout after 5 failures, Argon2 slowness |
| Enumeration | Same error message for wrong email/password |
| SQL injection | Parameterized queries via sqlx/pgx |
| OAuth token leak | AES encrypted access/refresh tokens in DB |

---

## 14. Implementation Roadmap

### Phase 1 — Core Auth (Sprint 1)
- [ ] Database migrations
- [ ] User entity + Argon2id password hashing
- [ ] Register use case
- [ ] Login use case (without MFA)
- [ ] JWT RS256 signing/validation
- [ ] Redis JTI management
- [ ] ValidateToken gRPC (hot path)

### Phase 2 — Token Management (Sprint 2)
- [ ] RefreshToken rotation use case
- [ ] Logout use case (JTI revocation)
- [ ] Session repository
- [ ] Account lockout (5 failed → lock, 15min auto-unlock)
- [ ] ValidateAPIKey gRPC
- [ ] API key CRUD

### Phase 3 — MFA + OAuth (Sprint 3)
- [ ] TOTP setup/confirm/disable
- [ ] Backup codes
- [ ] Login with MFA code
- [ ] Google OAuth2 callback
- [ ] GitHub OAuth2 callback
- [ ] Audit logging

### Phase 4 — Admin + Polish (Sprint 4)
- [ ] Admin user management (list, role change, lock/unlock)
- [ ] JWKS endpoint for public key distribution
- [ ] Rate limiting (prevent brute force)
- [ ] Prometheus metrics
- [ ] Integration tests

---

## 15. Acceptance Criteria Mapping

| Criterion | Implementation |
|-----------|---------------|
| Password < 12 chars → 400 | `RegisterUseCase.Execute()` validation |
| Login success → JWT (RS256) + refresh token | `LoginUseCase.Execute()` |
| 5 failed logins → account locked | `FailedLoginAttempts++` → `IsActive=false` |
| MFA enabled + no code → 400 "MFA required" | `ErrMFARequired` check |
| Refresh → new access + refresh (old revoked) | `RefreshTokenUseCase` + `sessionRepo.Revoke()` |
| Revoked refresh token reuse → 401 + family revoked | `session.RevokedAt != nil` check |
| ValidateToken gRPC → Redis only, no DB | `jwtCache.CheckJTI()` only |
| JTI not in Redis → 401 | `exists == 0` → `ErrTokenRevoked` |
| `POST /auth/api-keys` → full key ONCE only | Return `plainKey` in response only |
| API key lookup → prefix + constant-time compare | `subtle.ConstantTimeCompare` |
| Agent `Bearer ovs_xxx` → ValidateAPIKey gRPC | `strings.HasPrefix(key, "ovs_")` |
| MFA setup → `otpauth://` QR URL | `totp.Generate().URL()` |
| TOTP confirm → `mfa_enabled=true` | `ConfirmTOTP()` |
| Google OAuth callback → upsert user | `upsertOAuthUser()` |
| GitHub OAuth callback → upsert user | Same pattern |
| Audit log entries | `auditRepo.Log()` in each use case |
