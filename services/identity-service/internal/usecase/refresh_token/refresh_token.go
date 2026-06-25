// Package refreshtoken provides the token refresh use case.
// Implements refresh token rotation with replay attack detection via token families.
package refreshtoken

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/osv/identity-service/adapter/repository/postgres"
	"github.com/osv/identity-service/internal/domain/entity"
	domainerr "github.com/osv/identity-service/internal/domain/error"
	"github.com/osv/identity-service/internal/domain/repository"
	jwtpkg "github.com/osv/identity-service/internal/infrastructure/jwt"
	"net"
)

// Request is the input for the RefreshToken use case.
type Request struct {
	RefreshToken string
	IPAddress    string
	UserAgent    string
}

// Response contains the new token pair.
type Response struct {
	AccessToken      string
	RefreshToken     string
	ExpiresIn        int
	RefreshExpiresIn int
}

// UseCase orchestrates refresh token rotation.
type UseCase struct {
	userRepo    repository.UserRepository
	sessionRepo repository.SessionRepository
	jwtSvc      *jwtpkg.Service
}

// NewUseCase creates a new RefreshToken use case.
func NewUseCase(
	userRepo repository.UserRepository,
	sessionRepo repository.SessionRepository,
	jwtSvc *jwtpkg.Service,
) *UseCase {
	return &UseCase{userRepo: userRepo, sessionRepo: sessionRepo, jwtSvc: jwtSvc}
}

// Execute rotates refresh tokens using the token family pattern:
//  1. Look up session by refresh token hash
//  2. If session is REVOKED → replay attack → revoke entire family
//  3. If session is expired → reject
//  4. Revoke current session, create new session with new token
//  5. Issue new access token + refresh token
func (uc *UseCase) Execute(ctx context.Context, req Request) (*Response, error) {
	hash := postgres.HashRefreshToken(req.RefreshToken)

	session, err := uc.sessionRepo.FindByRefreshTokenHash(ctx, hash)
	if errors.Is(err, domainerr.ErrSessionNotFound) {
		return nil, domainerr.ErrInvalidToken
	}
	if err != nil {
		return nil, err
	}

	// Replay attack detection: token already revoked
	if session.RevokedAt != nil {
		// Revoke entire family to invalidate all tokens derived from this chain
		uc.sessionRepo.RevokeByFamily(ctx, session.TokenFamily) //nolint:errcheck
		return nil, domainerr.ErrTokenRevoked
	}

	// Expiry check
	if time.Now().After(session.ExpiresAt) {
		return nil, domainerr.ErrTokenExpired
	}

	// Load user
	user, err := uc.userRepo.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, domainerr.ErrAccountInactive
	}

	// Revoke current session (token rotation)
	uc.sessionRepo.RevokeByID(ctx, session.ID) //nolint:errcheck

	// Generate new token pair
	accessToken, _, err := uc.jwtSvc.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}
	newRefreshToken, err := uc.jwtSvc.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	// Create new session (same family → chain continues)
	newSession := &entity.Session{
		UserID:           user.ID,
		RefreshTokenHash: postgres.HashRefreshToken(newRefreshToken),
		TokenFamily:      session.TokenFamily, // preserve family
		IPAddress:        extractIP(req.IPAddress),
		UserAgent:        req.UserAgent,
		ExpiresAt:        session.ExpiresAt, // preserve original expiration
	}
	if err := uc.sessionRepo.Create(ctx, newSession); err != nil {
		return nil, err
	}

	refreshExpiresIn := int(time.Until(session.ExpiresAt).Seconds())
	if refreshExpiresIn < 0 {
		refreshExpiresIn = 0
	}

	return &Response{
		AccessToken:      accessToken,
		RefreshToken:     newRefreshToken,
		ExpiresIn:        int((15 * time.Minute).Seconds()),
		RefreshExpiresIn: refreshExpiresIn,
	}, nil
}

// ── Logout use case (bundled here for brevity) ────────────────────────────────

// LogoutUseCase revokes all sessions for a user or a specific session.
type LogoutUseCase struct {
	sessionRepo repository.SessionRepository
	userID      uuid.UUID
}

// NewLogoutUseCase creates a logout use case for the given user.
func NewLogoutUseCase(sessionRepo repository.SessionRepository) *LogoutUseCase {
	return &LogoutUseCase{sessionRepo: sessionRepo}
}

// Execute revokes all active sessions for the given user (logout all devices).
func (uc *LogoutUseCase) Execute(ctx context.Context, userID uuid.UUID, refreshToken string) error {
	if refreshToken != "" {
		// Single-device logout: revoke just this session
		hash := postgres.HashRefreshToken(refreshToken)
		session, err := uc.sessionRepo.FindByRefreshTokenHash(ctx, hash)
		if err == nil && session != nil {
			return uc.sessionRepo.RevokeByID(ctx, session.ID)
		}
	}
	// All-devices logout
	return uc.sessionRepo.RevokeByUserID(ctx, userID)
}

func extractIP(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
