// Package oauth provides the OAuth2 callback use case.
// Handles Google and GitHub OAuth2 flows with user creation or linking.
package oauth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/osv/identity-service/internal/domain/entity"
	domainerr "github.com/osv/identity-service/internal/domain/error"
	"github.com/osv/identity-service/internal/domain/repository"
	"github.com/osv/identity-service/internal/infrastructure/oauth"
	jwtpkg "github.com/osv/identity-service/internal/infrastructure/jwt"
	pgRepo "github.com/osv/identity-service/adapter/repository/postgres"
)

// CallbackRequest is input for the OAuth callback handler.
type CallbackRequest struct {
	Provider  string // "google" | "github"
	Code      string
	State     string
	IPAddress string
	UserAgent string
}

// CallbackResponse is returned on successful OAuth login/registration.
type CallbackResponse struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	UserID       string
	Role         string
	IsNewUser    bool
}

// OAuthProfile is the normalised profile from any OAuth provider.
type OAuthProfile struct {
	ProviderID string
	Provider   string
	Email      string
	Name       string
	AvatarURL  string
}

// UseCase handles OAuth2 callback flows.
type UseCase struct {
	userRepo     repository.UserRepository
	sessionRepo  repository.SessionRepository
	oauthRepo    repository.OAuthAccountRepository
	jwtSvc       *jwtpkg.Service
	google       *oauth.GoogleProvider
	github       *oauth.GitHubProvider
}

// NewUseCase creates the OAuth callback use case.
func NewUseCase(
	userRepo repository.UserRepository,
	sessionRepo repository.SessionRepository,
	oauthRepo repository.OAuthAccountRepository,
	jwtSvc *jwtpkg.Service,
	google *oauth.GoogleProvider,
	github *oauth.GitHubProvider,
) *UseCase {
	return &UseCase{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		oauthRepo:   oauthRepo,
		jwtSvc:      jwtSvc,
		google:      google,
		github:      github,
	}
}

// Execute processes the OAuth callback and returns tokens.
// Flow: exchange code → fetch profile → find/create user → link account → issue tokens.
func (uc *UseCase) Execute(ctx context.Context, req CallbackRequest) (*CallbackResponse, error) {
	// Step 1: Exchange code for profile
	profile, err := uc.fetchProfile(ctx, req.Provider, req.Code)
	if err != nil {
		return nil, fmt.Errorf("oauth %s: %w", req.Provider, err)
	}

	// Step 2: Try to find existing OAuth account link
	oauthAcct, err := uc.oauthRepo.FindByProviderID(ctx, profile.Provider, profile.ProviderID)

	var user *entity.User
	isNewUser := false

	if err == nil && oauthAcct != nil {
		// Existing linked account — load user
		user, err = uc.userRepo.FindByID(ctx, oauthAcct.UserID)
		if err != nil {
			return nil, err
		}
	} else if errors.Is(err, domainerr.ErrUserNotFound) {
		// No linked account — try to match by email or create new user
		user, err = uc.userRepo.FindByEmail(ctx, profile.Email)
		if errors.Is(err, domainerr.ErrUserNotFound) {
			// Create new user from OAuth profile
			user = &entity.User{
				Email:        profile.Email,
				Username:     generateUsername(profile.Name, profile.Email),
				Role:         "user",
				AuthProvider: entity.AuthProvider(profile.Provider),
				IsActive:     true,
				IsVerified:   true, // OAuth emails are pre-verified
			}
			if err := uc.userRepo.Create(ctx, user); err != nil {
				return nil, err
			}
			isNewUser = true
		} else if err != nil {
			return nil, err
		}

		// Link OAuth account to user
		acct := &entity.OAuthAccount{
			ID:         uuid.New(),
			UserID:     user.ID,
			Provider:   profile.Provider,
			ProviderID: profile.ProviderID,
			Email:      profile.Email,
			Name:       profile.Name,
			AvatarURL:  profile.AvatarURL,
		}
		uc.oauthRepo.Upsert(ctx, acct) //nolint:errcheck
	} else {
		return nil, err
	}

	if !user.IsActive {
		return nil, domainerr.ErrAccountInactive
	}

	// Step 3: Issue tokens
	accessToken, _, err := uc.jwtSvc.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}
	refreshToken, err := uc.jwtSvc.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	// Step 4: Create session
	session := &entity.Session{
		UserID:           user.ID,
		RefreshTokenHash: pgRepo.HashRefreshToken(refreshToken),
		TokenFamily:      uuid.New().String(),
		IPAddress:        req.IPAddress,
		UserAgent:        req.UserAgent,
		ExpiresAt:        time.Now().UTC().Add(7 * 24 * time.Hour),
	}
	uc.sessionRepo.Create(ctx, session) //nolint:errcheck

	uc.userRepo.UpdateLastLogin(ctx, user.ID) //nolint:errcheck

	return &CallbackResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int((15 * time.Minute).Seconds()),
		UserID:       user.ID.String(),
		Role:         user.Role,
		IsNewUser:    isNewUser,
	}, nil
}

// fetchProfile exchanges the OAuth code for a normalised profile.
func (uc *UseCase) fetchProfile(ctx context.Context, provider, code string) (*OAuthProfile, error) {
	switch provider {
	case "google":
		info, err := uc.google.Exchange(ctx, code)
		if err != nil {
			return nil, err
		}
		return &OAuthProfile{
			ProviderID: info.Sub,
			Provider:   "google",
			Email:      info.Email,
			Name:       info.Name,
			AvatarURL:  info.Picture,
		}, nil
	case "github":
		info, err := uc.github.Exchange(ctx, code)
		if err != nil {
			return nil, err
		}
		return &OAuthProfile{
			ProviderID: fmt.Sprintf("%d", info.ID),
			Provider:   "github",
			Email:      info.Email,
			Name:       info.Name,
			AvatarURL:  info.AvatarURL,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported OAuth provider: %s", provider)
	}
}

// generateUsername creates a unique-ish username from OAuth profile data.
func generateUsername(name, email string) string {
	// Try name first, fall back to email prefix
	base := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	if base == "" {
		parts := strings.SplitN(email, "@", 2)
		base = parts[0]
	}
	// Sanitise
	var clean strings.Builder
	for _, c := range base {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			clean.WriteRune(c)
		}
	}
	result := clean.String()
	if len(result) < 3 {
		result = "user_" + result
	}
	if len(result) > 40 {
		result = result[:40]
	}
	return result
}
