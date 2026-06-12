package entity

import (
	"time"

	"github.com/google/uuid"
)

// Session represents a user's active refresh token session.
// Sessions enable token rotation and replay attack detection via token families.
type Session struct {
	ID uuid.UUID

	// UserID links this session to a user.
	UserID uuid.UUID

	// RefreshTokenHash is SHA-256(refreshToken) — raw token never stored.
	RefreshTokenHash string

	// TokenFamily is a UUID grouping related refresh tokens.
	// If a revoked token in a family is used, the entire family is revoked
	// (replay attack detection).
	TokenFamily string

	// IPAddress of the client at session creation.
	IPAddress string

	// UserAgent of the client browser/app.
	UserAgent string

	// ExpiresAt is when this refresh token expires (typically 7 days).
	ExpiresAt time.Time

	// RevokedAt is set when the session is explicitly revoked.
	// Nil means the session is still active.
	RevokedAt *time.Time

	CreatedAt time.Time
}

// IsActive returns true if the session has not been revoked and has not expired.
func (s *Session) IsActive() bool {
	return s.RevokedAt == nil && time.Now().Before(s.ExpiresAt)
}
