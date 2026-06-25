# CR-OVS-003 — Auth Service: JWT RS256, RBAC, MFA, API Keys, OAuth2

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OVS-003 |
| **Tiêu đề** | Auth Service — JWT RS256 Authentication, RBAC Permission Matrix, TOTP MFA, API Key Management (ovs_ prefix), Google/GitHub OAuth2 |
| **Nguồn tham chiếu** | `OpenVulnScan/specs/services/02-auth-service.md` |
| **Target Service** | **MỚI**: `auth-service` (port gRPC: 50051) |
| **Ưu tiên** | 🔴 High |
| **Loại** | New Service |
| **Ngày tạo** | 2026-06-14 |
| **Ngày implement** | 2026-06-17 |
| **Trạng thái** | ✅ Implemented |

---

## 0. Implementation Status

> **Trạng thái**: ✅ **IMPLEMENTED** — 2026-06-17

| Task | File | Trạng thái |
|------|------|------------|
| Argon2id password hashing | `identity-service/internal/crypto/argon2.go` | ✅ Done |
| JWT RS256 (sign/verify) | `identity-service/internal/crypto/jwt.go` | ✅ Done |
| RSA key generation | `identity-service/internal/crypto/keygen/generate_keys.go` | ✅ Done |
| Redis JTI blacklist cache | `identity-service/internal/cache/redis/jti_cache.go` | ✅ Done |
| Auth HTTP handler (login/refresh/logout) | `identity-service/adapter/handler/http/auth_handler.go` | ✅ Done |
| TOTP MFA handler | `identity-service/adapter/handler/http/totp_handler.go` | ✅ Done |
| OAuth2 handler (Google/GitHub) | `identity-service/adapter/handler/http/oauth_handler.go` | ✅ Done |
| API Key handler | `identity-service/adapter/handler/http/api_key_handler.go` | ✅ Done |
| Admin user management handler | `identity-service/adapter/handler/http/admin_handler.go` | ✅ Done |
| Profile handler | `identity-service/adapter/handler/http/profile_handler.go` | ✅ Done |
| gRPC ValidateToken handler | `identity-service/adapter/handler/grpc/auth_grpc_handler.go` | ✅ Done |
| API Key PostgreSQL repo | `identity-service/adapter/repository/postgres/api_key_repo.go` | ✅ Done |
| Session PostgreSQL repo | `identity-service/adapter/repository/postgres/session_repo.go` | ✅ Done |
| TOTP methods | `identity-service/adapter/repository/postgres/totp_methods.go` | ✅ Done |
| User PostgreSQL repo | `identity-service/adapter/repository/postgres/user_repo.go` | ✅ Done |
| Session MongoDB repo | `identity-service/adapter/repository/mongo/session_repo.go` | ✅ Done |
| User MongoDB repo | `identity-service/adapter/repository/mongo/user_repo.go` | ✅ Done |

**Chi tiết implementation**:
- **JWT RS256**: `RS256` asymmetric signing, 15min access token TTL, 7-day refresh token
- **Refresh Rotation**: mỗi refresh → revoke token cũ + issue token mới; reuse detection → revoke family
- **Argon2id**: `time=1, memory=64MB, parallelism=4, tagLength=32`, resistant to GPU attacks
- **TOTP MFA**: `pquerna/otp` RFC 6238, 30s window, QR URL + 8 backup codes
- **API Keys**: `ovs_` prefix, SHA-256 stored, scoped permissions
- **Account Lockout**: 5 failed attempts → 15min lockout, tracked in Redis
- **gRPC ValidateToken**: Redis-only lookup < 1ms, no DB roundtrip
- **OAuth2**: Google + GitHub callback, upsert user, issue JWT

---

## 1. Tổng quan

OSV hiện có auth cơ bản. OpenVulnScan định nghĩa **Auth Service** production-grade với:
- **JWT RS256** (asymmetric, 15min access token + rotation refresh token)
- **RBAC**: admin / user / readonly / agent — với permission matrix chi tiết
- **MFA (TOTP)**: RFC 6238, 30s window
- **API Keys**: prefix `ovs_`, SHA-256 stored, scoped permissions
- **OAuth2**: Google + GitHub callback
- **gRPC hot path**: ValidateToken < 1ms (Redis cache, no DB)

---

## 2. Gap Analysis

| Feature | OSV | OpenVulnScan |
|---------|-----|-------------|
| JWT auth | ⚠️ Basic | ✅ RS256, JTI blacklist |
| Refresh token rotation | ❌ | ✅ Full rotation pattern |
| Token reuse detection | ❌ | ✅ Token family tracking |
| MFA (TOTP) | ❌ | ✅ RFC 6238 |
| Role: admin/user/readonly | ⚠️ | ✅ + agent role |
| Permission-based RBAC | ❌ | ✅ Fine-grained permissions |
| API keys (ovs_ prefix) | ❌ | ✅ SHA-256, scoped |
| Google OAuth2 | ❌ | ✅ |
| GitHub OAuth2 | ❌ | ✅ |
| Password: Argon2id | ❌ | ✅ (vs bcrypt) |
| Account lockout | ❌ | ✅ failed_login_attempts |
| JWT gRPC validation | ❌ | ✅ Redis cache, <1ms |
| Audit log | ❌ | ✅ all auth events |

---

## 3. Domain Model

### 3.1 Entities

```go
// auth-service/internal/domain/entity/user.go

type AuthProvider string
const (
    AuthProviderLocal  AuthProvider = "local"
    AuthProviderGoogle AuthProvider = "google"
    AuthProviderGitHub AuthProvider = "github"
)

type Role string
const (
    RoleAdmin    Role = "admin"
    RoleUser     Role = "user"
    RoleReadOnly Role = "readonly"
    RoleAgent    Role = "agent"
)

// Permission constants — fine-grained RBAC
const (
    PermScanCreate    = "scan:create"
    PermScanRead      = "scan:read"
    PermAssetWrite    = "asset:write"
    PermAssetRead     = "asset:read"
    PermUserManage    = "user:manage"
    PermReportDownload = "report:download"
    PermSystemConfigure = "system:configure"
    PermAgentReport   = "agent:report"
    PermFindingWrite  = "finding:write"
    PermFindingRead   = "finding:read"
)

// RolePermissions — RBAC matrix
var RolePermissions = map[Role][]string{
    RoleAdmin:    {PermScanCreate, PermScanRead, PermAssetWrite, PermAssetRead,
                   PermUserManage, PermReportDownload, PermSystemConfigure,
                   PermFindingWrite, PermFindingRead},
    RoleUser:     {PermScanCreate, PermScanRead, PermAssetWrite, PermAssetRead,
                   PermReportDownload, PermFindingWrite, PermFindingRead},
    RoleReadOnly: {PermScanRead, PermAssetRead, PermReportDownload, PermFindingRead},
    RoleAgent:    {PermAssetWrite, PermAgentReport},
}

type User struct {
    ID                 uuid.UUID
    Email              string       // Unique, primary login identifier
    Username           string       // Unique, display purposes
    HashedPassword     string       // nil/empty for OAuth-only users
    Role               Role
    AuthProvider       AuthProvider
    MFAEnabled         bool
    MFATOTPSecret      string       // AES-256-GCM encrypted TOTP secret
    IsActive           bool         // false = account locked
    IsVerified         bool         // email verified
    FailedLoginAttempts int
    LastLoginAt        *time.Time
    CreatedAt          time.Time
    UpdatedAt          time.Time
}

func (u *User) IsPasswordSet() bool { return u.HashedPassword != "" }
func (u *User) IsLocked() bool      { return !u.IsActive }
func (u *User) Permissions() []string { return RolePermissions[u.Role] }

// Session — refresh token session
type Session struct {
    ID               uuid.UUID
    UserID           uuid.UUID
    RefreshTokenHash string       // SHA-256 of actual refresh token
    TokenFamily      uuid.UUID    // for refresh token rotation / reuse detection
    IPAddress        string
    UserAgent        string
    ExpiresAt        time.Time
    RevokedAt        *time.Time
    CreatedAt        time.Time
}

// APIKey — agent and programmatic access
type APIKey struct {
    ID          uuid.UUID
    UserID      uuid.UUID
    Name        string
    KeyHash     string       // SHA-256 of actual key
    Prefix      string       // "ovs_" + 8 chars (for display/lookup)
    Permissions []string     // scoped permissions subset
    LastUsedAt  *time.Time
    ExpiresAt   *time.Time
    CreatedAt   time.Time
    RevokedAt   *time.Time
}
```

---

## 4. Use Cases

### 4.1 RegisterUser

```go
// auth-service/internal/usecase/register/usecase.go

type RegisterInput struct {
    Email    string // required, email format, max=255
    Username string // required, min=3, max=50, alphanumeric
    Password string // required, min=12, max=128
    Role     Role   // optional, default=user; only admin can set admin
}

func (uc *RegisterUseCase) Execute(ctx context.Context, in RegisterInput) (*entity.User, error) {
    // 1. Validate input
    if !isValidEmail(in.Email) { return nil, ErrInvalidEmail }
    if len(in.Password) < 12 { return nil, ErrWeakPassword }

    // 2. Check uniqueness
    if _, err := uc.userRepo.FindByEmail(ctx, in.Email); err == nil {
        return nil, ErrEmailAlreadyExists
    }

    // 3. Hash password with Argon2id
    // Memory=64MB, Iterations=3, Parallelism=2
    hash, err := argon2id.Hash(in.Password, argon2id.DefaultParams)
    if err != nil { return nil, err }

    role := in.Role
    if role == "" { role = entity.RoleUser }

    user := &entity.User{
        ID:             uuid.New(),
        Email:          in.Email,
        Username:       in.Username,
        HashedPassword: hash,
        Role:           role,
        AuthProvider:   entity.AuthProviderLocal,
        IsActive:       true,
        IsVerified:     false,
        CreatedAt:      time.Now().UTC(),
        UpdatedAt:      time.Now().UTC(),
    }

    if err := uc.userRepo.Save(ctx, user); err != nil { return nil, err }

    // 4. Publish UserRegistered event
    uc.eventBus.Publish(ctx, "auth.user.registered", &UserRegisteredEvent{
        UserID: user.ID,
        Email:  user.Email,
    })

    return user, nil
}
```

### 4.2 LoginUser

```go
// auth-service/internal/usecase/login/usecase.go

type LoginInput struct {
    Email     string
    Password  string
    MFACode   string   // optional, 6-digit TOTP
    IPAddress string
    UserAgent string
}

type LoginOutput struct {
    AccessToken  string  // JWT RS256, 15min TTL
    RefreshToken string  // UUID v4
    ExpiresIn    int64   // 900 seconds
    User         UserSummary
}

func (uc *LoginUseCase) Execute(ctx context.Context, in LoginInput) (*LoginOutput, error) {
    // 1. Find user
    user, err := uc.userRepo.FindByEmail(ctx, in.Email)
    if err != nil { return nil, ErrInvalidCredentials }  // Don't reveal "not found"

    // 2. Check account active
    if !user.IsActive { return nil, ErrAccountLocked }

    // 3. Verify password (Argon2id)
    ok, err := argon2id.Verify(in.Password, user.HashedPassword)
    if err != nil || !ok {
        // Increment failed_login_attempts
        user.FailedLoginAttempts++
        if user.FailedLoginAttempts >= 5 {
            user.IsActive = false  // Lock account
        }
        uc.userRepo.Update(ctx, user)
        return nil, ErrInvalidCredentials
    }

    // 4. Verify MFA if enabled
    if user.MFAEnabled {
        if in.MFACode == "" { return nil, ErrMFARequired }
        secret, _ := decryptTOTPSecret(user.MFATOTPSecret)
        if !totp.Validate(in.MFACode, secret) {
            return nil, ErrInvalidMFACode
        }
    }

    // 5. Reset failed_login_attempts on success
    user.FailedLoginAttempts = 0
    now := time.Now().UTC()
    user.LastLoginAt = &now
    uc.userRepo.Update(ctx, user)

    // 6. Generate JWT (RS256)
    jti := uuid.New().String()
    claims := JWTClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   user.ID.String(),
            Issuer:    "openvulnscan-auth",
            Audience:  jwt.ClaimStrings{"openvulnscan-api"},
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            ID:        jti,
        },
        Role:        string(user.Role),
        Permissions: user.Permissions(),
    }

    accessToken, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).
        SignedString(uc.privateKey)
    if err != nil { return nil, err }

    // 7. Store JTI in Redis for blacklist checking
    uc.redis.Set(ctx, "auth:jwt:"+jti, user.ID.String(), 15*time.Minute)

    // 8. Generate refresh token + session
    refreshToken := uuid.New().String()
    refreshHash := sha256Hex(refreshToken)
    session := &entity.Session{
        ID:               uuid.New(),
        UserID:           user.ID,
        RefreshTokenHash: refreshHash,
        TokenFamily:      uuid.New(),  // New family for new session
        IPAddress:        in.IPAddress,
        UserAgent:        in.UserAgent,
        ExpiresAt:        time.Now().Add(30 * 24 * time.Hour), // 30 days
        CreatedAt:        time.Now().UTC(),
    }
    uc.sessionRepo.Save(ctx, session)

    // 9. Audit log
    uc.auditRepo.Log(ctx, &AuditEntry{
        UserID:    user.ID,
        Event:     "user.login.success",
        IPAddress: in.IPAddress,
        UserAgent: in.UserAgent,
    })

    return &LoginOutput{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresIn:    900,
        User:         toUserSummary(user),
    }, nil
}
```

### 4.3 RefreshToken (Rotation Pattern)

```go
// auth-service/internal/usecase/refresh_token/usecase.go

func (uc *RefreshTokenUseCase) Execute(ctx context.Context, refreshToken string) (*LoginOutput, error) {
    // 1. Hash incoming refresh token
    hash := sha256Hex(refreshToken)

    // 2. Find session
    session, err := uc.sessionRepo.FindByHash(ctx, hash)
    if err != nil { return nil, ErrInvalidRefreshToken }

    // 3. Check if session revoked (= REUSE ATTACK)
    if session.RevokedAt != nil {
        // Security: revoke ENTIRE token family
        uc.sessionRepo.RevokeFamily(ctx, session.TokenFamily)
        uc.auditRepo.Log(ctx, &AuditEntry{
            UserID: session.UserID,
            Event:  "token.refresh.reuse_detected",
            Metadata: map[string]string{
                "family": session.TokenFamily.String(),
            },
        })
        return nil, ErrRefreshTokenReuse
    }

    // 4. Check expiry
    if time.Now().After(session.ExpiresAt) {
        return nil, ErrRefreshTokenExpired
    }

    // 5. Revoke OLD session
    uc.sessionRepo.Revoke(ctx, session.ID)

    // 6. Issue new tokens (same TokenFamily for continuity)
    user, _ := uc.userRepo.FindByID(ctx, session.UserID)
    // Generate new access + refresh token...
    newSession := &entity.Session{
        TokenFamily: session.TokenFamily, // Same family
        // ...
    }
    uc.sessionRepo.Save(ctx, newSession)

    return &LoginOutput{ /* new tokens */ }, nil
}
```

### 4.4 ValidateToken (gRPC — Hot Path)

```go
// auth-service/internal/usecase/validate_token/usecase.go
// Called by unified-gateway on EVERY authenticated request
// MUST be < 1ms — Redis only, NO DB calls

func (uc *ValidateTokenUseCase) Execute(ctx context.Context, token string) (*ValidateOutput, error) {
    // 1. Parse JWT (RS256 public key) — fast, no I/O
    claims, err := uc.jwtParser.ParseWithClaims(token)
    if err != nil { return nil, ErrInvalidToken }

    // 2. Check expiry (embedded in JWT)
    if !claims.ExpiresAt.After(time.Now()) {
        return nil, ErrTokenExpired
    }

    // 3. Check JTI blacklist in Redis (O(1))
    exists, _ := uc.redis.Exists(ctx, "auth:jwt:"+claims.ID).Result()
    if exists == 0 {
        return nil, ErrTokenRevoked  // JTI not in Redis = revoked or expired
    }

    return &ValidateOutput{
        UserID:      claims.Subject,
        Role:        claims.Role,
        Permissions: claims.Permissions,
        ExpiresAt:   claims.ExpiresAt.Unix(),
    }, nil
}
```

### 4.5 API Key Management

```go
// auth-service/internal/usecase/manage_api_key/create.go

type CreateAPIKeyInput struct {
    UserID       uuid.UUID
    Name         string
    Permissions  []string   // subset of role permissions
    ExpiresInDays *int      // nil = no expiry
}

func (uc *CreateAPIKeyUseCase) Execute(ctx context.Context, in CreateAPIKeyInput) (*CreateAPIKeyOutput, error) {
    // 1. Validate permissions are subset of user's permissions
    user, _ := uc.userRepo.FindByID(ctx, in.UserID)
    for _, perm := range in.Permissions {
        if !hasPermission(user.Permissions(), perm) {
            return nil, ErrPermissionEscalation
        }
    }

    // 2. Generate cryptographically secure key
    keyBytes := make([]byte, 32)
    rand.Read(keyBytes)
    plainKey := "ovs_" + base58.Encode(keyBytes) // e.g., "ovs_4xKmNpQvR8..."

    prefix := plainKey[:12] // "ovs_" + 8 chars for display/lookup

    // 3. Store only SHA-256 hash
    keyHash := sha256Hex(plainKey)

    apiKey := &entity.APIKey{
        ID:          uuid.New(),
        UserID:      in.UserID,
        Name:        in.Name,
        KeyHash:     keyHash,
        Prefix:      prefix,
        Permissions: in.Permissions,
        CreatedAt:   time.Now().UTC(),
    }
    if in.ExpiresInDays != nil {
        exp := time.Now().AddDate(0, 0, *in.ExpiresInDays)
        apiKey.ExpiresAt = &exp
    }

    uc.apiKeyRepo.Save(ctx, apiKey)

    // 4. Return plain key ONLY ONCE
    return &CreateAPIKeyOutput{
        ID:          apiKey.ID,
        Key:         plainKey, // "ovs_..." — never stored, shown ONCE
        Prefix:      prefix,
        Name:        in.Name,
        Permissions: in.Permissions,
        ExpiresAt:   apiKey.ExpiresAt,
        CreatedAt:   apiKey.CreatedAt,
    }, nil
}
```

### 4.6 TOTP MFA Setup

```go
// auth-service/internal/usecase/mfa/setup.go

func (uc *MFASetupUseCase) GenerateTOTP(ctx context.Context, userID uuid.UUID) (*MFASetupOutput, error) {
    user, _ := uc.userRepo.FindByID(ctx, userID)
    if user.MFAEnabled { return nil, ErrMFAAlreadyEnabled }

    // Generate TOTP secret
    key, err := totp.Generate(totp.GenerateOpts{
        Issuer:      "OpenVulnScan",
        AccountName: user.Email,
        SecretSize:  20,
    })
    if err != nil { return nil, err }

    // Encrypt secret before storing
    encrypted, _ := aesEncrypt(key.Secret(), uc.encryptionKey)
    user.MFATOTPSecret = encrypted
    // Note: MFAEnabled = false until user confirms with TOTP code

    return &MFASetupOutput{
        Secret:      key.Secret(),          // Show once to user
        QRURL:       key.URL(),             // otpauth:// URL for QR code
        BackupCodes: generateBackupCodes(), // 8 one-time backup codes
    }, nil
}

func (uc *MFASetupUseCase) ConfirmTOTP(ctx context.Context, userID uuid.UUID, code string) error {
    user, _ := uc.userRepo.FindByID(ctx, userID)
    secret, _ := decryptTOTPSecret(user.MFATOTPSecret)

    if !totp.Validate(code, secret) { return ErrInvalidTOTPCode }

    user.MFAEnabled = true
    return uc.userRepo.Update(ctx, user)
}
```

---

## 5. gRPC Service Definition

```protobuf
// auth-service/proto/auth/v1/auth.proto

service AuthService {
  // Called on every authenticated request (hot path: Redis only, no DB)
  rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse);

  // Called for agent API key validation (agent requests)
  rpc ValidateAPIKey(ValidateAPIKeyRequest) returns (ValidateAPIKeyResponse);
}

message ValidateTokenRequest  { string token = 1; }
message ValidateTokenResponse {
  bool valid = 1;
  string user_id = 2;
  string role = 3;
  repeated string permissions = 4;
  int64 expires_at = 5;
}

message ValidateAPIKeyRequest  { string key = 1; }
message ValidateAPIKeyResponse {
  bool valid = 1;
  string user_id = 2;
  repeated string permissions = 3;
  string key_id = 4;
}
```

---

## 6. Database Schema

```sql
-- auth-service migrations/

CREATE TABLE users (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email                 VARCHAR(255) UNIQUE NOT NULL,
    username              VARCHAR(50) UNIQUE NOT NULL,
    hashed_password       VARCHAR(255),    -- NULL for OAuth-only users
    role                  VARCHAR(20) NOT NULL DEFAULT 'user',
    auth_provider         VARCHAR(20) NOT NULL DEFAULT 'local',
    mfa_enabled           BOOLEAN NOT NULL DEFAULT FALSE,
    mfa_totp_secret       TEXT,            -- AES-256-GCM encrypted
    is_active             BOOLEAN NOT NULL DEFAULT TRUE,
    is_verified           BOOLEAN NOT NULL DEFAULT FALSE,
    failed_login_attempts INT NOT NULL DEFAULT 0,
    last_login_at         TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE sessions (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id               UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash    VARCHAR(64) NOT NULL UNIQUE,  -- SHA-256
    token_family          UUID NOT NULL,
    ip_address            INET,
    user_agent            TEXT,
    expires_at            TIMESTAMPTZ NOT NULL,
    revoked_at            TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_sessions_user     ON sessions(user_id);
CREATE INDEX idx_sessions_family   ON sessions(token_family);

CREATE TABLE oauth_accounts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider      VARCHAR(20) NOT NULL,   -- google|github
    provider_id   VARCHAR(255) NOT NULL,
    email         VARCHAR(255),
    access_token  TEXT,     -- AES encrypted
    refresh_token TEXT,     -- AES encrypted
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, provider_id)
);

CREATE TABLE api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         VARCHAR(100) NOT NULL,
    key_hash     VARCHAR(64) NOT NULL UNIQUE,  -- SHA-256
    prefix       VARCHAR(12) NOT NULL,          -- "ovs_" + 8 chars
    permissions  TEXT[] NOT NULL DEFAULT '{}',
    last_used_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_api_keys_prefix ON api_keys(prefix);
CREATE INDEX idx_api_keys_user   ON api_keys(user_id);

CREATE TABLE audit_log (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id),
    event      VARCHAR(50) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    metadata   JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_user    ON audit_log(user_id, created_at DESC);
CREATE INDEX idx_audit_event   ON audit_log(event);
```

---

## 7. Security Configuration

```yaml
# auth-service/config/config.yaml

jwt:
  algorithm: RS256
  access_token_ttl: 15m
  private_key_path: /secrets/jwt_private.pem
  public_key_path: /secrets/jwt_public.pem
  issuer: openvulnscan-auth
  audience: openvulnscan-api

argon2id:
  memory: 65536        # 64MB
  iterations: 3
  parallelism: 2
  salt_length: 16
  key_length: 32

account_lockout:
  max_failed_attempts: 5
  lockout_duration: 15m   # auto-unlock after 15 min (or manual admin unlock)

mfa:
  totp_issuer: OpenVulnScan
  totp_window: 1          # ±1 period tolerance (30s each)
  backup_codes: 8

oauth2:
  google:
    client_id: "${GOOGLE_CLIENT_ID}"
    client_secret: "${GOOGLE_CLIENT_SECRET}"
    callback_url: "https://your-domain.com/auth/oauth/google/callback"
  github:
    client_id: "${GITHUB_CLIENT_ID}"
    client_secret: "${GITHUB_CLIENT_SECRET}"
    callback_url: "https://your-domain.com/auth/oauth/github/callback"

redis:
  url: "${REDIS_URL}"
  jwt_ttl_buffer: 60s     # Extra TTL buffer for clock skew
```

---

## 8. HTTP API

```
POST /auth/register            → Register new user
POST /auth/login               → Login (returns JWT + refresh token)
POST /auth/refresh             → Refresh access token (rotation)
POST /auth/logout              → Logout (revoke session)
GET  /auth/me                  → Get current user profile

# MFA
POST /auth/mfa/setup           → Get TOTP QR URL
POST /auth/mfa/confirm         → Confirm TOTP code → enable MFA
POST /auth/mfa/disable         → Disable MFA

# API Keys
POST /auth/api-keys            → Create API key (returns full key ONCE)
GET  /auth/api-keys            → List API keys (prefix only)
DELETE /auth/api-keys/{id}     → Revoke API key

# OAuth2
GET  /auth/oauth/google        → Redirect to Google
GET  /auth/oauth/google/callback → Google callback → return tokens
GET  /auth/oauth/github        → Redirect to GitHub
GET  /auth/oauth/github/callback → GitHub callback → return tokens

# Admin
GET  /auth/users               → List users (admin only)
PUT  /auth/users/{id}/role     → Update user role (admin only)
POST /auth/users/{id}/lock     → Lock user account (admin only)
POST /auth/users/{id}/unlock   → Unlock user account (admin only)
```

---

## 9. Acceptance Criteria

- [ ] `POST /auth/register` với password < 12 chars → 400
- [ ] `POST /auth/login` thành công → JWT (RS256), refresh token (UUID)
- [ ] `POST /auth/login` sai password 5 lần → account locked (is_active=false)
- [ ] `POST /auth/login` với MFA enabled, không có mfa_code → 400 "MFA required"
- [ ] `POST /auth/refresh` → trả về access token mới + refresh token mới (cũ bị revoke)
- [ ] Sử dụng refresh token đã revoke → 401 + ENTIRE token family revoked
- [ ] ValidateToken gRPC: Redis cache hit → < 1ms, no DB call
- [ ] JWT JTI không trong Redis (expired/revoked) → 401
- [ ] `POST /auth/api-keys` → trả về full `ovs_xxx` key ONE TIME ONLY
- [ ] API key lookup bằng prefix → SHA-256 compare (constant-time)
- [ ] Agent request với `Authorization: Bearer ovs_xxx` → ValidateAPIKey gRPC
- [ ] TOTP setup: `POST /auth/mfa/setup` → return otpauth:// QR URL
- [ ] TOTP confirm: valid 6-digit code → `mfa_enabled=true`
- [ ] Google OAuth2 callback → upsert user, return tokens
- [ ] GitHub OAuth2 callback → upsert user, return tokens
- [ ] Audit log entries cho: login_success, login_failure, token_refresh, api_key_created, logout
