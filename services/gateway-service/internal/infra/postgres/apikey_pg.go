// Package postgres — PostgreSQL implementation of APIKeyRepository using pgx/v5.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/osv/gateway-service/internal/domain/entity"
	"github.com/osv/gateway-service/internal/domain/repository"
)

type pgAPIKeyRepository struct {
	db *pgxpool.Pool
}

// NewAPIKeyRepository creates a PostgreSQL-backed APIKeyRepository.
func NewAPIKeyRepository(db *pgxpool.Pool) repository.APIKeyRepository {
	return &pgAPIKeyRepository{db: db}
}

// FindByHash looks up an active API key by its SHA-256 hash.
func (r *pgAPIKeyRepository) FindByHash(ctx context.Context, hash string) (*entity.APIKey, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, owner_id, description, scopes, rate_limit, expires_at, last_used_at, is_active, created_at
		FROM api_keys
		WHERE key_hash = $1
	`, hash)

	key := &entity.APIKey{KeyHash: hash}
	var (
		rateLimit  *int
		expiresAt  *time.Time
		lastUsedAt *time.Time
	)
	err := row.Scan(
		&key.ID, &key.OwnerID, &key.Description, &key.Scopes,
		&rateLimit, &expiresAt, &lastUsedAt, &key.IsActive, &key.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("apikey find by hash: %w", err)
	}
	key.RateLimit = rateLimit
	key.ExpiresAt = expiresAt
	key.LastUsedAt = lastUsedAt
	return key, nil
}

// Save persists a new API key record.
func (r *pgAPIKeyRepository) Save(ctx context.Context, key *entity.APIKey) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO api_keys
			(id, key_hash, owner_id, description, scopes, rate_limit, expires_at, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		key.ID, key.KeyHash, key.OwnerID, key.Description,
		key.Scopes, key.RateLimit, key.ExpiresAt, key.IsActive, key.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("apikey save: %w", err)
	}
	return nil
}

// ListByOwner returns all active API keys for an owner (without KeyHash).
func (r *pgAPIKeyRepository) ListByOwner(ctx context.Context, ownerID string) ([]*entity.APIKey, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, description, scopes, expires_at, last_used_at, created_at
		FROM api_keys
		WHERE owner_id = $1 AND is_active = TRUE
		ORDER BY created_at DESC
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("apikey list by owner: %w", err)
	}
	defer rows.Close()

	var keys []*entity.APIKey
	for rows.Next() {
		key := &entity.APIKey{OwnerID: ownerID, IsActive: true}
		var expiresAt, lastUsedAt *time.Time
		if err := rows.Scan(
			&key.ID, &key.Description, &key.Scopes,
			&expiresAt, &lastUsedAt, &key.CreatedAt,
		); err != nil {
			return nil, err
		}
		key.ExpiresAt = expiresAt
		key.LastUsedAt = lastUsedAt
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// Revoke soft-deletes an API key (sets is_active=false).
// Returns ErrAPIKeyNotFound if the key does not belong to ownerID.
func (r *pgAPIKeyRepository) Revoke(ctx context.Context, id, ownerID string) error {
	tag, err := r.db.Exec(ctx,
		"UPDATE api_keys SET is_active = FALSE WHERE id = $1 AND owner_id = $2",
		id, ownerID,
	)
	if err != nil {
		return fmt.Errorf("apikey revoke: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrAPIKeyNotFound
	}
	return nil
}

// UpdateLastUsed sets last_used_at = NOW() for the given key ID.
// Intended for async fire-and-forget — errors are non-fatal.
func (r *pgAPIKeyRepository) UpdateLastUsed(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx,
		"UPDATE api_keys SET last_used_at = NOW() WHERE id = $1",
		id,
	)
	return err
}
