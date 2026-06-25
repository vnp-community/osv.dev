// Package entity defines the core domain entities for the auth service.
package entity

import (
	"time"

	"github.com/google/uuid"
)

// AuthProvider identifies how the user authenticates.
type AuthProvider string

const (
	AuthProviderLocal  AuthProvider = "local"
	AuthProviderGoogle AuthProvider = "google"
	AuthProviderGitHub AuthProvider = "github"
)

// Role is a type alias for user role strings.
// Usecases use entity.Role as a named type for clarity.
type Role = string

const (
	RoleAdmin    Role = "admin"
	RoleUser     Role = "user"
	RoleReadonly Role = "readonly"
	RoleAgent    Role = "agent"
)

// User is the central authentication domain entity.
type User struct {
	ID uuid.UUID

	// Email is unique across all users and is the primary login identifier.
	Email string

	// Username is unique and used for display purposes.
	Username string

	// HashedPassword is nil when the user uses OAuth-only authentication.
	HashedPassword string

	// Role is the user's primary access role: admin|user|readonly.
	Role string

	// AuthProvider tracks how this account was created.
	AuthProvider AuthProvider

	// MFAEnabled is true when TOTP two-factor authentication is active.
	MFAEnabled bool

	// MFATOTPSecret is the encrypted TOTP secret (AES-256-GCM).
	// Nil when MFA is not enabled (DB stores NULL).
	MFATOTPSecret *string

	// IsActive controls login access; inactive users cannot authenticate.
	IsActive bool

	// IsVerified is true when the user's email has been confirmed.
	IsVerified bool

	// FailedLoginAttempts counts consecutive failures (also tracked in Redis).
	FailedLoginAttempts int

	// LastLoginAt is the last successful authentication timestamp.
	LastLoginAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsPasswordSet returns true when the user has a local password.
func (u *User) IsPasswordSet() bool {
	return u.HashedPassword != ""
}

// IsLocked returns true if the user has exceeded login attempt limits.
// The Redis-based check in the use case is authoritative; this is a domain helper.
func (u *User) IsLocked() bool {
	return !u.IsActive
}

// Permissions returns the user's permission strings derived from their role.
// Usecases call this when issuing JWT access tokens.
func (u *User) Permissions() []string {
	switch u.Role {
	case RoleAdmin:
		return []string{"read", "write", "admin", "delete"}
	case RoleUser:
		return []string{"read", "write"}
	case RoleReadonly:
		return []string{"read"}
	case RoleAgent:
		return []string{"read", "write", "scan"}
	default:
		return []string{"read"}
	}
}

// ── SEED-001: Admin bulk-create types ─────────────────────────────────────────

// UserCreateInput is the input for admin direct-create (bypasses invite flow).
type UserCreateInput struct {
	Email      string
	Username   string
	Password   string // plaintext — hashed by use case (bcrypt cost=12)
	Role       string // "admin" | "user" | "readonly" | "agent"
	IsActive   bool
	IsVerified bool
}

// UserCreateResult is the per-item outcome from a bulk-create operation.
type UserCreateResult struct {
	Email   string     `json:"email"`
	Status  string     `json:"status"`  // "created" | "error"
	ID      *uuid.UUID `json:"id,omitempty"`
	Message string     `json:"message,omitempty"`
}

// RoleAssignment is the input to grant a global or product-scoped role.
type RoleAssignment struct {
	UserID     uuid.UUID
	RoleID     int
	Scope      string     // "global" | "product"
	ResourceID *uuid.UUID // nil when Scope == "global"
	AssignedBy uuid.UUID
}
