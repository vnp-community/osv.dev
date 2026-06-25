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
