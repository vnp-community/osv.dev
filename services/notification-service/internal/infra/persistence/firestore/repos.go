// Package firestore implements WebhookRepo and NotificationRepo for the Notification Service.
package firestore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/osv/notification-service/internal/domain/aggregate/webhook"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	collWebhooks      = "webhooks"
	collNotifications = "notifications"
)

// WebhookRepo persists Webhook aggregates in Firestore.
type WebhookRepo struct {
	client *firestore.Client
	log    zerolog.Logger
}

// NewWebhookRepo creates a Firestore-backed webhook repository.
func NewWebhookRepo(client *firestore.Client, log zerolog.Logger) *WebhookRepo {
	return &WebhookRepo{client: client, log: log}
}

// Save persists a webhook (upsert by ID).
func (r *WebhookRepo) Save(ctx context.Context, wh *webhook.Webhook) error {
	_, err := r.client.Collection(collWebhooks).Doc(wh.ID()).Set(ctx, map[string]interface{}{
		"id":            wh.ID(),
		"owner_id":      wh.OwnerID(),
		"callback_url":  wh.URL(),
		"secret":        wh.Secret(),
		"event_filters": wh.Events(),
		"is_active":     wh.IsActive(),
		"created_at":    wh.CreatedAt(),
		"updated_at":    time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("save webhook %s: %w", wh.ID(), err)
	}
	return nil
}

// GetByID retrieves a webhook by ID.
func (r *WebhookRepo) GetByID(ctx context.Context, id string) (*webhook.Webhook, error) {
	doc, err := r.client.Collection(collWebhooks).Doc(id).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, fmt.Errorf("webhook %s not found", id)
		}
		return nil, fmt.Errorf("get webhook %s: %w", id, err)
	}
	return reconstitueWebhook(doc)
}

// ListActive returns all active webhooks.
func (r *WebhookRepo) ListActive(ctx context.Context) ([]*webhook.Webhook, error) {
	docs, err := r.client.Collection(collWebhooks).
		Where("is_active", "==", true).
		Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}

	webhooks := make([]*webhook.Webhook, 0, len(docs))
	for _, doc := range docs {
		wh, err := reconstitueWebhook(doc)
		if err != nil {
			r.log.Warn().Err(err).Str("id", doc.Ref.ID).Msg("skip: unmarshal failed")
			continue
		}
		webhooks = append(webhooks, wh)
	}
	return webhooks, nil
}

// ListByOwner returns all webhooks for an owner.
func (r *WebhookRepo) ListByOwner(ctx context.Context, ownerID string) ([]*webhook.Webhook, error) {
	docs, err := r.client.Collection(collWebhooks).
		Where("owner_id", "==", ownerID).
		Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("list owner webhooks %s: %w", ownerID, err)
	}

	webhooks := make([]*webhook.Webhook, 0, len(docs))
	for _, doc := range docs {
		wh, err := reconstitueWebhook(doc)
		if err != nil {
			continue
		}
		webhooks = append(webhooks, wh)
	}
	return webhooks, nil
}

// ── NotificationRepo ──────────────────────────────────────────────────────────

// NotificationRecord is a delivery attempt record.
type NotificationRecord struct {
	ID          string    `firestore:"id"`
	WebhookID   string    `firestore:"webhook_id"`
	EventID     string    `firestore:"event_id"`
	EventType   string    `firestore:"event_type"`
	VulnID      string    `firestore:"vuln_id"`
	Status      string    `firestore:"status"` // DELIVERED | FAILED | PENDING
	Attempts    int       `firestore:"attempts"`
	LastAttempt time.Time `firestore:"last_attempt"`
	NextRetry   time.Time `firestore:"next_retry,omitempty"`
	Error       string    `firestore:"error,omitempty"`
	CreatedAt   time.Time `firestore:"created_at"`
}

// NotificationRepo persists delivery attempt records.
type NotificationRepo struct {
	client *firestore.Client
	log    zerolog.Logger
}

// NewNotificationRepo creates a Firestore-backed notification repository.
func NewNotificationRepo(client *firestore.Client, log zerolog.Logger) *NotificationRepo {
	return &NotificationRepo{client: client, log: log}
}

// Save records a delivery attempt.
func (r *NotificationRepo) Save(ctx context.Context, rec *NotificationRecord) error {
	_, err := r.client.Collection(collNotifications).Doc(rec.ID).Set(ctx, rec)
	if err != nil {
		return fmt.Errorf("save notification %s: %w", rec.ID, err)
	}
	return nil
}

// ListFailed returns notifications pending retry.
func (r *NotificationRepo) ListFailed(ctx context.Context, limit int) ([]*NotificationRecord, error) {
	now := time.Now().UTC()
	docs, err := r.client.Collection(collNotifications).
		Where("status", "==", "FAILED").
		Where("next_retry", "<=", now).
		Limit(limit).
		Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("list failed notifications: %w", err)
	}

	records := make([]*NotificationRecord, 0, len(docs))
	for _, doc := range docs {
		var rec NotificationRecord
		if err := doc.DataTo(&rec); err != nil {
			continue
		}
		records = append(records, &rec)
	}
	return records, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func reconstitueWebhook(doc *firestore.DocumentSnapshot) (*webhook.Webhook, error) {
	data := doc.Data()
	return webhook.ReconstituteFromStrings(
		doc.Ref.ID,
		stringField(data, "owner_id"),
		stringField(data, "callback_url"),
		stringSliceField(data, "event_filters"),
		stringField(data, "secret"),
		boolField(data, "is_active"),
		timeField(data, "created_at"),
		timeField(data, "updated_at"),
	), nil
}

func stringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func boolField(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func timeField(m map[string]interface{}, key string) time.Time {
	if v, ok := m[key].(time.Time); ok {
		return v
	}
	return time.Time{}
}

func stringSliceField(m map[string]interface{}, key string) []string {
	v, ok := m[key].([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(v))
	for _, item := range v {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
