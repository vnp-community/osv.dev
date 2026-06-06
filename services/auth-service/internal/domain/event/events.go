// Package event defines domain events published by the auth service.
// Events are published to NATS JetStream for consumption by other services.
package event

import "time"

// EventType identifies the event variant.
type EventType string

const (
	EventUserRegistered EventType = "auth.user.registered"
	EventUserLoggedIn   EventType = "auth.user.logged_in"
	EventTokenRevoked   EventType = "auth.token.revoked"
	EventAPIKeyCreated  EventType = "auth.api_key.created"
	EventAPIKeyRevoked  EventType = "auth.api_key.revoked"
)

// BaseEvent contains fields common to all auth events.
type BaseEvent struct {
	Type      EventType `json:"type"`
	OccurredAt time.Time `json:"occurred_at"`
	ServiceID string    `json:"service_id"` // "auth-service"
}

// UserRegisteredEvent is published when a new user completes registration.
type UserRegisteredEvent struct {
	BaseEvent
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Provider string `json:"provider"` // "local" | "google" | "github"
}

// UserLoggedInEvent is published on successful authentication.
type UserLoggedInEvent struct {
	BaseEvent
	UserID    string `json:"user_id"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
}

// TokenRevokedEvent is published when a JWT is explicitly blacklisted (logout).
type TokenRevokedEvent struct {
	BaseEvent
	UserID    string    `json:"user_id"`
	JTI       string    `json:"jti"`
	ExpiresAt time.Time `json:"expires_at"`
}

// APIKeyCreatedEvent is published when a new API key is generated.
type APIKeyCreatedEvent struct {
	BaseEvent
	UserID    string `json:"user_id"`
	KeyID     string `json:"key_id"`
	KeyPrefix string `json:"key_prefix"`
	Name      string `json:"name"`
}

// APIKeyRevokedEvent is published when an API key is revoked.
type APIKeyRevokedEvent struct {
	BaseEvent
	UserID string `json:"user_id"`
	KeyID  string `json:"key_id"`
}

// NewBase creates a BaseEvent with the current timestamp.
func NewBase(eventType EventType) BaseEvent {
	return BaseEvent{
		Type:       eventType,
		OccurredAt: time.Now().UTC(),
		ServiceID:  "auth-service",
	}
}
