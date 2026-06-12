// Package webhook provides the Webhook aggregate for the notification service.
// This is a clean-architecture rewrite of the original webhook domain.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Sentinel domain errors.
var (
	ErrInvalidURL    = errors.New("webhook URL must start with https://")
	ErrEmptyEvents   = errors.New("at least one event type required")
	ErrInactive      = errors.New("webhook is inactive")
)

// SupportedEvents lists valid event type strings.
var SupportedEvents = map[string]bool{
	"cve.created":    true,
	"cve.updated":    true,
	"kev.added":      true,
	"sync.completed": true,
}

// Webhook is the aggregate root for a registered webhook endpoint.
type Webhook struct {
	id        string
	ownerID   string
	rawURL    string
	events    []string
	secret    string
	active    bool
	createdAt time.Time
	updatedAt time.Time
}

// New creates and validates a new Webhook aggregate.
func New(rawURL string, events []string, secret, ownerID string) (*Webhook, error) {
	if !strings.HasPrefix(rawURL, "https://") {
		return nil, ErrInvalidURL
	}
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return nil, ErrInvalidURL
	}
	if len(events) == 0 {
		return nil, ErrEmptyEvents
	}

	now := time.Now().UTC()
	return &Webhook{
		id:        uuid.NewString(),
		ownerID:   ownerID,
		rawURL:    rawURL,
		events:    events,
		secret:    secret,
		active:    true,
		createdAt: now,
		updatedAt: now,
	}, nil
}

// Reconstitute rebuilds a Webhook from persisted state (no validation).
func Reconstitute(id, ownerID, rawURL string, events []string, secret string, active bool, createdAt, updatedAt time.Time) *Webhook {
	return &Webhook{
		id: id, ownerID: ownerID, rawURL: rawURL,
		events: events, secret: secret, active: active,
		createdAt: createdAt, updatedAt: updatedAt,
	}
}

// Accessors.
func (w *Webhook) ID() string          { return w.id }
func (w *Webhook) OwnerID() string     { return w.ownerID }
func (w *Webhook) URL() string         { return w.rawURL }
func (w *Webhook) Events() []string    { return w.events }
func (w *Webhook) Secret() string      { return w.secret }
func (w *Webhook) IsActive() bool      { return w.active }
func (w *Webhook) CreatedAt() time.Time { return w.createdAt }
func (w *Webhook) UpdatedAt() time.Time { return w.updatedAt }

// ShouldDeliver returns true if this webhook is active and subscribes to eventType.
func (w *Webhook) ShouldDeliver(eventType string) bool {
	if !w.active {
		return false
	}
	for _, e := range w.events {
		if e == eventType {
			return true
		}
	}
	return false
}

// Sign creates an HMAC-SHA256 signature of payload.
// Returns: "sha256=<hex>"
func (w *Webhook) Sign(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(w.secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Deactivate marks the webhook as inactive.
func (w *Webhook) Deactivate() {
	w.active = false
	w.updatedAt = time.Now().UTC()
}
