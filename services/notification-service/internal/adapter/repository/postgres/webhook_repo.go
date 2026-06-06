// Package postgres implements the WebhookRepository backed by PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/globalcve/notification-service/internal/domain/aggregate/webhook"
	domainerrors "github.com/globalcve/notification-service/internal/domain/errors"
	"github.com/globalcve/notification-service/internal/domain/repository"
)

type webhookRepository struct{ db *pgxpool.Pool }

// NewWebhookRepository creates a PostgreSQL-backed WebhookRepository.
func NewWebhookRepository(db *pgxpool.Pool) repository.WebhookRepository {
	return &webhookRepository{db: db}
}

// Save inserts or updates a webhook record.
func (r *webhookRepository) Save(ctx context.Context, w *webhook.Webhook) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO webhooks (id, owner_id, url, events, secret, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			url        = EXCLUDED.url,
			events     = EXCLUDED.events,
			active     = EXCLUDED.active,
			updated_at = NOW()`,
		w.ID(), w.OwnerID(), w.URL(), w.Events(),
		w.Secret(), w.IsActive(), w.CreatedAt(), w.UpdatedAt(),
	)
	if err != nil {
		return fmt.Errorf("webhook save: %w", err)
	}
	return nil
}

// FindByID loads a webhook by primary key.
func (r *webhookRepository) FindByID(ctx context.Context, id string) (*webhook.Webhook, error) {
	return r.scanOne(ctx, `
		SELECT id, owner_id, url, events, secret, active, created_at, updated_at
		FROM webhooks WHERE id = $1`, id)
}

// FindByOwner returns all webhooks owned by ownerID.
func (r *webhookRepository) FindByOwner(ctx context.Context, ownerID string) ([]*webhook.Webhook, error) {
	return r.scanMany(ctx, `
		SELECT id, owner_id, url, events, secret, active, created_at, updated_at
		FROM webhooks WHERE owner_id = $1 ORDER BY created_at DESC`, ownerID)
}

// Delete removes a webhook by id, guarded by ownerID.
func (r *webhookRepository) Delete(ctx context.Context, id, ownerID string) error {
	tag, err := r.db.Exec(ctx,
		"DELETE FROM webhooks WHERE id = $1 AND owner_id = $2", id, ownerID)
	if err != nil {
		return fmt.Errorf("webhook delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domainerrors.ErrWebhookNotFound
	}
	return nil
}

// FindActiveByEvent returns all active webhooks subscribed to event.
func (r *webhookRepository) FindActiveByEvent(ctx context.Context, event string) ([]*webhook.Webhook, error) {
	return r.scanMany(ctx, `
		SELECT id, owner_id, url, events, secret, active, created_at, updated_at
		FROM webhooks WHERE active = TRUE AND $1 = ANY(events)`, event)
}

// Count returns the number of active webhooks.
func (r *webhookRepository) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM webhooks WHERE active = TRUE").Scan(&n)
	return n, err
}

// --- helpers ---

func (r *webhookRepository) scanOne(ctx context.Context, sql string, args ...interface{}) (*webhook.Webhook, error) {
	var (
		id, ownerID, rawURL, secret string
		events                       []string
		active                       bool
		createdAt, updatedAt         time.Time
	)
	err := r.db.QueryRow(ctx, sql, args...).Scan(
		&id, &ownerID, &rawURL, &events, &secret, &active, &createdAt, &updatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainerrors.ErrWebhookNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("webhook scan: %w", err)
	}
	return webhook.Reconstitute(id, ownerID, rawURL, events, secret, active, createdAt, updatedAt), nil
}

func (r *webhookRepository) scanMany(ctx context.Context, sql string, args ...interface{}) ([]*webhook.Webhook, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("webhook query: %w", err)
	}
	defer rows.Close()

	var result []*webhook.Webhook
	for rows.Next() {
		var (
			id, ownerID, rawURL, secret string
			events                       []string
			active                       bool
			createdAt, updatedAt         time.Time
		)
		if err := rows.Scan(&id, &ownerID, &rawURL, &events, &secret, &active, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		result = append(result, webhook.Reconstitute(id, ownerID, rawURL, events, secret, active, createdAt, updatedAt))
	}
	return result, rows.Err()
}
