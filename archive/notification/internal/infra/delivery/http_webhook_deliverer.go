// infra/delivery/http_webhook_deliverer.go
package delivery

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/osv/notification/internal/domain/aggregate/webhook"
	"github.com/rs/zerolog"
)

// HTTPWebhookDeliverer delivers notifications to webhook endpoints via HTTP POST.
type HTTPWebhookDeliverer struct {
	client *http.Client
	log    zerolog.Logger
}

// NewHTTPWebhookDeliverer creates a deliverer with a configured HTTP client.
func NewHTTPWebhookDeliverer(log zerolog.Logger) *HTTPWebhookDeliverer {
	return &HTTPWebhookDeliverer{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		log: log,
	}
}

// ErrClientError is returned for 4xx responses (no retry needed).
type ErrClientError struct {
	StatusCode int
}

func (e ErrClientError) Error() string {
	return fmt.Sprintf("webhook client error: HTTP %d", e.StatusCode)
}

// ErrMaxRetriesExceeded is returned after all retry attempts fail.
var ErrMaxRetriesExceeded = fmt.Errorf("max retries exceeded")

// Deliver sends an HTTP POST to the webhook URL with HMAC signature and retry logic.
// Retries: up to 3 attempts with exponential backoff (1s, 2s, 4s).
// 4xx errors: no retry.
// 5xx errors: retry.
func (d *HTTPWebhookDeliverer) Deliver(ctx context.Context, hook *webhook.Webhook, payload []byte, eventType string) error {
	signature := hook.Sign(payload)

	backoffs := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	for attempt, backoff := range backoffs {
		err := d.doDeliver(ctx, hook.URL(), signature, eventType, payload)
		if err == nil {
			return nil
		}

		// Client errors: don't retry
		if _, ok := err.(ErrClientError); ok {
			return err
		}

		d.log.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Str("webhook_id", hook.ID()).
			Dur("backoff", backoff).
			Msg("webhook delivery failed, retrying")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}

	return ErrMaxRetriesExceeded
}

func (d *HTTPWebhookDeliverer) doDeliver(ctx context.Context, url, signature, eventType string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OSV-Signature-256", signature)
	req.Header.Set("X-OSV-Event", eventType)
	req.Header.Set("User-Agent", "osv.dev/notification-service/1.0")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP POST failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return ErrClientError{StatusCode: resp.StatusCode}
	}
	return fmt.Errorf("server error: HTTP %d", resp.StatusCode)
}
