package refresh

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"

    "github.com/osv/identity-service/internal/domain/entity"
)

var (
    ErrInvalidRefreshToken  = errors.New("invalid or expired refresh token")
    ErrSessionRevoked       = errors.New("session has been revoked")
    ErrTokenReuseDetected   = errors.New("token reuse detected — all sessions invalidated")
    ErrSessionExpired       = errors.New("session expired")
)

// SessionRepository defines session storage for token rotation.
type SessionRepository interface {
    FindByTokenHash(ctx context.Context, hash string) (*entity.Session, error)
    FindByTokenFamily(ctx context.Context, family uuid.UUID) ([]*entity.Session, error)
    Update(ctx context.Context, session *entity.Session) error
    RevokeByFamily(ctx context.Context, family uuid.UUID) error
    Save(ctx context.Context, session *entity.Session) error
}

// UserRepository defines user lookup needed on refresh.
type UserRepository interface {
    FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
}

// JWTManager issues tokens.
type JWTManager interface {
    IssueAccessToken(userID uuid.UUID, email string, role entity.Role, permissions []string) (string, string, error)
    IssueRefreshToken(sessionID uuid.UUID) (string, error)
    ParseRefreshToken(token string) (sessionID uuid.UUID, err error)
}

// RefreshOutput is the output of a successful token refresh.
type RefreshOutput struct {
    AccessToken  string
    RefreshToken string
    Session      *entity.Session
}

// UseCase handles refresh token rotation with token reuse detection.
type UseCase struct {
    sessionRepo SessionRepository
    userRepo    UserRepository
    jwtMgr      JWTManager
}

// New creates a RefreshUseCase.
func New(sessionRepo SessionRepository, userRepo UserRepository, jwtMgr JWTManager) *UseCase {
    return &UseCase{sessionRepo: sessionRepo, userRepo: userRepo, jwtMgr: jwtMgr}
}

// Execute rotates a refresh token. Implements token family tracking to detect reuse.
func (uc *UseCase) Execute(ctx context.Context, refreshToken, ipAddress, userAgent string) (*RefreshOutput, error) {
    // 1. Hash the provided refresh token
    hash := sha256Hex(refreshToken)

    // 2. Lookup session by hash
    session, err := uc.sessionRepo.FindByTokenHash(ctx, hash)
    if err != nil || session == nil {
        return nil, ErrInvalidRefreshToken
    }

    // 3. Token reuse detection: if session already revoked, kill entire family
    if session.RevokedAt != nil {
        _ = uc.sessionRepo.RevokeByFamily(ctx, session.TokenFamily)
        return nil, ErrTokenReuseDetected
    }

    // 4. Check expiry
    if time.Now().UTC().After(session.ExpiresAt) {
        return nil, ErrSessionExpired
    }

    // 5. Revoke current session (rotate: old session → new session)
    now := time.Now().UTC()
    session.RevokedAt = &now
    session.UpdatedAt = now
    if err := uc.sessionRepo.Update(ctx, session); err != nil {
        return nil, fmt.Errorf("revoke old session: %w", err)
    }

    // 6. Fetch user
    user, err := uc.userRepo.FindByID(ctx, session.UserID)
    if err != nil {
        return nil, fmt.Errorf("find user: %w", err)
    }

    // 7. Issue new access token
    accessToken, _, err := uc.jwtMgr.IssueAccessToken(user.ID, user.Email, user.Role, user.Permissions())
    if err != nil {
        return nil, fmt.Errorf("issue access token: %w", err)
    }

    // 8. Create new session (same token family)
    newSession := &entity.Session{
        ID:          uuid.New(),
        UserID:      user.ID,
        TokenFamily: session.TokenFamily, // keep same family
        IPAddress:   ipAddress,
        UserAgent:   userAgent,
        ExpiresAt:   time.Now().UTC().Add(7 * 24 * time.Hour),
        CreatedAt:   time.Now().UTC(),
    }

    newRefreshToken, err := uc.jwtMgr.IssueRefreshToken(newSession.ID)
    if err != nil {
        return nil, fmt.Errorf("issue refresh token: %w", err)
    }

    newSession.RefreshTokenHash = sha256Hex(newRefreshToken)
    if err := uc.sessionRepo.Save(ctx, newSession); err != nil {
        return nil, fmt.Errorf("save new session: %w", err)
    }

    return &RefreshOutput{
        AccessToken:  accessToken,
        RefreshToken: newRefreshToken,
        Session:      newSession,
    }, nil
}

func sha256Hex(s string) string {
    h := sha256.Sum256([]byte(s))
    return hex.EncodeToString(h[:])
}
