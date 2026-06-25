package webhook

import (
	"time"
)

// EventType constants
type EventType string

const (
	EventNewKEV      EventType = "kev.new"
	EventNewCritical EventType = "cve.new.critical"
	EventNewHigh     EventType = "cve.new.high"
	EventHighEPSS    EventType = "cve.epss.high"
	EventVendorCVE   EventType = "cve.vendor"
	EventProductCVE  EventType = "cve.product"
)

// Webhook represents a registered webhook endpoint.
type Webhook struct {
	ID        string
	URL       string
	Secret    string      // HMAC-SHA256 signing key
	Events    []EventType // subscribed event types
	IsActive  bool
	OwnerID   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// HasEvent returns true if webhook subscribes to this event type.
func (w *Webhook) HasEvent(e EventType) bool {
	for _, ev := range w.Events {
		if ev == e {
			return true
		}
	}
	return false
}

// DeliveryStatus constants
type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryFailed    DeliveryStatus = "failed"
	DeliveryRetrying  DeliveryStatus = "retrying"
)

// WebhookDelivery records each delivery attempt.
type WebhookDelivery struct {
	ID          string
	WebhookID   string
	EventType   EventType
	Payload     string // JSON payload
	StatusCode  *int
	Attempt     int
	Status      DeliveryStatus // "pending"|"delivered"|"failed"|"retrying"
	DeliveredAt *time.Time
	NextRetryAt *time.Time
	CreatedAt   time.Time
}
