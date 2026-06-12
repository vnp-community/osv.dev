// Package postgres provides PostgreSQL implementations of the auth domain repositories.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/identity-service/internal/domain/entity"
	domainerr "github.com/osv/identity-service/internal/domain/error"
)

// UserRepo implements repository.UserRepository using pgx/v5.
type UserRepo struct {
	db *pgxpool.Pool
}

// NewUserRepo creates a new UserRepo backed by the given pgx pool.
func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

// Create inserts a new user record. Returns ErrEmailAlreadyExists on duplicate email.
func (r *UserRepo) Create(ctx context.Context, u *entity.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	now := time.Now().UTC()
	u.CreatedAt = now
	u.UpdatedAt = now

	_, err := r.db.Exec(ctx, `
		INSERT INTO auth.users
		  (id, email, username, hashed_password, role, auth_provider,
		   mfa_enabled, mfa_totp_secret, is_active, is_verified, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		u.ID, u.Email, u.Username, u.HashedPassword, u.Role, u.AuthProvider,
		u.MFAEnabled, u.MFATOTPSecret, u.IsActive, u.IsVerified, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return domainerr.ErrEmailAlreadyExists
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// FindByID returns a user by UUID. Returns ErrUserNotFound if not found.
func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	return r.scanOne(ctx, `SELECT `+userColumns+` FROM auth.users WHERE id=$1`, id)
}

// FindByEmail returns a user by email (case-insensitive via CITEXT).
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	return r.scanOne(ctx, `SELECT `+userColumns+` FROM auth.users WHERE email=$1`, email)
}

// FindByUsername returns a user by username.
func (r *UserRepo) FindByUsername(ctx context.Context, username string) (*entity.User, error) {
	return r.scanOne(ctx, `SELECT `+userColumns+` FROM auth.users WHERE username=$1`, username)
}

// Update saves all mutable user fields.
func (r *UserRepo) Update(ctx context.Context, u *entity.User) error {
	u.UpdatedAt = time.Now().UTC()
	_, err := r.db.Exec(ctx, `
		UPDATE auth.users SET
		  username=$2, hashed_password=$3, role=$4,
		  mfa_enabled=$5, mfa_totp_secret=$6,
		  is_active=$7, is_verified=$8, updated_at=$9
		WHERE id=$1`,
		u.ID, u.Username, u.HashedPassword, u.Role,
		u.MFAEnabled, u.MFATOTPSecret, u.IsActive, u.IsVerified, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// UpdateLastLogin sets last_login_at to now and resets failed_login_attempts.
func (r *UserRepo) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx,
		`UPDATE auth.users SET last_login_at=$2, failed_login_attempts=0, updated_at=$2 WHERE id=$1`,
		id, now,
	)
	return err
}

// Delete removes a user record by ID.
func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM auth.users WHERE id=$1`, id)
	return err
}

// ── helpers ───────────────────────────────────────────────────────────────────

const userColumns = `
	id, email, username, hashed_password, role, auth_provider,
	mfa_enabled, mfa_totp_secret, is_active, is_verified,
	failed_login_attempts, last_login_at, created_at, updated_at`

func (r *UserRepo) scanOne(ctx context.Context, query string, args ...any) (*entity.User, error) {
	row := r.db.QueryRow(ctx, query, args...)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainerr.ErrUserNotFound
	}
	return u, err
}

func scanUser(row pgx.Row) (*entity.User, error) {
	var u entity.User
	err := row.Scan(
		&u.ID, &u.Email, &u.Username, &u.HashedPassword, &u.Role, &u.AuthProvider,
		&u.MFAEnabled, &u.MFATOTPSecret, &u.IsActive, &u.IsVerified,
		&u.FailedLoginAttempts, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func isDuplicateKey(err error) bool {
	return err != nil && (contains(err.Error(), "unique") || contains(err.Error(), "duplicate"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
