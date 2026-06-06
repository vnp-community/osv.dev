// Package errors defines domain sentinel errors for the notification service.
package errors

import "errors"

var (
	ErrWebhookNotFound    = errors.New("webhook not found")
	ErrWebhookInvalidURL  = errors.New("webhook URL must start with https://")
	ErrWebhookInvalidEvent = errors.New("unknown event type")
	ErrUnauthorized       = errors.New("unauthorized: owner mismatch")
)
