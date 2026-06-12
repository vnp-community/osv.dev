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

// APIKeyRepo implements repository.APIKeyRepository using pgx/v5.
type APIKeyRepo struct {
	db *pgxpool.Pool
}

// NewAPIKeyRepo creates a new APIKeyRepo backed by the given pgx pool.
func NewAPIKeyRepo(db *pgxpool.Pool) *APIKeyRepo {
	return &APIKeyRepo{db: db}
}

// Create inserts a new API key record. The raw key is NEVER stored; only KeyHash and Prefix.
func (r *APIKeyRepo) Create(ctx context.Context, k *entity.APIKey) error {
	if k.ID == uuid.Nil {
		k.ID = uuid.New()
	}
	k.CreatedAt = time.Now().UTC()

	_, err := r.db.Exec(ctx, `
		INSERT INTO auth.api_keys
		  (id, user_id, name, key_hash, prefix, permissions, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		k.ID, k.UserID, k.Name, k.KeyHash, k.Prefix, k.Permissions, k.CreatedAt, k.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create api_key: %w", err)
	}
	return nil
}

// FindByPrefix finds an active API key by its prefix (first 12 chars: "ovs_" + 8 chars).
// Prefix is unique per key and used for efficient DB lookup without exposing the full key hash.
func (r *APIKeyRepo) FindByPrefix(ctx context.Context, prefix string) (*entity.APIKey, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, name, key_hash, prefix, permissions,
		       created_at, last_used_at, expires_at, revoked_at
		FROM auth.api_keys
		WHERE prefix=$1 AND revoked_at IS NULL`,
		prefix,
	)
	k, err := scanAPIKey(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainerr.ErrAPIKeyNotFound
	}
	return k, err
}

// ListByUserID returns all API keys (including revoked) for a user.
func (r *APIKeyRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.APIKey, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, name, key_hash, prefix, permissions,
		       created_at, last_used_at, expires_at, revoked_at
		FROM auth.api_keys
		WHERE user_id=$1
		ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list api_keys: %w", err)
	}
	defer rows.Close()

	var keys []*entity.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// UpdateLastUsed sets last_used_at to now for the given key ID.
func (r *APIKeyRepo) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE auth.api_keys SET last_used_at=$2 WHERE id=$1`,
		id, time.Now().UTC(),
	)
	return err
}

// Revoke marks a key as revoked. Revoked keys cannot be used for authentication.
func (r *APIKeyRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx,
		`UPDATE auth.api_keys SET revoked_at=$2 WHERE id=$1 AND revoked_at IS NULL`,
		id, now,
	)
	return err
}

// OAuthAccountRepo implements repository.OAuthAccountRepository using pgx/v5.
type OAuthAccountRepo struct {
	db *pgxpool.Pool
}

// NewOAuthAccountRepo creates a new OAuthAccountRepo.
func NewOAuthAccountRepo(db *pgxpool.Pool) *OAuthAccountRepo {
	return &OAuthAccountRepo{db: db}
}

// Upsert inserts or updates an OAuth account link. Uses ON CONFLICT for idempotency.
func (r *OAuthAccountRepo) Upsert(ctx context.Context, a *entity.OAuthAccount) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
		INSERT INTO auth.oauth_accounts
		  (id, user_id, provider, provider_id, email, name, avatar_url, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8)
		ON CONFLICT (provider, provider_id) DO UPDATE SET
		  email=EXCLUDED.email, name=EXCLUDED.name,
		  avatar_url=EXCLUDED.avatar_url, updated_at=EXCLUDED.updated_at`,
		a.ID, a.UserID, a.Provider, a.ProviderID, a.Email, a.Name, a.AvatarURL, now,
	)
	return err
}

// FindByProviderID finds an OAuth account by provider name and provider's user ID.
func (r *OAuthAccountRepo) FindByProviderID(ctx context.Context, provider, providerID string) (*entity.OAuthAccount, error) {
	var a entity.OAuthAccount
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, provider, provider_id, email, name, avatar_url, created_at, updated_at
		FROM auth.oauth_accounts
		WHERE provider=$1 AND provider_id=$2`,
		provider, providerID,
	).Scan(&a.ID, &a.UserID, &a.Provider, &a.ProviderID, &a.Email, &a.Name, &a.AvatarURL, &a.CreatedAt, &a.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainerr.ErrUserNotFound
	}
	return &a, err
}

// FindByUserID returns all OAuth accounts linked to a user.
func (r *OAuthAccountRepo) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.OAuthAccount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, provider, provider_id, email, name, avatar_url, created_at, updated_at
		FROM auth.oauth_accounts WHERE user_id=$1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*entity.OAuthAccount
	for rows.Next() {
		var a entity.OAuthAccount
		if err := rows.Scan(&a.ID, &a.UserID, &a.Provider, &a.ProviderID, &a.Email, &a.Name, &a.AvatarURL, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, &a)
	}
	return accounts, rows.Err()
}

// ── helpers ───────────────────────────────────────────────────────────────────

type apiKeyScanner interface {
	Scan(dest ...any) error
}

func scanAPIKey(row apiKeyScanner) (*entity.APIKey, error) {
	var k entity.APIKey
	err := row.Scan(
		&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.Prefix, &k.Permissions,
		&k.CreatedAt, &k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan api_key: %w", err)
	}
	return &k, nil
}
