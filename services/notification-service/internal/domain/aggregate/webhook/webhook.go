// Package webhook defines the Webhook aggregate root.
// A Webhook is a registered HTTP endpoint that receives CVE event notifications.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	wh "github.com/osv/notification-service/internal/domain/webhook"
)

// EventType re-exports the domain webhook event type.
type EventType = wh.EventType

// Event type constants.
const (
	EventNewKEV      = wh.EventNewKEV
	EventNewCritical = wh.EventNewCritical
	EventNewHigh     = wh.EventNewHigh
	EventHighEPSS    = wh.EventHighEPSS
	EventVendorCVE   = wh.EventVendorCVE
	EventProductCVE  = wh.EventProductCVE
)

// Webhook is the aggregate root representing a registered webhook endpoint.
// Fields are private; access via accessor methods to preserve encapsulation.
type Webhook struct {
	id        string
	ownerID   string
	url       string
	events    []EventType
	secret    string
	isActive  bool
	createdAt time.Time
	updatedAt time.Time
}

// New creates a new active Webhook aggregate.
func New(rawURL string, events []EventType, secret, ownerID string) (*Webhook, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("webhook: URL is required")
	}
	if ownerID == "" {
		return nil, fmt.Errorf("webhook: ownerID is required")
	}
	now := time.Now().UTC()
	return &Webhook{
		id:        uuid.NewString(),
		ownerID:   ownerID,
		url:       rawURL,
		events:    events,
		secret:    secret,
		isActive:  true,
		createdAt: now,
		updatedAt: now,
	}, nil
}

// Reconstitute rebuilds a Webhook from persisted data (no validation).
func Reconstitute(id, ownerID, url string, events []EventType, secret string, active bool, createdAt, updatedAt time.Time) *Webhook {
	return &Webhook{
		id:        id,
		ownerID:   ownerID,
		url:       url,
		events:    events,
		secret:    secret,
		isActive:  active,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

// ReconstituteFromStrings rebuilds a Webhook accepting []string events (e.g., from PostgreSQL pgx scan).
func ReconstituteFromStrings(id, ownerID, url string, events []string, secret string, active bool, createdAt, updatedAt time.Time) *Webhook {
	evtTypes := make([]EventType, len(events))
	for i, e := range events {
		evtTypes[i] = EventType(e)
	}
	return Reconstitute(id, ownerID, url, evtTypes, secret, active, createdAt, updatedAt)
}

// ── Accessors ──────────────────────────────────────────────────────────────────

func (w *Webhook) ID() string            { return w.id }
func (w *Webhook) OwnerID() string       { return w.ownerID }
func (w *Webhook) URL() string           { return w.url }
func (w *Webhook) Events() []EventType   { return w.events }
func (w *Webhook) Secret() string        { return w.secret }
func (w *Webhook) IsActive() bool        { return w.isActive }
func (w *Webhook) CreatedAt() time.Time  { return w.createdAt }
func (w *Webhook) UpdatedAt() time.Time  { return w.updatedAt }

// ── Domain methods ─────────────────────────────────────────────────────────────

// ShouldDeliver returns true if this webhook is active and subscribes to the given event.
func (w *Webhook) ShouldDeliver(eventType string) bool {
	if !w.isActive {
		return false
	}
	for _, e := range w.events {
		if string(e) == eventType {
			return true
		}
	}
	return false
}

// Sign generates an HMAC-SHA256 signature for the payload using the webhook secret.
// Returns the hex-encoded signature (used in X-Hub-Signature-256 header).
func (w *Webhook) Sign(payload []byte) string {
	if w.secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(w.secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Activate enables this webhook for delivery.
func (w *Webhook) Activate() {
	w.isActive = true
	w.updatedAt = time.Now().UTC()
}

// Deactivate disables this webhook.
func (w *Webhook) Deactivate() {
	w.isActive = false
	w.updatedAt = time.Now().UTC()
}

// UpdateSecret rotates the signing secret.
func (w *Webhook) UpdateSecret(newSecret string) {
	w.secret = newSecret
	w.updatedAt = time.Now().UTC()
}

// ── Delivery types (re-exported from domain/webhook) ──────────────────────────

// DeliveryStatus re-exports the delivery status type.
type DeliveryStatus = wh.DeliveryStatus

const (
	DeliveryPending   = wh.DeliveryPending
	DeliveryDelivered = wh.DeliveryDelivered
	DeliveryFailed    = wh.DeliveryFailed
	DeliveryRetrying  = wh.DeliveryRetrying
)

// WebhookDelivery re-exports the delivery record type.
type WebhookDelivery = wh.WebhookDelivery
