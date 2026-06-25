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
