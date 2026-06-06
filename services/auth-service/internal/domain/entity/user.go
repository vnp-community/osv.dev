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
	// Empty when MFA is not enabled.
	MFATOTPSecret string

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
