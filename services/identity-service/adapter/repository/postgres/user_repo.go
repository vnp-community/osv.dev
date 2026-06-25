// Package postgres provides PostgreSQL implementations of the auth domain repositories.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/identity-service/internal/domain/entity"
	domainerr "github.com/osv/identity-service/internal/domain/error"
	"github.com/osv/identity-service/internal/domain/repository"
	"golang.org/x/crypto/bcrypt"
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

// List returns a paginated list of users based on the filter.
func (r *UserRepo) List(ctx context.Context, filter repository.UserFilter) ([]*entity.User, int, error) {
	query := "SELECT " + userColumns + " FROM auth.users WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM auth.users WHERE 1=1"

	args := []any{}
	argID := 1

	if filter.Role != "" {
		query += fmt.Sprintf(" AND role = $%d", argID)
		countQuery += fmt.Sprintf(" AND role = $%d", argID)
		args = append(args, filter.Role)
		argID++
	}
	if filter.IsActive != "" {
		isActive := filter.IsActive == "true"
		query += fmt.Sprintf(" AND is_active = $%d", argID)
		countQuery += fmt.Sprintf(" AND is_active = $%d", argID)
		args = append(args, isActive)
		argID++
	}
	if filter.Query != "" {
		query += fmt.Sprintf(" AND (email ILIKE $%d OR username ILIKE $%d)", argID, argID)
		countQuery += fmt.Sprintf(" AND (email ILIKE $%d OR username ILIKE $%d)", argID, argID)
		args = append(args, "%"+filter.Query+"%")
		argID++
	}

	// Get total count
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	// Add pagination
	query += " ORDER BY created_at DESC"
	if filter.PageSize > 0 {
		offset := (filter.Page - 1) * filter.PageSize
		if offset < 0 {
			offset = 0
		}
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", filter.PageSize, offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var users []*entity.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate users: %w", err)
	}

	return users, total, nil
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

// UpdateMFA sets the MFA status and secret.
func (r *UserRepo) UpdateMFA(ctx context.Context, userID uuid.UUID, enabled bool, secret string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx,
		`UPDATE auth.users SET mfa_totp_secret=$2, mfa_enabled=$3, updated_at=$4 WHERE id=$1`,
		userID, secret, enabled, now,
	)
	return err
}

// ── SEED-001: Admin bulk-create & role assignment ─────────────────────────────

// CreateDirect inserts a user with a pre-hashed password.
// Returns ErrEmailAlreadyExists when the email already exists (via ON CONFLICT check).
func (r *UserRepo) CreateDirect(ctx context.Context, u *entity.User, hashedPassword string) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	now := time.Now().UTC()

	tag, err := r.db.Exec(ctx, `
		INSERT INTO auth.users
		  (id, email, username, hashed_password, role, auth_provider,
		   mfa_enabled, is_active, is_verified, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,'local',false,$6,$7,$8,$9)
		ON CONFLICT (email) DO NOTHING`,
		u.ID, u.Email, u.Username, hashedPassword, u.Role,
		u.IsActive, u.IsVerified, now, now,
	)
	if err != nil {
		return fmt.Errorf("create direct user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domainerr.ErrEmailAlreadyExists
	}
	u.CreatedAt = now
	u.UpdatedAt = now
	return nil
}

// CreateBulk inserts multiple users within a single transaction.
// Rows that fail validation or conflict are recorded as status="error";
// the transaction commits whatever succeeded.
func (r *UserRepo) CreateBulk(ctx context.Context, inputs []entity.UserCreateInput) ([]entity.UserCreateResult, error) {
	results := make([]entity.UserCreateResult, 0, len(inputs))

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	now := time.Now().UTC()
	for _, in := range inputs {
		res := entity.UserCreateResult{Email: in.Email}

		if in.Email == "" {
			res.Status = "error"
			res.Message = "email is required"
			results = append(results, res)
			continue
		}
		if len(in.Password) < 8 {
			res.Status = "error"
			res.Message = "password must be at least 8 characters"
			results = append(results, res)
			continue
		}

		hashed, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			res.Status = "error"
			res.Message = "password hashing failed"
			results = append(results, res)
			continue
		}

		id := uuid.New()
		tag, err := tx.Exec(ctx, `
			INSERT INTO auth.users
			  (id, email, username, hashed_password, role, auth_provider,
			   mfa_enabled, is_active, is_verified, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,'local',false,$6,$7,$8,$9)
			ON CONFLICT (email) DO NOTHING`,
			id, in.Email, in.Username, string(hashed), in.Role,
			in.IsActive, in.IsVerified, now, now,
		)
		if err != nil {
			res.Status = "error"
			res.Message = err.Error()
			results = append(results, res)
			continue
		}
		if tag.RowsAffected() == 0 {
			res.Status = "error"
			res.Message = "email already exists"
			results = append(results, res)
			continue
		}

		res.Status = "created"
		res.ID = &id
		results = append(results, res)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit bulk create: %w", err)
	}
	return results, nil
}

// AssignRole upserts a role_assignment record.
func (r *UserRepo) AssignRole(ctx context.Context, a entity.RoleAssignment) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO auth.role_assignments
		  (user_id, role_id, scope, resource_id, assigned_by, assigned_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (user_id, role_id, scope,
		  COALESCE(resource_id, '00000000-0000-0000-0000-000000000000'::UUID))
		DO UPDATE SET assigned_by = EXCLUDED.assigned_by, assigned_at = NOW()`,
		a.UserID, a.RoleID, a.Scope, a.ResourceID, a.AssignedBy,
	)
	if err != nil {
		return fmt.Errorf("assign role: %w", err)
	}
	return nil
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
	return err != nil && (strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate"))
}

// Activate sets is_active=true and is_verified=true for the user.
// [FIX TASK-HC-014] Called by AcceptInvite after validating invitation token.
func (r *UserRepo) Activate(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE auth.users SET is_active=true, is_verified=true, updated_at=NOW() WHERE id=$1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("activate user: %w", err)
	}
	return nil
}
