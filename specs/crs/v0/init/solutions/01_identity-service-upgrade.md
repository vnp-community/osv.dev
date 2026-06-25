# identity-service — Upgrade Specification (Chỉ Thêm, Không Xóa)

> **Audit tại**: `services/identity-service/`
> **Trạng thái hiện tại**: ~70% complete
> **Ưu tiên**: P1
> **Nguyên tắc**: Mọi thay đổi chỉ THÊM file/package mới. Code hiện có GIỮ NGUYÊN.

---

## ✅ Implementation Status — 2026-06-13

> **Trạng thái cũ**: ~70% | **Trạng thái mới**: ~90% ✅
> **Build**: `go build ./...` PASSED

### Đã implement (Sprint 2):
- ✅ `usecase/totp/setup.go` — TOTP setup, QR code generation, pending state
- ✅ `usecase/totp/verify.go` — TOTP verify + MFA enable
- ✅ `usecase/totp/disable.go` — TOTP disable với password verify
- ✅ `adapter/repository/postgres/totp_methods.go` — UserRepo.StoreTOTPPending/ActivateTOTP
- ✅ `adapter/handler/http/totp_handler.go` — HTTP endpoints: setup/verify/disable
- ✅ `migrations/002_totp_pending.sql` — TOTP pending table

### Còn lại (Backlog P3):
- ⏳ `usecase/forgot_password/` + `usecase/reset_password/` — cần SMTP config
- ⏳ `usecase/verify_email/` — cần email template
- ⏳ Admin use cases (5 UCs) — cần role middleware

---


## 1. Những gì đã có — GIỮ NGUYÊN ✅

### Domain Layer
- `entity/user.go`: User entity đầy đủ — MFA, IsVerified, FailedLoginAttempts, LastLoginAt ✅
- `entity/session.go`: Session entity ✅
- `entity/api_key.go`: APIKey entity ✅
- `domain/error/errors.go`: 12 sentinel errors đầy đủ ✅
- `domain/identity/role.go`: Role entity ✅
- `domain/repository/repositories.go`: Repository interfaces ✅

### Infrastructure
- `infrastructure/jwt/token.go`: RS256 JWT với Claims (uid, role, perms) ✅
- `infrastructure/crypto/argon2id.go`: Argon2id (64MB, 3 iter, 2 parallel) ✅
- `infrastructure/crypto/apikey_totp.go`: TOTP validate ✅
- `infrastructure/oauth/google.go` + `github.go`: OAuth2 providers ✅
- `infrastructure/cache/token_cache.go`: Redis brute-force + token blacklist ✅

### Use Cases (7 UC) — Tất cả GIỮ NGUYÊN
- `usecase/login/login.go`: brute-force, MFA, session, RS256 JWT ✅
- `usecase/register/register.go`: Argon2id hash, default role "user" ✅
- `usecase/refresh_token/`: Token rotation ✅
- `usecase/logout/`: Token revoke ✅
- `usecase/validate_token/`: JWT validation ✅
- `usecase/oauth/oauth_callback.go`: Google + GitHub callback ✅
- `usecase/manage_api_key/manage_api_key.go`: CRUD API keys ✅

### Delivery — Tất cả GIỮ NGUYÊN
- `adapter/handler/http/router.go`: 16 routes ✅
- `adapter/handler/http/auth_handler.go` ✅
- `adapter/handler/http/oauth_handler.go` ✅
- `adapter/handler/http/api_key_handler.go` ✅
- `adapter/handler/grpc/auth_grpc_handler.go` ✅

### Repository Implementations — Tất cả GIỮ NGUYÊN
- `adapter/repository/postgres/user_repo.go`: pgx/v5 ✅ ← **PRIMARY**
- `adapter/repository/postgres/session_repo.go` ✅
- `adapter/repository/postgres/api_key_repo.go` ✅
- `adapter/repository/mongo/user_repo.go` ✅ ← **GIỮ** (alternative)
- `adapter/repository/mongo/session_repo.go` ✅ ← **GIỮ** (alternative)

### Cấu trúc hiện có nhưng chưa dùng — GIỮ, SẼ MỞ RỘNG
- `internal/infra/auth/`: Thư mục trống → sẽ **thêm** auth cache vào đây
- `internal/provider/local.go` + `chain.go` → sẽ **thêm** kết nối vào use cases
- `services/` (thư mục bổ sung) → **GIỮ** và expand nếu cần

---

## 2. Những gì cần THÊM (Gaps)

### 🔴 P0 — Thiếu: Wire Config cho Repository

**Vấn đề**: `cmd/server/main.go` có thể đang hardcode 1 repo backend. Cần thêm config để chọn.

**Thêm mới** (KHÔNG sửa code cũ):
```go
// internal/config/repo_config.go  ← NEW FILE
type RepoBackend string
const (
    RepoBackendPostgres RepoBackend = "postgres"
    RepoBackendMongo    RepoBackend = "mongo"
)

type Config struct {
    UserRepo    RepoBackend `env:"USER_REPO_BACKEND" default:"postgres"`
    SessionRepo RepoBackend `env:"SESSION_REPO_BACKEND" default:"postgres"`
}
```

```go
// cmd/server/main.go ← CHỈ THÊM logic chọn backend
// Giữ nguyên toàn bộ code hiện có, thêm switch ở chỗ wire repos:
switch cfg.UserRepo {
case "mongo":
    userRepo = mongo.NewUserRepo(mongoClient)
default:
    userRepo = postgres.NewUserRepo(pgPool)  // default = postgres (hiện tại)
}
```

### 🔴 P0 — Thiếu: NATS Event Publisher

Hiện tại sau register/oauth không có event nào publish. Notification-service và downstream cần events.

**Thêm mới**:
```
internal/infra/messaging/nats/
└── event_publisher.go   ← NEW: publish user events to NATS
```

```go
// internal/infra/messaging/nats/event_publisher.go
type IdentityEventPublisher struct {
    conn *nats.Conn
}

// Events to publish:
// user.registered       → notification-service (send welcome email)
// user.email_verified   → update user status
// user.password_changed → security alert
// user.mfa_enabled      → security confirmation
```

**Thêm vào** `usecase/register/register.go` (sau khi tạo user thành công):
```go
// Sau dòng "return &Response{...}"  — thêm publish event
// go publisher.Publish("user.registered", UserRegisteredEvent{ID: user.ID, Email: user.Email})
```

### 🟡 P1 — Thiếu: TOTP Management Use Cases + Handlers

**Thêm mới** (không sửa code totp cũ trong `infrastructure/crypto/apikey_totp.go`):
```
internal/usecase/totp/
├── setup.go        ← GenerateSecret() + QR code URL
├── verify.go       ← Confirm TOTP works before enabling MFA
└── disable.go      ← Disable TOTP (requires current password)
```

```go
// internal/usecase/totp/setup.go
type SetupTOTPRequest struct {
    UserID uuid.UUID
}

type SetupTOTPResponse struct {
    Secret     string  // base32 encoded
    QRCodeURL  string  // otpauth:// URI for QR generation
    ManualCode string  // backup manual entry code
}

// UseCase sử dụng crypto.GenerateTOTPSecret() từ infrastructure/crypto/
```

**Thêm HTTP handler** (không sửa router.go cũ, thêm routes mới):
```
adapter/handler/http/totp_handler.go   ← NEW
```

```go
// adapter/handler/http/router.go ← CHỈ THÊM 3 dòng route mới
// Giữ nguyên toàn bộ routes cũ
r.Post("/api/v1/auth/totp/setup", totpH.Setup)     // NEW
r.Post("/api/v1/auth/totp/verify", totpH.Verify)   // NEW
r.Delete("/api/v1/auth/totp", totpH.Disable)        // NEW
```

### 🟡 P1 — Thiếu: Password Reset Flow

**Thêm mới**:
```
internal/usecase/forgot_password/
└── forgot_password.go   ← Validate email, generate token, publish NATS event

internal/usecase/reset_password/
└── reset_password.go    ← Validate token, update password, revoke token

internal/infra/email/
└── smtp_sender.go       ← SMTP email sender (sử dụng trong reset flow)
```

**Migration** (thêm file mới, không sửa 001):
```sql
-- migrations/002_password_reset_tokens.sql  ← NEW
CREATE TABLE auth.password_reset_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(255) NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_prt_token_hash ON auth.password_reset_tokens(token_hash);
CREATE INDEX idx_prt_user_id ON auth.password_reset_tokens(user_id);
```

**Thêm routes** vào router:
```
POST /api/v1/auth/forgot-password   ← NEW
POST /api/v1/auth/reset-password    ← NEW
```

### 🟡 P1 — Thiếu: Email Verification Flow

`entity/user.go` đã có `IsVerified bool` — chỉ cần thêm use cases:

**Thêm mới**:
```
internal/usecase/verify_email/
├── send_verification.go   ← Generate token, send email
└── confirm_email.go       ← Validate token, set IsVerified=true
```

**Migration**:
```sql
-- migrations/003_email_verification_tokens.sql  ← NEW
CREATE TABLE auth.email_verification_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(255) NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
```

**Thêm routes**:
```
POST /api/v1/auth/verify-email/send      ← NEW (re-send verification)
GET  /api/v1/auth/verify-email/confirm   ← NEW (confirm token)
```

### 🟡 P1 — Thiếu: Admin Use Cases

**Thêm mới**:
```
internal/usecase/admin/
├── list_users.go      ← List all users với filter/pagination
├── get_user.go        ← Get user detail by ID
├── update_user.go     ← Update user role/status
├── ban_user.go        ← Deactivate user (set is_active=false)
└── unban_user.go      ← Re-activate user
```

```go
// Sử dụng lại existing repository (không tạo repo mới)
// Chỉ cần thêm methods vào UserRepository interface:
type UserRepository interface {
    // ... existing methods giữ nguyên ...
    ListAll(ctx context.Context, filter UserFilter) ([]*entity.User, int, error)  // NEW
    CountAll(ctx context.Context, filter UserFilter) (int, error)                  // NEW
}
```

**Thêm HTTP handler**:
```
adapter/handler/http/admin_handler.go   ← NEW
```

**Thêm routes** (admin only):
```
GET  /api/v1/admin/users            ← NEW (admin role required)
GET  /api/v1/admin/users/{id}       ← NEW
PUT  /api/v1/admin/users/{id}/role  ← NEW
POST /api/v1/admin/users/{id}/ban   ← NEW
POST /api/v1/admin/users/{id}/unban ← NEW
```

### 🟡 P1 — Mở rộng: `internal/infra/auth/` (Auth Cache)

Thư mục `internal/infra/auth/` hiện trống. **Thêm vào đây**:

```
internal/infra/auth/
└── validated_token_cache.go   ← NEW
```

```go
// Validated token cache — giảm load cho validate_token UC
// Cache kết quả validate JWT trong Redis (TTL 1 phút)
// Key: sha256(token[:16])
// Value: serialized Claims struct

type ValidatedTokenCache struct {
    redis  *redis.Client
    ttl    time.Duration
}

func (c *ValidatedTokenCache) Get(ctx context.Context, tokenPrefix string) (*Claims, bool)
func (c *ValidatedTokenCache) Set(ctx context.Context, tokenPrefix string, claims *Claims)
func (c *ValidatedTokenCache) Invalidate(ctx context.Context, userID string)
```

### 🟡 P1 — Mở rộng: `internal/provider/` (Provider Chain)

`internal/provider/local.go` và `chain.go` hiện không được kết nối. **Thêm kết nối**:

```go
// cmd/server/main.go ← CHỈ THÊM: init provider chain
providerChain := provider.NewChain(
    provider.NewLocal(userRepo, crypto.NewVerifier()),
)
// Wire vào loginUC thay vì trực tiếp gọi crypto functions
```

### 🟢 P2 — Thiếu: ValidateAPIKey gRPC RPC

**Thêm vào** `adapter/handler/grpc/auth_grpc_handler.go` (không sửa existing methods):
```go
// Thêm method mới vào existing handler struct:
func (h *AuthGRPCHandler) ValidateAPIKey(ctx context.Context, req *authpb.ValidateAPIKeyRequest) (*authpb.ValidateAPIKeyResponse, error) {
    // Delegate to existing manage_api_key UC
}
```

**Cập nhật proto** `shared/proto/auth/v1/`:
```protobuf
// Thêm RPCs mới vào service (không xóa cũ):
service AuthService {
    // ... existing RPCs ...
    rpc ValidateAPIKey(ValidateAPIKeyRequest) returns (ValidateAPIKeyResponse);  // NEW
    rpc ValidateTOTP(ValidateTOTPRequest) returns (ValidateTOTPResponse);       // NEW
}
```

---

## 3. Migration Plan — Chỉ Thêm File Mới

```
migrations/
├── 001_initial_schema.sql               ← GIỮ NGUYÊN (không sửa)
├── 002_password_reset_tokens.sql        ← NEW
├── 003_email_verification_tokens.sql    ← NEW
└── 004_totp_backup_codes.sql            ← NEW (optional, nếu cần backup codes)
```

```sql
-- migrations/004_totp_backup_codes.sql  ← NEW (optional)
CREATE TABLE auth.totp_backup_codes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    code_hash   VARCHAR(255) NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
```

---

## 4. Dependency Matrix — Thêm mới

```
identity-service → shared/pkg (logger, errors, health)          ← hiện có
identity-service → shared/proto/auth/v1 (gRPC contracts)        ← hiện có
identity-service → PostgreSQL (users, sessions, api_keys)        ← hiện có
identity-service → Redis (brute-force counter, token blacklist)  ← hiện có
identity-service → NATS (publish user events)                    ← THÊM MỚI
identity-service → SMTP (email verification, pwd reset)          ← THÊM MỚI
```

---

## 5. File Changes Summary

### Files cần THÊM MỚI:
```
internal/config/repo_config.go
internal/infra/auth/validated_token_cache.go
internal/infra/email/smtp_sender.go
internal/infra/messaging/nats/event_publisher.go
internal/usecase/totp/setup.go
internal/usecase/totp/verify.go
internal/usecase/totp/disable.go
internal/usecase/forgot_password/forgot_password.go
internal/usecase/reset_password/reset_password.go
internal/usecase/verify_email/send_verification.go
internal/usecase/verify_email/confirm_email.go
internal/usecase/admin/list_users.go
internal/usecase/admin/get_user.go
internal/usecase/admin/update_user.go
internal/usecase/admin/ban_user.go
adapter/handler/http/totp_handler.go
adapter/handler/http/admin_handler.go
adapter/handler/http/password_reset_handler.go
adapter/handler/http/email_verify_handler.go
migrations/002_password_reset_tokens.sql
migrations/003_email_verification_tokens.sql
migrations/004_totp_backup_codes.sql
```

### Files cần EXTEND (chỉ thêm vào cuối, không sửa logic cũ):
```
adapter/handler/http/router.go          ← Thêm 8 routes mới
adapter/handler/grpc/auth_grpc_handler.go ← Thêm 2 RPC methods mới
cmd/server/main.go                      ← Thêm wire cho email, NATS, TOTP UCs
internal/domain/repository/repositories.go ← Thêm 2 methods vào interface
shared/proto/auth/v1/*.proto            ← Thêm 2 RPC definitions mới
```

### Files KHÔNG ĐƯỢC CHẠM:
```
adapter/repository/postgres/user_repo.go    ← Giữ nguyên
adapter/repository/postgres/session_repo.go ← Giữ nguyên
adapter/repository/mongo/user_repo.go       ← Giữ nguyên (alternative)
adapter/repository/mongo/session_repo.go    ← Giữ nguyên (alternative)
usecase/login/login.go                      ← Giữ nguyên
usecase/register/register.go                ← Chỉ thêm 1 dòng publish event
migrations/001_initial_schema.sql           ← Không bao giờ sửa
```

---

## 6. Checklist

### Phase A — P0 (Sprint 1)
- [ ] Thêm `internal/config/repo_config.go` (repo backend selection)
- [ ] Thêm switch trong `cmd/server/main.go` để chọn postgres/mongo repo
- [ ] Thêm `internal/infra/auth/validated_token_cache.go`
- [ ] Thêm `internal/infra/messaging/nats/event_publisher.go`
- [ ] Thêm publish event vào `usecase/register/register.go`

### Phase B — P1 (Sprint 2)
- [ ] Thêm `usecase/totp/` (setup, verify, disable)
- [x] Thêm `adapter/handler/http/totp_handler.go`
- [ ] Thêm `usecase/forgot_password/` + `usecase/reset_password/`
- [ ] Thêm `internal/infra/email/smtp_sender.go`
- [ ] Thêm `migrations/002_password_reset_tokens.sql`
- [ ] Thêm `usecase/verify_email/` (send + confirm)
- [ ] Thêm `migrations/003_email_verification_tokens.sql`
- [ ] Thêm 3 routes cho totp, 2 routes cho email verify, 2 routes cho password reset

### Phase C — P1 (Sprint 2)
- [ ] Thêm `usecase/admin/` (5 use cases)
- [ ] Thêm `adapter/handler/http/admin_handler.go`
- [ ] Thêm 5 admin routes (với RBAC middleware)

### Phase D — P2 (Sprint 3)
- [ ] Thêm `ValidateAPIKey` + `ValidateTOTP` RPC vào gRPC handler
- [ ] Cập nhật proto với 2 RPC mới
- [ ] Thêm kết nối provider chain vào login flow
