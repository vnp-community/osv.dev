package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/defectdojo/identity/internal/domain/entity"
	domainerr "github.com/defectdojo/identity/internal/domain/error"
)

// SessionRepo implements repository.SessionRepository using pgx/v5.
type SessionRepo struct {
	db *pgxpool.Pool
}

// NewSessionRepo creates a new SessionRepo backed by the given pgx pool.
func NewSessionRepo(db *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{db: db}
}

// Create inserts a new session. The raw refresh token MUST NOT be stored;
// only the SHA-256 hash is written to the DB.
func (r *SessionRepo) Create(ctx context.Context, s *entity.Session) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	s.CreatedAt = time.Now().UTC()

	_, err := r.db.Exec(ctx, `
		INSERT INTO auth.sessions
		  (id, user_id, refresh_token_hash, token_family, ip_address, user_agent, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		s.ID, s.UserID, s.RefreshTokenHash, s.TokenFamily,
		s.IPAddress, s.UserAgent, s.ExpiresAt, s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// FindByRefreshTokenHash looks up an active session by SHA-256 hash of the refresh token.
func (r *SessionRepo) FindByRefreshTokenHash(ctx context.Context, hash string) (*entity.Session, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, refresh_token_hash, token_family,
		       ip_address, user_agent, expires_at, revoked_at, created_at
		FROM auth.sessions
		WHERE refresh_token_hash=$1`,
		hash,
	)
	return scanSession(row)
}

// HashRefreshToken returns SHA-256 hex of the given token — helper for callers.
func HashRefreshToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// RevokeByID marks a single session as revoked.
func (r *SessionRepo) RevokeByID(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx,
		`UPDATE auth.sessions SET revoked_at=$2 WHERE id=$1 AND revoked_at IS NULL`,
		id, now,
	)
	return err
}

// RevokeByFamily revokes ALL sessions in a token family.
// Called on replay attack detection (reuse of a revoked refresh token).
func (r *SessionRepo) RevokeByFamily(ctx context.Context, family string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx,
		`UPDATE auth.sessions SET revoked_at=$2 WHERE token_family=$1 AND revoked_at IS NULL`,
		family, now,
	)
	return err
}

// RevokeByUserID revokes all active sessions for a user (logout all devices).
func (r *SessionRepo) RevokeByUserID(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx,
		`UPDATE auth.sessions SET revoked_at=$2 WHERE user_id=$1 AND revoked_at IS NULL`,
		userID, now,
	)
	return err
}

// CleanExpired deletes sessions expired more than 24h ago (called by a cron job).
func (r *SessionRepo) CleanExpired(ctx context.Context) error {
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	_, err := r.db.Exec(ctx,
		`DELETE FROM auth.sessions WHERE expires_at < $1`,
		cutoff,
	)
	return err
}

func scanSession(row pgx.Row) (*entity.Session, error) {
	var s entity.Session
	err := row.Scan(
		&s.ID, &s.UserID, &s.RefreshTokenHash, &s.TokenFamily,
		&s.IPAddress, &s.UserAgent, &s.ExpiresAt, &s.RevokedAt, &s.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainerr.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}
	return &s, nil
}
