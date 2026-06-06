// domain/aggregate/webhook/webhook.go
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidURL    = errors.New("webhook URL must be HTTPS")
	ErrSecretTooShort = errors.New("webhook secret must be at least 32 bytes")
	ErrInactive      = errors.New("webhook is inactive")
)

// Webhook aggregate represents a registered webhook endpoint.
type Webhook struct {
	id           string
	url          string // HTTPS URL
	secret       []byte // HMAC secret (>= 32 bytes)
	eventFilter  map[string]struct{} // allowed event types
	isActive     bool
	maxPerMinute int
	createdAt    time.Time
	events       []interface{}
}

// New creates a new Webhook with validation.
func New(url string, secret []byte, eventTypes []string, maxPerMinute int) (*Webhook, error) {
	if !strings.HasPrefix(url, "https://") {
		return nil, ErrInvalidURL
	}
	if len(secret) < 32 {
		return nil, ErrSecretTooShort
	}

	filter := make(map[string]struct{}, len(eventTypes))
	for _, et := range eventTypes {
		filter[et] = struct{}{}
	}

	return &Webhook{
		id:           uuid.NewString(),
		url:          url,
		secret:       secret,
		eventFilter:  filter,
		isActive:     true,
		maxPerMinute: maxPerMinute,
		createdAt:    time.Now().UTC(),
	}, nil
}

// Reconstitute rebuilds a Webhook from persisted state.
func Reconstitute(id, url string, secret []byte, eventTypes []string, isActive bool, maxPerMinute int, createdAt time.Time) *Webhook {
	filter := make(map[string]struct{}, len(eventTypes))
	for _, et := range eventTypes {
		filter[et] = struct{}{}
	}
	return &Webhook{
		id: id, url: url, secret: secret,
		eventFilter: filter, isActive: isActive,
		maxPerMinute: maxPerMinute, createdAt: createdAt,
	}
}

func (w *Webhook) ID() string        { return w.id }
func (w *Webhook) URL() string       { return w.url }
func (w *Webhook) IsActive() bool    { return w.isActive }
func (w *Webhook) MaxPerMinute() int { return w.maxPerMinute }
func (w *Webhook) CreatedAt() time.Time { return w.createdAt }

// EventTypes returns the list of allowed event type strings.
func (w *Webhook) EventTypes() []string {
	types := make([]string, 0, len(w.eventFilter))
	for et := range w.eventFilter {
		types = append(types, et)
	}
	return types
}

// ShouldDeliver returns true if the webhook is active and the event type is in the filter.
// An empty filter means "receive all event types".
func (w *Webhook) ShouldDeliver(eventType string) bool {
	if !w.isActive {
		return false
	}
	if len(w.eventFilter) == 0 {
		return true // receive all
	}
	_, ok := w.eventFilter[eventType]
	return ok
}

// Sign creates an HMAC-SHA256 signature of the payload.
// Returns: "sha256={hex_encoded_mac}"
// Header to use: X-OSV-Signature-256
func (w *Webhook) Sign(payload []byte) string {
	mac := hmac.New(sha256.New, w.secret)
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Activate enables the webhook.
func (w *Webhook) Activate() {
	w.isActive = true
}

// Deactivate disables the webhook (temporary failure, rate limit, etc.).
func (w *Webhook) Deactivate() {
	w.isActive = false
}
