// Package login provides the user login use case.
package login

import (
	"context"
	"errors"
	"net"
	"strings"

	domainerr "github.com/osv/auth-service/internal/domain/error"
	"github.com/osv/auth-service/internal/domain/entity"
	"github.com/osv/auth-service/internal/domain/repository"
	"github.com/osv/auth-service/internal/infrastructure/cache"
	"github.com/osv/auth-service/internal/infrastructure/crypto"
	jwtpkg "github.com/osv/auth-service/internal/infrastructure/jwt"
	"github.com/google/uuid"
	pgRepo "github.com/osv/auth-service/adapter/repository/postgres"
	"time"
)

// Request is the input DTO for the Login use case.
type Request struct {
	Email     string
	Password  string
	TOTPCode  string // optional; required when MFA is enabled
	IPAddress string
	UserAgent string
}

// Response is the output DTO for a successful login.
type Response struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // seconds until access token expires
	UserID       string
	Role         string
}

// UseCase orchestrates the login flow:
//  1. Brute force check (Redis)
//  2. User lookup + password verification
//  3. MFA check (if enabled)
//  4. Issue access token (RS256 JWT) + refresh token
//  5. Create session record in DB
type UseCase struct {
	userRepo    repository.UserRepository
	sessionRepo repository.SessionRepository
	tokenCache  *cache.TokenCache
	jwtSvc      *jwtpkg.Service
}

// NewUseCase creates a new Login use case.
func NewUseCase(
	userRepo repository.UserRepository,
	sessionRepo repository.SessionRepository,
	tokenCache *cache.TokenCache,
	jwtSvc *jwtpkg.Service,
) *UseCase {
	return &UseCase{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		tokenCache:  tokenCache,
		jwtSvc:      jwtSvc,
	}
}

// Execute runs the login flow and returns tokens on success.
func (uc *UseCase) Execute(ctx context.Context, req Request) (*Response, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Step 1: Brute force check
	locked, err := uc.tokenCache.IsLockedOut(ctx, email)
	if err == nil && locked {
		return nil, domainerr.ErrAccountLocked
	}

	// Step 2: Find user
	user, err := uc.userRepo.FindByEmail(ctx, email)
	if errors.Is(err, domainerr.ErrUserNotFound) {
		uc.recordFailure(ctx, email)
		return nil, domainerr.ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	// Step 3: Account status checks
	if !user.IsActive {
		return nil, domainerr.ErrAccountInactive
	}
	if !user.IsPasswordSet() {
		return nil, domainerr.ErrInvalidCredentials // OAuth-only user
	}

	// Step 4: Password verification (Argon2id — constant time)
	match, err := crypto.VerifyPassword(req.Password, user.HashedPassword)
	if err != nil || !match {
		uc.recordFailure(ctx, email)
		return nil, domainerr.ErrInvalidCredentials
	}

	// Step 5: MFA verification
	if user.MFAEnabled {
		if req.TOTPCode == "" {
			return nil, domainerr.ErrMFARequired
		}
		if !crypto.ValidateTOTP(user.MFATOTPSecret, req.TOTPCode) {
			return nil, domainerr.ErrInvalidMFACode
		}
	}

	// Step 6: Reset brute force counter + update last_login
	uc.tokenCache.ResetLoginAttempts(ctx, email)
	uc.userRepo.UpdateLastLogin(ctx, user.ID)

	// Step 7: Generate tokens
	accessToken, _, err := uc.jwtSvc.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}
	refreshToken, err := uc.jwtSvc.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	// Step 8: Create session
	session := &entity.Session{
		UserID:           user.ID,
		RefreshTokenHash: pgRepo.HashRefreshToken(refreshToken),
		TokenFamily:      uuid.New().String(),
		IPAddress:        extractIP(req.IPAddress),
		UserAgent:        req.UserAgent,
		ExpiresAt:        time.Now().UTC().Add(7 * 24 * time.Hour),
	}
	if err := uc.sessionRepo.Create(ctx, session); err != nil {
		return nil, err
	}

	return &Response{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int((15 * time.Minute).Seconds()),
		UserID:       user.ID.String(),
		Role:         user.Role,
	}, nil
}

func (uc *UseCase) recordFailure(ctx context.Context, email string) {
	uc.tokenCache.IncrLoginAttempt(ctx, email) //nolint:errcheck
}

func extractIP(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
