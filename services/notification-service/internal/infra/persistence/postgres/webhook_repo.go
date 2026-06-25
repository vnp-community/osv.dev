// Package postgres implements the WebhookRepository backed by PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/osv/notification-service/internal/domain/aggregate/webhook"
	domainerrors "github.com/osv/notification-service/internal/domain/errors"
	"github.com/osv/notification-service/internal/domain/repository"
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
	return webhook.ReconstituteFromStrings(id, ownerID, rawURL, events, secret, active, createdAt, updatedAt), nil
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
		result = append(result, webhook.ReconstituteFromStrings(id, ownerID, rawURL, events, secret, active, createdAt, updatedAt))
	}
	return result, rows.Err()
}

// FindByID (with ownerID for security scoping).
func (r *webhookRepository) FindByID(ctx context.Context, id, ownerID string) (*webhook.Webhook, error) {
	sql := `SELECT id, owner_id, url, events, secret, active, created_at, updated_at
		FROM webhooks WHERE id = $1`
	args := []interface{}{id}
	if ownerID != "" {
		sql += ` AND owner_id = $2`
		args = append(args, ownerID)
	}
	return r.scanOne(ctx, sql, args...)
}

// FindByEvent returns webhooks subscribed to the given event type.
func (r *webhookRepository) FindByEvent(ctx context.Context, event webhook.EventType) ([]*webhook.Webhook, error) {
	return r.FindActiveByEvent(ctx, string(event))
}

// Update modifies an existing webhook.
func (r *webhookRepository) Update(ctx context.Context, w *webhook.Webhook) error {
	_, err := r.db.Exec(ctx, `
		UPDATE webhooks SET url = $1, events = $2, secret = $3, active = $4, updated_at = NOW()
		WHERE id = $5 AND owner_id = $6`,
		w.URL(), w.Events(), w.Secret(), w.IsActive(), w.ID(), w.OwnerID(),
	)
	return err
}

// SaveDelivery persists a new delivery attempt record.
func (r *webhookRepository) SaveDelivery(ctx context.Context, d *webhook.WebhookDelivery) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO webhook_deliveries
			(id, webhook_id, event_type, payload, status_code, attempt, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
		d.ID, d.WebhookID, string(d.EventType), d.Payload, d.StatusCode, d.Attempt, string(d.Status),
	)
	return err
}

// UpdateDelivery updates an existing delivery attempt.
func (r *webhookRepository) UpdateDelivery(ctx context.Context, d *webhook.WebhookDelivery) error {
	_, err := r.db.Exec(ctx, `
		UPDATE webhook_deliveries
		SET status_code = $1, attempt = $2, status = $3, delivered_at = $4, next_retry_at = $5
		WHERE id = $6`,
		d.StatusCode, d.Attempt, string(d.Status), d.DeliveredAt, d.NextRetryAt, d.ID,
	)
	return err
}

// ListDeliveries returns recent deliveries for a webhook.
func (r *webhookRepository) ListDeliveries(ctx context.Context, webhookID string, limit int) ([]*webhook.WebhookDelivery, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, webhook_id, event_type, payload, status_code, attempt, status, delivered_at, next_retry_at, created_at
		FROM webhook_deliveries WHERE webhook_id = $1 ORDER BY created_at DESC LIMIT $2`,
		webhookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeliveries(rows)
}

// GetPendingRetries returns deliveries that are scheduled for retry.
func (r *webhookRepository) GetPendingRetries(ctx context.Context) ([]*webhook.WebhookDelivery, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, webhook_id, event_type, payload, status_code, attempt, status, delivered_at, next_retry_at, created_at
		FROM webhook_deliveries WHERE status = 'retrying' AND next_retry_at <= NOW()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeliveries(rows)
}

func scanDeliveries(rows interface{ Next() bool; Scan(...interface{}) error; Err() error }) ([]*webhook.WebhookDelivery, error) {
	var result []*webhook.WebhookDelivery
	for rows.Next() {
		d := &webhook.WebhookDelivery{}
		var evtType, status string
		if err := rows.Scan(&d.ID, &d.WebhookID, &evtType, &d.Payload, &d.StatusCode, &d.Attempt, &status, &d.DeliveredAt, &d.NextRetryAt, &d.CreatedAt); err != nil {
			continue
		}
		d.EventType = webhook.EventType(evtType)
		d.Status = webhook.DeliveryStatus(status)
		result = append(result, d)
	}
	return result, rows.Err()
}
