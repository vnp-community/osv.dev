package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/notification-service/internal/domain/aggregate/webhook"
	"github.com/osv/notification-service/internal/domain/repository"
)

type pgWebhookRepository struct{ db *pgxpool.Pool }

// NewWebhookRepository creates a PostgreSQL-backed WebhookRepository.
func NewWebhookRepository(db *pgxpool.Pool) repository.WebhookRepository {
	return &pgWebhookRepository{db: db}
}

func (r *pgWebhookRepository) Save(ctx context.Context, wh *webhook.Webhook) error {
	events := make([]string, len(wh.Events()))
	for i, e := range wh.Events() {
		events[i] = string(e)
	}
	_, err := r.db.Exec(ctx, `
        INSERT INTO webhooks (id, url, secret, events, active, owner_id, created_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
        ON CONFLICT (id) DO UPDATE SET url=EXCLUDED.url, events=EXCLUDED.events, active=EXCLUDED.active, updated_at=NOW()
    `, wh.ID(), wh.URL(), wh.Secret(), events, wh.IsActive(), wh.OwnerID(), wh.CreatedAt(), wh.UpdatedAt())
	return err
}

func (r *pgWebhookRepository) FindByID(ctx context.Context, id, ownerID string) (*webhook.Webhook, error) {
	var (
		whID, url, secret, owner string
		events                   []string
		isActive                 bool
		createdAt, updatedAt     time.Time
	)
	query := `SELECT id, url, secret, events, active, owner_id, created_at, updated_at
	          FROM webhooks WHERE id = $1`
	args := []interface{}{id}
	if ownerID != "" {
		query += ` AND owner_id = $2`
		args = append(args, ownerID)
	}
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&whID, &url, &secret, &events, &isActive, &owner, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("webhook not found")
		}
		return nil, err
	}
	return webhook.ReconstituteFromStrings(whID, owner, url, events, secret, isActive, createdAt, updatedAt), nil
}

func (r *pgWebhookRepository) FindByOwner(ctx context.Context, ownerID string) ([]*webhook.Webhook, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, url, secret, events, active, owner_id, created_at, updated_at
		FROM webhooks
		WHERE owner_id = $1 AND active = TRUE
	`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanWebhooks(rows)
}

func (r *pgWebhookRepository) FindByEvent(ctx context.Context, event webhook.EventType) ([]*webhook.Webhook, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, url, secret, events, active, owner_id, created_at, updated_at
        FROM webhooks
        WHERE $1 = ANY(events) AND active = TRUE
    `, string(event))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanWebhooks(rows)
}

func (r *pgWebhookRepository) FindActiveByEvent(ctx context.Context, event string) ([]*webhook.Webhook, error) {
	return r.FindByEvent(ctx, webhook.EventType(event))
}

func (r *pgWebhookRepository) scanWebhooks(rows pgx.Rows) ([]*webhook.Webhook, error) {
	var result []*webhook.Webhook
	for rows.Next() {
		var (
			id, url, secret, ownerID string
			events                   []string
			isActive                 bool
			createdAt, updatedAt     time.Time
		)
		if err := rows.Scan(&id, &url, &secret, &events, &isActive, &ownerID, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		result = append(result, webhook.ReconstituteFromStrings(id, ownerID, url, events, secret, isActive, createdAt, updatedAt))
	}
	return result, rows.Err()
}

func (r *pgWebhookRepository) Update(ctx context.Context, wh *webhook.Webhook) error {
	events := make([]string, len(wh.Events()))
	for i, e := range wh.Events() {
		events[i] = string(e)
	}
	_, err := r.db.Exec(ctx, `
		UPDATE webhooks SET url = $1, events = $2, active = $3, updated_at = NOW()
		WHERE id = $4
	`, wh.URL(), events, wh.IsActive(), wh.ID())
	return err
}

func (r *pgWebhookRepository) Delete(ctx context.Context, id, ownerID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM webhooks WHERE id = $1 AND owner_id = $2`, id, ownerID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("not found or unauthorized")
	}
	return nil
}

func (r *pgWebhookRepository) SaveDelivery(ctx context.Context, d *webhook.WebhookDelivery) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, status_code, attempt, status, delivered_at, next_retry_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, d.ID, d.WebhookID, string(d.EventType), d.Payload, d.StatusCode, d.Attempt, string(d.Status), d.DeliveredAt, d.NextRetryAt, d.CreatedAt)
	return err
}

func (r *pgWebhookRepository) ListDeliveries(ctx context.Context, webhookID string, limit int) ([]*webhook.WebhookDelivery, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, webhook_id, event_type, payload, status_code, attempt, status, delivered_at, next_retry_at, created_at
		FROM webhook_deliveries
		WHERE webhook_id = $1
		ORDER BY created_at DESC LIMIT $2
	`, webhookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeliveryRows(rows)
}

func (r *pgWebhookRepository) GetPendingRetries(ctx context.Context) ([]*webhook.WebhookDelivery, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, webhook_id, event_type, payload, status_code, attempt, status, delivered_at, next_retry_at, created_at
		FROM webhook_deliveries
		WHERE status = 'retrying' AND next_retry_at <= NOW()
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeliveryRows(rows)
}

func (r *pgWebhookRepository) UpdateDelivery(ctx context.Context, d *webhook.WebhookDelivery) error {
	_, err := r.db.Exec(ctx, `
		UPDATE webhook_deliveries
		SET status_code = $1, attempt = $2, status = $3, delivered_at = $4, next_retry_at = $5
		WHERE id = $6
	`, d.StatusCode, d.Attempt, string(d.Status), d.DeliveredAt, d.NextRetryAt, d.ID)
	return err
}

func scanDeliveryRows(rows pgx.Rows) ([]*webhook.WebhookDelivery, error) {
	var deliveries []*webhook.WebhookDelivery
	for rows.Next() {
		d := &webhook.WebhookDelivery{}
		var status, event string
		if err := rows.Scan(
			&d.ID, &d.WebhookID, &event, &d.Payload, &d.StatusCode, &d.Attempt, &status, &d.DeliveredAt, &d.NextRetryAt, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		d.EventType = webhook.EventType(event)
		d.Status = webhook.DeliveryStatus(status)
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}
