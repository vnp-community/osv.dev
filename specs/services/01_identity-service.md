# identity-service

**Bounded Context**: Identity & Access Management (IAM)
**Go Module**: `github.com/osv/identity-service`

---

## Merge từ

| Source | Trạng thái |
|--------|-----------|
| `services/auth-service` | ✅ Active — base chính |
| `archive/identity` | 📦 Archive — merged |
| `archive/admin` | 📦 Archive — merged |

---

## Chức năng

| # | Chức năng | Mô tả |
|---|-----------|-------|
| 1 | **Register** | Đăng ký tài khoản (email + password) |
| 2 | **Login** | Xác thực username/password, cấp JWT access token + refresh token |
| 3 | **Logout** | Huỷ session, blacklist token |
| 4 | **Refresh Token** | Làm mới JWT access token bằng refresh token |
| 5 | **Validate Token** | Kiểm tra JWT hợp lệ (dùng nội bộ bởi gateway-service) |
| 6 | **OAuth2** | Đăng nhập qua Google, GitHub, SSO providers |
| 7 | **2FA / TOTP** | Xác thực 2 bước với TOTP (Google Authenticator) |
| 8 | **API Key** | CRUD API keys cho programmatic access |
| 9 | **RBAC** | Role-based access control — quản lý roles & permissions |
| 10 | **Admin** | Quản trị user accounts (ban, reset password, assign roles) |

---

## Clean Architecture Layout

```
identity-service/
├── cmd/
│   └── server/
│       └── main.go             # Entry point
│
├── internal/
│   ├── domain/                 # ← Business rules (no external deps)
│   │   ├── user/
│   │   │   ├── entity.go       # User aggregate root
│   │   │   ├── repository.go   # UserRepository interface
│   │   │   └── events.go       # UserRegistered, UserLoggedIn, etc.
│   │   ├── token/
│   │   │   ├── access_token.go # JWT access token value object
│   │   │   ├── refresh_token.go
│   │   │   └── api_key.go
│   │   ├── role/
│   │   │   ├── entity.go       # Role aggregate
│   │   │   ├── permission.go   # Permission value object
│   │   │   └── repository.go
│   │   ├── session/
│   │   │   └── entity.go       # Session entity
│   │   └── errors/
│   │       └── errors.go       # Domain errors
│   │
│   ├── usecase/                # ← Application use cases
│   │   ├── register/
│   │   │   ├── usecase.go
│   │   │   └── dto.go
│   │   ├── login/
│   │   │   ├── usecase.go
│   │   │   └── dto.go
│   │   ├── logout/
│   │   │   └── usecase.go
│   │   ├── refresh_token/
│   │   │   └── usecase.go
│   │   ├── validate_token/
│   │   │   └── usecase.go
│   │   ├── oauth/
│   │   │   ├── usecase.go
│   │   │   └── providers.go
│   │   ├── totp/
│   │   │   ├── setup.go
│   │   │   └── verify.go
│   │   └── manage_api_key/
│   │       └── usecase.go
│   │
│   ├── delivery/               # ← Transport layer
│   │   ├── grpc/
│   │   │   ├── server.go
│   │   │   └── auth_handler.go # ValidateToken, GetUserInfo RPCs
│   │   └── http/
│   │       ├── router.go
│   │       ├── auth_handler.go
│   │       ├── oauth_handler.go
│   │       ├── apikey_handler.go
│   │       └── admin_handler.go
│   │
│   └── infra/                  # ← External systems
│       ├── postgres/
│       │   ├── user_repo.go
│       │   └── role_repo.go
│       ├── mongo/
│       │   └── session_repo.go
│       ├── redis/
│       │   └── token_cache.go  # Blacklist, session cache
│       └── oauth2/
│           ├── google.go
│           └── github.go
│
├── migrations/
│   ├── 001_create_users.sql
│   ├── 002_create_roles.sql
│   ├── 003_create_api_keys.sql
│   └── 004_create_permissions.sql
│
├── go.mod
└── Dockerfile
```

---

## Domain Model

### User Aggregate
```go
type User struct {
    ID           uuid.UUID
    Email        string
    PasswordHash string
    TOTPSecret   string
    TOTPEnabled  bool
    Status       UserStatus  // active | suspended | pending
    Roles        []RoleID
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### Token Value Objects
```go
type AccessToken struct {
    Raw       string
    UserID    uuid.UUID
    Roles     []string
    ExpiresAt time.Time
}

type APIKey struct {
    ID          uuid.UUID
    UserID      uuid.UUID
    KeyHash     string
    Scopes      []string   // read:cve, write:finding, etc.
    LastUsedAt  *time.Time
    ExpiresAt   *time.Time
}
```

### Role & Permission
```go
type Role struct {
    ID          uuid.UUID
    Name        string         // admin | analyst | viewer | api_user
    Permissions []Permission
}

type Permission string
// Examples: "cve:read", "finding:write", "scan:execute", "admin:*"
```

---

## API Specification

### HTTP REST Endpoints

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `POST` | `/auth/register` | Public | Đăng ký tài khoản mới |
| `POST` | `/auth/login` | Public | Đăng nhập, nhận JWT tokens |
| `POST` | `/auth/logout` | JWT | Huỷ session |
| `POST` | `/auth/refresh` | RefreshToken | Làm mới access token |
| `GET`  | `/auth/oauth/{provider}` | Public | Khởi tạo OAuth2 flow |
| `GET`  | `/auth/oauth/{provider}/callback` | Public | OAuth2 callback |
| `POST` | `/auth/totp/setup` | JWT | Bật 2FA |
| `POST` | `/auth/totp/verify` | JWT | Xác thực TOTP |
| `GET`  | `/auth/me` | JWT | Thông tin user hiện tại |
| `GET`  | `/auth/api-keys` | JWT | Liệt kê API keys |
| `POST` | `/auth/api-keys` | JWT | Tạo API key mới |
| `DELETE` | `/auth/api-keys/{id}` | JWT | Xoá API key |
| `GET`  | `/admin/users` | Admin | Danh sách users |
| `PUT`  | `/admin/users/{id}/status` | Admin | Kích hoạt/suspend user |
| `POST` | `/admin/users/{id}/roles` | Admin | Gán roles |

### gRPC Services (internal)

```protobuf
service AuthService {
    // Validate JWT — dùng bởi gateway-service
    rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse);

    // Get user info by ID — dùng bởi các services khác
    rpc GetUser(GetUserRequest) returns (GetUserResponse);

    // Check permission — RBAC check
    rpc CheckPermission(CheckPermissionRequest) returns (CheckPermissionResponse);
}
```

---

## Event Publishing (NATS)

| Event | Subject | Trigger |
|-------|---------|---------|
| `UserRegistered` | `identity.user.registered` | Đăng ký thành công |
| `UserLoggedIn` | `identity.user.logged_in` | Đăng nhập thành công |
| `UserSuspended` | `identity.user.suspended` | Admin suspend user |

---

## Dependencies

### External Libraries
```
github.com/golang-jwt/jwt/v5   # JWT signing & verification
golang.org/x/crypto            # bcrypt password hashing
golang.org/x/oauth2            # OAuth2 client
github.com/pquerna/otp         # TOTP generation & verification
github.com/jackc/pgx/v5        # PostgreSQL
go.mongodb.org/mongo-driver    # MongoDB
github.com/redis/go-redis/v9   # Redis
github.com/go-chi/chi/v5       # HTTP router
google.golang.org/grpc         # gRPC server
github.com/osv/shared/pkg      # Shared utilities
```

### Internal Dependencies
- Không phụ thuộc vào service nào khác
- Được gọi bởi: `gateway-service`, `scan-service`, `finding-service`

---

## Configuration

```yaml
# config.yaml
server:
  http_port: 8081
  grpc_port: 50051

jwt:
  secret: "${JWT_SECRET}"
  access_ttl: "15m"
  refresh_ttl: "7d"

postgres:
  dsn: "${POSTGRES_DSN}"

redis:
  addr: "${REDIS_ADDR}"
  db: 0

mongo:
  uri: "${MONGO_URI}"
  database: "identity"

oauth2:
  google:
    client_id: "${GOOGLE_CLIENT_ID}"
    client_secret: "${GOOGLE_CLIENT_SECRET}"
  github:
    client_id: "${GITHUB_CLIENT_ID}"
    client_secret: "${GITHUB_CLIENT_SECRET}"
```

---

## Database Schema (PostgreSQL)

```sql
-- Users
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255),
    totp_secret   VARCHAR(255),
    totp_enabled  BOOLEAN DEFAULT FALSE,
    status        VARCHAR(20) DEFAULT 'active',
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

-- Roles
CREATE TABLE roles (
    id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL
);

-- User-Role mapping
CREATE TABLE user_roles (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

-- API Keys
CREATE TABLE api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID REFERENCES users(id) ON DELETE CASCADE,
    key_hash     VARCHAR(255) NOT NULL,
    name         VARCHAR(255),
    scopes       TEXT[],
    last_used_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);
```
