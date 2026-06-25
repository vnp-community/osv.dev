# TASK-AUTH-001 — Domain Entities: User, Session, APIKey

| Field | Value |
|-------|-------|
| **Task ID** | T-AUTH-001 |
| **Service** | `identity-service` |
| **Status** | ✅ Completed |
| **Solution Ref** | SOL-OVS-003 §3 Domain Model |
| **Priority** | 🔴 Critical (foundation) |
| **Depends On** | — |
| **Estimated** | 2h |

---

## Context

`identity-service` đã tồn tại tại `services/identity-service/`. Cấu trúc domain cần được bổ sung entities cho authentication:
- `User` — account với Argon2id password, role-based access
- `Session` — refresh token với token family tracking
- `APIKey` — API keys với prefix `ovs_`

**Existing path**: `services/identity-service/internal/domain/`

---

## Goal

Tạo đầy đủ Go domain entities (structs, constants, constructors, validation) cho 3 entities cốt lõi của auth system.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/identity-service/internal/domain/user/entity.go` |
| CREATE | `services/identity-service/internal/domain/user/errors.go` |
| CREATE | `services/identity-service/internal/domain/session/entity.go` |
| CREATE | `services/identity-service/internal/domain/session/errors.go` |
| CREATE | `services/identity-service/internal/domain/apikey/entity.go` |
| CREATE | `services/identity-service/internal/domain/apikey/errors.go` |

---

## Implementation

### File 1: `services/identity-service/internal/domain/user/entity.go`

```go
package user

import (
    "strings"
    "time"

    "github.com/google/uuid"
)

// Role defines the user's access level in the system
type Role string

const (
    RoleAdmin    Role = "admin"
    RoleUser     Role = "user"
    RoleReadOnly Role = "readonly"
    RoleAgent    Role = "agent"
)

// AuthProvider identifies the login method
type AuthProvider string

const (
    AuthProviderLocal  AuthProvider = "local"
    AuthProviderGoogle AuthProvider = "google"
    AuthProviderGitHub AuthProvider = "github"
)

// Permission constants — all valid permissions in the system
const (
    PermScanCreate    = "scan:create"
    PermScanRead      = "scan:read"
    PermScanDelete    = "scan:delete"
    PermFindingRead   = "finding:read"
    PermFindingWrite  = "finding:write"
    PermReportCreate  = "report:create"
    PermReportDownload = "report:download"
    PermProductRead   = "product:read"
    PermProductWrite  = "product:write"
    PermUserManage    = "user:manage"
    PermAgentReport   = "agent:report"
)

// RolePermissions maps roles to their default permissions
var RolePermissions = map[Role][]string{
    RoleAdmin: {
        PermScanCreate, PermScanRead, PermScanDelete,
        PermFindingRead, PermFindingWrite,
        PermReportCreate, PermReportDownload,
        PermProductRead, PermProductWrite,
        PermUserManage, PermAgentReport,
    },
    RoleUser: {
        PermScanCreate, PermScanRead,
        PermFindingRead, PermFindingWrite,
        PermReportCreate, PermReportDownload,
        PermProductRead, PermProductWrite,
        PermAgentReport,
    },
    RoleReadOnly: {
        PermScanRead, PermFindingRead,
        PermReportDownload, PermProductRead,
    },
    RoleAgent: {
        PermAgentReport, PermScanRead,
    },
}

// User represents an authenticated user account
type User struct {
    ID                   uuid.UUID
    Email                string
    Username             string
    HashedPassword       string       // Argon2id hash; empty for OAuth-only users
    Role                 Role
    AuthProvider         AuthProvider
    MFAEnabled           bool
    MFATOTPSecret        string       // AES-256-GCM encrypted TOTP secret
    MFABackupCodes       []string     // SHA-256 hashed backup codes
    IsActive             bool
    IsVerified           bool
    FailedLoginAttempts  int
    LockedAt             *time.Time
    LastLoginAt          *time.Time
    CreatedAt            time.Time
    UpdatedAt            time.Time
}

// NewUser creates a new local user with validation
func NewUser(email, username string) (*User, error) {
    email = strings.TrimSpace(strings.ToLower(email))
    username = strings.TrimSpace(username)

    if email == "" {
        return nil, ErrEmailRequired
    }
    if !isValidEmail(email) {
        return nil, ErrEmailInvalid
    }
    if username == "" {
        return nil, ErrUsernameRequired
    }
    if len(username) < 3 || len(username) > 50 {
        return nil, ErrUsernameLength
    }

    now := time.Now().UTC()
    return &User{
        ID:           uuid.New(),
        Email:        email,
        Username:     username,
        Role:         RoleUser,
        AuthProvider: AuthProviderLocal,
        IsActive:     true,
        IsVerified:   false,
        CreatedAt:    now,
        UpdatedAt:    now,
    }, nil
}

// NewOAuthUser creates a user authenticated via OAuth provider
func NewOAuthUser(email, username string, provider AuthProvider) (*User, error) {
    u, err := NewUser(email, username)
    if err != nil {
        return nil, err
    }
    u.AuthProvider = provider
    u.IsVerified = true // OAuth emails are pre-verified
    return u, nil
}

// Permissions returns the list of permissions for this user's role
func (u *User) Permissions() []string {
    perms, ok := RolePermissions[u.Role]
    if !ok {
        return []string{}
    }
    return perms
}

// HasPermission checks if the user has a specific permission
func (u *User) HasPermission(perm string) bool {
    for _, p := range u.Permissions() {
        if p == perm {
            return true
        }
    }
    return false
}

// IsLocked returns true if the account is temporarily locked
func (u *User) IsLocked() bool {
    return u.LockedAt != nil
}

// Lock disables login by recording the lockout time
func (u *User) Lock() {
    now := time.Now().UTC()
    u.LockedAt = &now
    u.UpdatedAt = now
}

// Unlock re-enables login and resets failure counter
func (u *User) Unlock() {
    u.LockedAt = nil
    u.FailedLoginAttempts = 0
    u.UpdatedAt = time.Now().UTC()
}

// RecordFailedLogin increments failure counter; locks after maxAttempts
func (u *User) RecordFailedLogin(maxAttempts int) {
    u.FailedLoginAttempts++
    u.UpdatedAt = time.Now().UTC()
    if u.FailedLoginAttempts >= maxAttempts {
        u.Lock()
    }
}

// RecordSuccessfulLogin resets failure counter and updates LastLoginAt
func (u *User) RecordSuccessfulLogin() {
    now := time.Now().UTC()
    u.FailedLoginAttempts = 0
    u.LastLoginAt = &now
    u.UpdatedAt = now
}

// isValidEmail performs basic email format validation
func isValidEmail(email string) bool {
    at := strings.Index(email, "@")
    if at < 1 {
        return false
    }
    dot := strings.LastIndex(email[at:], ".")
    return dot > 1
}
```

### File 2: `services/identity-service/internal/domain/user/errors.go`

```go
package user

import "errors"

var (
    ErrEmailRequired    = errors.New("email is required")
    ErrEmailInvalid     = errors.New("email format is invalid")
    ErrEmailTaken       = errors.New("email is already registered")
    ErrUsernameRequired = errors.New("username is required")
    ErrUsernameLength   = errors.New("username must be between 3 and 50 characters")
    ErrUsernameTaken    = errors.New("username is already taken")
    ErrPasswordTooShort = errors.New("password must be at least 12 characters")
    ErrUserNotFound     = errors.New("user not found")
    ErrUserLocked       = errors.New("account is locked due to too many failed login attempts")
    ErrUserInactive     = errors.New("account is inactive")
    ErrInvalidCredentials = errors.New("invalid email or password")
    ErrMFARequired      = errors.New("MFA code is required")
    ErrMFAInvalidCode   = errors.New("invalid or expired MFA code")
    ErrMFAAlreadyEnabled = errors.New("MFA is already enabled")
    ErrMFANotEnabled    = errors.New("MFA is not enabled")
)
```

### File 3: `services/identity-service/internal/domain/session/entity.go`

```go
package session

import (
    "time"

    "github.com/google/uuid"
)

// Session represents a user's refresh token session with family tracking
// for detecting token reuse attacks.
type Session struct {
    ID               uuid.UUID
    UserID           uuid.UUID
    RefreshTokenHash string    // SHA-256 of the plain refresh token
    TokenFamily      uuid.UUID // All rotations of the same original login share this family ID
    IPAddress        string
    UserAgent        string
    ExpiresAt        time.Time
    RevokedAt        *time.Time // Non-nil means this session is invalidated
    CreatedAt        time.Time
}

// NewSession creates a new session for a user login
func NewSession(userID uuid.UUID, refreshTokenHash, ip, userAgent string, ttl time.Duration) *Session {
    now := time.Now().UTC()
    return &Session{
        ID:               uuid.New(),
        UserID:           userID,
        RefreshTokenHash: refreshTokenHash,
        TokenFamily:      uuid.New(), // New family for initial login
        IPAddress:        ip,
        UserAgent:        userAgent,
        ExpiresAt:        now.Add(ttl),
        CreatedAt:        now,
    }
}

// Rotate creates a new session in the same family (for refresh token rotation)
func (s *Session) Rotate(newTokenHash, ip, userAgent string, ttl time.Duration) *Session {
    now := time.Now().UTC()
    return &Session{
        ID:               uuid.New(),
        UserID:           s.UserID,
        RefreshTokenHash: newTokenHash,
        TokenFamily:      s.TokenFamily, // Inherit the same family
        IPAddress:        ip,
        UserAgent:        userAgent,
        ExpiresAt:        now.Add(ttl),
        CreatedAt:        now,
    }
}

// Revoke marks the session as invalidated
func (s *Session) Revoke() {
    now := time.Now().UTC()
    s.RevokedAt = &now
}

// IsRevoked returns true if this session has been invalidated
func (s *Session) IsRevoked() bool {
    return s.RevokedAt != nil
}

// IsExpired returns true if the session has passed its expiry time
func (s *Session) IsExpired() bool {
    return time.Now().UTC().After(s.ExpiresAt)
}

// IsValid returns true if the session can be used for token refresh
func (s *Session) IsValid() bool {
    return !s.IsRevoked() && !s.IsExpired()
}
```

### File 4: `services/identity-service/internal/domain/session/errors.go`

```go
package session

import "errors"

var (
    ErrSessionNotFound      = errors.New("session not found")
    ErrSessionRevoked       = errors.New("session has been revoked")
    ErrSessionExpired       = errors.New("session has expired")
    ErrRefreshTokenReuse    = errors.New("refresh token reuse detected — all sessions revoked")
)
```

### File 5: `services/identity-service/internal/domain/apikey/entity.go`

```go
package apikey

import (
    "strings"
    "time"

    "github.com/google/uuid"
)

const (
    // KeyPrefix is the required prefix for all API keys
    // Allows instant identification (similar to GitHub's ghp_, Stripe's sk_)
    KeyPrefix = "ovs_"

    // PrefixDisplayLength is how many chars of the key are stored/displayed
    // "ovs_" (4) + 8 random chars = 12 chars total
    PrefixDisplayLength = 12
)

// APIKey represents a long-lived API key for programmatic access
type APIKey struct {
    ID          uuid.UUID
    UserID      uuid.UUID
    Name        string      // Human-readable label (e.g., "CI Pipeline")
    KeyHash     string      // SHA-256 of the full plain key; never store plain key
    Prefix      string      // First 12 chars of the key (for lookup)
    Permissions []string    // Subset of the user's permissions
    LastUsedAt  *time.Time
    ExpiresAt   *time.Time  // nil = no expiry
    RevokedAt   *time.Time  // nil = active
    CreatedAt   time.Time
}

// NewAPIKey creates an APIKey entity from a pre-hashed key
// plainKey and keyHash are provided by the crypto layer
func NewAPIKey(userID uuid.UUID, name, plainKey, keyHash string, permissions []string, expiresAt *time.Time) (*APIKey, error) {
    name = strings.TrimSpace(name)
    if name == "" {
        return nil, ErrNameRequired
    }
    if len(name) > 100 {
        return nil, ErrNameTooLong
    }
    if !strings.HasPrefix(plainKey, KeyPrefix) {
        return nil, ErrInvalidKeyFormat
    }
    if len(plainKey) < PrefixDisplayLength {
        return nil, ErrInvalidKeyFormat
    }

    return &APIKey{
        ID:          uuid.New(),
        UserID:      userID,
        Name:        name,
        KeyHash:     keyHash,
        Prefix:      plainKey[:PrefixDisplayLength],
        Permissions: permissions,
        ExpiresAt:   expiresAt,
        CreatedAt:   time.Now().UTC(),
    }, nil
}

// IsActive returns true if the key can be used for authentication
func (k *APIKey) IsActive() bool {
    if k.RevokedAt != nil {
        return false
    }
    if k.ExpiresAt != nil && time.Now().UTC().After(*k.ExpiresAt) {
        return false
    }
    return true
}

// Revoke invalidates the API key
func (k *APIKey) Revoke() {
    now := time.Now().UTC()
    k.RevokedAt = &now
}

// RecordUse updates the last used timestamp (called asynchronously)
func (k *APIKey) RecordUse() {
    now := time.Now().UTC()
    k.LastUsedAt = &now
}

// HasPermission checks if this key grants a specific permission
func (k *APIKey) HasPermission(perm string) bool {
    for _, p := range k.Permissions {
        if p == perm {
            return true
        }
    }
    return false
}
```

### File 6: `services/identity-service/internal/domain/apikey/errors.go`

```go
package apikey

import "errors"

var (
    ErrNameRequired     = errors.New("API key name is required")
    ErrNameTooLong      = errors.New("API key name must be 100 characters or less")
    ErrInvalidKeyFormat = errors.New("invalid API key format: must start with 'ovs_'")
    ErrKeyNotFound      = errors.New("API key not found")
    ErrKeyRevoked       = errors.New("API key has been revoked")
    ErrKeyExpired       = errors.New("API key has expired")
    ErrPermissionEscalation = errors.New("API key cannot have permissions beyond user's role")
    ErrMaxKeysReached   = errors.New("maximum number of API keys per user reached (10)")
)
```

---

## Verification

Sau khi tạo các files trên, chạy:

```bash
cd services/identity-service
go build ./internal/domain/...
```

**Expected**: Builds without errors.

```bash
go vet ./internal/domain/...
```

**Expected**: No warnings.

### Unit Test Checklist

Kiểm tra bằng `go test` hoặc manual review:

- [x] `NewUser("", "john")` → returns `ErrEmailRequired`
- [x] `NewUser("not-an-email", "john")` → returns `ErrEmailInvalid`
- [x] `NewUser("john@example.com", "jo")` → returns `ErrUsernameLength` (< 3 chars)
- [x] `NewUser("john@example.com", "john")` → returns valid User with `Role=RoleUser`
- [x] `user.RecordFailedLogin(5)` called 5 times → `user.IsLocked() == true`
- [x] `user.Unlock()` → `user.LockedAt == nil && user.FailedLoginAttempts == 0`
- [x] `user.HasPermission("scan:create")` for RoleAdmin → `true`
- [x] `user.HasPermission("user:manage")` for RoleUser → `false`
- [x] `session.Rotate(...)` → new session has same `TokenFamily` as parent
- [x] `session.IsValid()` after `Revoke()` → `false`
- [x] `NewAPIKey(..., "invalid_key", ...)` → `ErrInvalidKeyFormat`
- [x] `apiKey.IsActive()` after `Revoke()` → `false`

---

## Notes for AI

1. Đặt package name đúng với directory name (`package user`, `package session`, `package apikey`)
2. Tất cả times phải dùng `time.Now().UTC()` (không local timezone)
3. Không import external packages ngoài standard library và `github.com/google/uuid`
4. Kiểm tra `services/identity-service/go.mod` để xác nhận module name trước khi viết import paths
