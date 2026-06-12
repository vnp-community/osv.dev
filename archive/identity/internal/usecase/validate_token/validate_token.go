// Package validatetoken provides the token validation use case (called by gRPC handler).
package validatetoken

import (
	"context"
	"errors"
	"time"

	domainerr "github.com/defectdojo/identity/internal/domain/error"
	"github.com/defectdojo/identity/internal/infrastructure/cache"
	jwtpkg "github.com/defectdojo/identity/internal/infrastructure/jwt"
)

// Response is the validated token payload.
type Response struct {
	Valid        bool
	UserID       string
	Role         string
	Permissions  []string
	ErrorMessage string
}

// UseCase validates a JWT token (used internally by gRPC handler).
type UseCase struct {
	jwtSvc     *jwtpkg.Service
	tokenCache *cache.TokenCache
}

// NewUseCase creates a ValidateToken use case.
func NewUseCase(jwtSvc *jwtpkg.Service, tokenCache *cache.TokenCache) *UseCase {
	return &UseCase{jwtSvc: jwtSvc, tokenCache: tokenCache}
}

// Execute validates a JWT token string and returns the claims.
// Fast path: RSA signature verify + JTI blacklist (no DB).
func (uc *UseCase) Execute(ctx context.Context, tokenStr string) *Response {
	claims, err := uc.jwtSvc.ValidateToken(tokenStr)
	if err != nil {
		return &Response{Valid: false, ErrorMessage: err.Error()}
	}

	// JTI blacklist check
	revoked, err := uc.tokenCache.IsJTIRevoked(ctx, claims.ID)
	if err == nil && revoked {
		return &Response{Valid: false, ErrorMessage: domainerr.ErrTokenRevoked.Error()}
	}

	return &Response{
		Valid:       true,
		UserID:      claims.UserID,
		Role:        claims.Role,
		Permissions: claims.Permissions,
	}
}

// RevokeToken adds a JTI to the blacklist (called on logout).
func (uc *UseCase) RevokeToken(ctx context.Context, tokenStr string) error {
	claims, err := uc.jwtSvc.ValidateToken(tokenStr)
	if errors.Is(err, domainerr.ErrTokenExpired) {
		return nil // already expired, nothing to blacklist
	}
	if err != nil {
		return err
	}

	expiresAt := claims.ExpiryTime()
	if expiresAt.IsZero() {
		return nil
	}
	remaining := time.Until(expiresAt)
	if remaining <= 0 {
		return nil
	}
	return uc.tokenCache.RevokeJTI(ctx, claims.ID, remaining)
}
