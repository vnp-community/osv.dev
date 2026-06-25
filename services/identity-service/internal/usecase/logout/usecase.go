package logout

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"

    "github.com/osv/identity-service/internal/cache/redis"
    "github.com/osv/identity-service/internal/domain/entity"
)

var ErrSessionNotFound = errors.New("session not found")

// SessionRepository defines session storage for logout.
type SessionRepository interface {
    FindByID(ctx context.Context, id uuid.UUID) (*entity.Session, error)
    FindByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Session, error)
    Update(ctx context.Context, session *entity.Session) error
    RevokeByUserID(ctx context.Context, userID uuid.UUID) error
}

// UseCase handles user logout (single session) and global logout (all sessions).
type UseCase struct {
    sessionRepo SessionRepository
    jtiCache    *redis.JTICache
}

// New creates a LogoutUseCase.
func New(sessionRepo SessionRepository, jtiCache *redis.JTICache) *UseCase {
    return &UseCase{sessionRepo: sessionRepo, jtiCache: jtiCache}
}

// Execute revokes the current session and blacklists the JTI.
func (uc *UseCase) Execute(ctx context.Context, sessionID uuid.UUID, jti string) error {
    session, err := uc.sessionRepo.FindByID(ctx, sessionID)
    if err != nil || session == nil {
        return ErrSessionNotFound
    }

    // Revoke session
    now := time.Now().UTC()
    session.RevokedAt = &now
    session.UpdatedAt = now
    if err := uc.sessionRepo.Update(ctx, session); err != nil {
        return fmt.Errorf("revoke session: %w", err)
    }

    // Blacklist JTI in Redis (remaining TTL ≈ access token expiry)
    if err := uc.jtiCache.Revoke(ctx, jti, 15*time.Minute); err != nil {
        // Non-fatal: session already revoked; log warning
        _ = err
    }

    return nil
}

// ExecuteAll revokes all sessions for a user (password change, account compromise).
func (uc *UseCase) ExecuteAll(ctx context.Context, userID uuid.UUID) error {
    if err := uc.sessionRepo.RevokeByUserID(ctx, userID); err != nil {
        return fmt.Errorf("revoke all sessions: %w", err)
    }
    return nil
}
