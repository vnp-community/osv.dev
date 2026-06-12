// Package dispatcher provides the HTTP dispatcher for CVE webhook events.
package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/notification-service/internal/domain/aggregate/webhook"
)

// Event types constants.
const (
	EventCVECreated    = "cve.created"
	EventCVEUpdated    = "cve.updated"
	EventKEVAdded      = "kev.added"
	EventSyncCompleted = "sync.completed"
)

// CVEEvent is the payload sent to webhook endpoints.
type CVEEvent struct {
	EventType string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// HTTPDispatcher delivers CVEEvent payloads to webhook URLs via HTTP POST.
type HTTPDispatcher struct {
	httpClient *http.Client
	log        zerolog.Logger
}

// New creates an HTTPDispatcher with the given timeout.
func New(timeout time.Duration, log zerolog.Logger) *HTTPDispatcher {
	return &HTTPDispatcher{
		httpClient: &http.Client{Timeout: timeout},
		log:        log.With().Str("component", "http_dispatcher").Logger(),
	}
}

// Dispatch POSTs a signed CVEEvent to the webhook URL.
func (d *HTTPDispatcher) Dispatch(ctx context.Context, w *webhook.Webhook, event *CVEEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("dispatch: marshal: %w", err)
	}

	sig := w.Sign(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("dispatch: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GlobalCVE-Signature", sig)
	req.Header.Set("X-GlobalCVE-Event", event.EventType)
	req.Header.Set("X-GlobalCVE-Delivery", fmt.Sprintf("%s-%d", w.ID(), time.Now().UnixNano()))

	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.log.Warn().Err(err).Str("webhook_id", w.ID()).Str("url", w.URL()).Msg("dispatch failed")
		return fmt.Errorf("dispatch: http: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		d.log.Warn().
			Str("webhook_id", w.ID()).
			Int("status", resp.StatusCode).
			Msg("dispatch received error response")
		return fmt.Errorf("dispatch: upstream returned %d", resp.StatusCode)
	}

	d.log.Debug().
		Str("webhook_id", w.ID()).
		Str("event", event.EventType).
		Int("status", resp.StatusCode).
		Msg("dispatch ok")
	return nil
}
