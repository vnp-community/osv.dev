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
	AuthProviderSAML   AuthProvider = "saml"
	AuthProviderLDAP   AuthProvider = "ldap"
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

	// ── DefectDojo RBAC extensions ────────────────────────────────────────────

	// FirstName and LastName for display purposes.
	FirstName string
	LastName  string

	// IsStaff grants access to the Django-compatible admin panel.
	IsStaff bool

	// IsSuperuser bypasses ALL RBAC permission checks.
	// Superusers can do anything in the system.
	IsSuperuser bool

	// IsLockedFlag is set manually by an admin to lock an account.
	// Distinct from IsActive (which is set automatically on deactivation).
	IsLockedFlag bool

	// ForcePasswordChange requires the user to change password on next login.
	ForcePasswordChange bool

	// GlobalRoleID is the system-wide role for this user (nil = no global role).
	// Product/ProductType-specific roles are in the role_assignments table.
	GlobalRoleID *RoleID
}

// IsPasswordSet returns true when the user has a local password.
func (u *User) IsPasswordSet() bool {
	return u.HashedPassword != ""
}

// IsLocked returns true if the user has exceeded login attempt limits or was manually locked.
func (u *User) IsLocked() bool {
	return !u.IsActive || u.IsLockedFlag
}
