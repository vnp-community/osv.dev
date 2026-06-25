// Package notification implements the HTTP client to call notification-service after CVE sync.
package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/osv/data-service/internal/domain/entity"
	"github.com/rs/zerolog"
)

// Hook calls notification-service to dispatch CVE alerts.
// Implements fire-and-forget pattern — non-blocking, non-fatal.
type Hook struct {
	baseURL string // notification-service HTTP base URL
	client  *http.Client
	logger  zerolog.Logger
}

// NewHook creates a notification hook.
// notificationServiceURL: e.g. "http://notification-service:8084"
func NewHook(notificationServiceURL string, log zerolog.Logger) *Hook {
	return &Hook{
		baseURL: notificationServiceURL,
		client:  &http.Client{Timeout: 10 * time.Second},
		logger:  log.With().Str("component", "notification-hook").Logger(),
	}
}

// IsEnabled returns true if NOTIFICATION_SERVICE_URL env is set.
func IsEnabled() bool {
	return os.Getenv("NOTIFICATION_SERVICE_URL") != ""
}

// CVENotification is the payload sent to notification-service.
type CVENotification struct {
	CVEID       string   `json:"cve_id"`
	Severity    string   `json:"severity"`
	EPSS        float64  `json:"epss"`
	Vendors     []string `json:"vendors"`
	Products    []string `json:"products"`
	IsKEV       bool     `json:"is_kev"`
	IsExploit   bool     `json:"is_exploit"`
	Description string   `json:"description"`
}

// NotifyBatch sends a batch of CVE notifications to notification-service.
// Non-blocking: runs in goroutine. Non-fatal: errors are only logged.
func (h *Hook) NotifyBatch(cves []*entity.CVE) {
	if len(cves) == 0 {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		notifications := make([]*CVENotification, 0, len(cves))
		for _, cve := range cves {
			// Only notify for high-priority CVEs
			if !isHighPriority(cve) {
				continue
			}
			var epss float64
			if cve.EPSS != 0 {
				epss = cve.EPSS
			}

			notifications = append(notifications, &CVENotification{
				CVEID:       cve.ID,
				Severity:    string(cve.Severity),
				EPSS:        epss,
				Vendors:     cve.Vendors,
				Products:    cve.Products,
				IsKEV:       cve.IsKEV,
				IsExploit:   cve.IsExploit,
				Description: cve.Description,
			})
		}
		if len(notifications) == 0 {
			return
		}

		if err := h.sendNotifications(ctx, notifications); err != nil {
			h.logger.Warn().Err(err).Int("count", len(notifications)).
				Msg("notification hook failed (non-fatal)")
		} else {
			h.logger.Info().Int("count", len(notifications)).
				Msg("notifications dispatched")
		}
	}()
}

func (h *Hook) sendNotifications(ctx context.Context, notifications []*CVENotification) error {
	body, err := json.Marshal(map[string]interface{}{
		"events": notifications,
	})
	if err != nil {
		return err
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		h.baseURL+"/internal/events/cve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Service", "data-service")

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("notification hook: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("notification hook: status %d", resp.StatusCode)
	}
	return nil
}

// isHighPriority returns true if CVE warrants notification.
func isHighPriority(cve *entity.CVE) bool {
	return cve.Severity == entity.SeverityCritical ||
		cve.Severity == entity.SeverityHigh ||
		cve.EPSS >= 0.7 ||
		cve.IsKEV ||
		cve.IsExploit
}
