// Package send_digest — usecase.go
// SendDigestUseCase aggregates notifications into daily/weekly digest emails.
// S3-NOTIF-02: Digest Mode — additive alongside existing alert channels.
package send_digest

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// DigestType identifies digest frequency.
type DigestType string

const (
	DigestTypeDaily  DigestType = "daily"
	DigestTypeWeekly DigestType = "weekly"
)

// AlertRecord is a simplified alert for digest grouping.
type AlertRecord struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	ProductName string
	EventType   string
	Title       string
	CreatedAt   time.Time
}

// AlertReader reads alerts for a time window.
type AlertReader interface {
	ListAlertsSince(ctx context.Context, since time.Time, limit int) ([]*AlertRecord, error)
}

// DigestSender dispatches digest emails/messages.
type DigestSender interface {
	// SendDigest delivers a rendered digest to the given email.
	SendDigest(ctx context.Context, to string, subject, htmlBody string) error
}

// UserEmailResolver maps user IDs to email addresses.
type UserEmailResolver interface {
	GetEmail(ctx context.Context, userID uuid.UUID) (string, error)
}

// SendDigestUseCase aggregates and dispatches digests.
type SendDigestUseCase struct {
	alerts   AlertReader
	sender   DigestSender
	resolver UserEmailResolver
	log      zerolog.Logger
}

// New creates a SendDigestUseCase.
func New(alerts AlertReader, sender DigestSender, resolver UserEmailResolver, log zerolog.Logger) *SendDigestUseCase {
	return &SendDigestUseCase{alerts: alerts, sender: sender, resolver: resolver, log: log}
}

// Execute runs the digest for the given type (daily = last 24h, weekly = last 7d).
func (uc *SendDigestUseCase) Execute(ctx context.Context, dtype DigestType) error {
	since := uc.windowStart(dtype)

	records, err := uc.alerts.ListAlertsSince(ctx, since, 10000)
	if err != nil {
		return fmt.Errorf("digest: list alerts: %w", err)
	}
	if len(records) == 0 {
		uc.log.Info().Str("type", string(dtype)).Msg("digest: no alerts, skipping")
		return nil
	}

	// Group by user_id
	byUser := make(map[uuid.UUID][]*AlertRecord)
	for _, r := range records {
		byUser[r.UserID] = append(byUser[r.UserID], r)
	}

	sent, failed := 0, 0
	for userID, alerts := range byUser {
		email, err := uc.resolver.GetEmail(ctx, userID)
		if err != nil {
			uc.log.Warn().Err(err).Str("user_id", userID.String()).Msg("digest: email lookup failed")
			failed++
			continue
		}

		subject := uc.subject(dtype)
		body := uc.renderHTML(dtype, alerts)

		if err := uc.sender.SendDigest(ctx, email, subject, body); err != nil {
			uc.log.Error().Err(err).Str("email", email).Msg("digest: send failed")
			failed++
			continue
		}
		sent++
	}

	uc.log.Info().
		Str("type", string(dtype)).
		Int("sent", sent).
		Int("failed", failed).
		Int("total_users", len(byUser)).
		Msg("digest: completed")

	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (uc *SendDigestUseCase) windowStart(dtype DigestType) time.Time {
	now := time.Now().UTC()
	switch dtype {
	case DigestTypeWeekly:
		return now.AddDate(0, 0, -7)
	default: // daily
		return now.Add(-24 * time.Hour)
	}
}

func (uc *SendDigestUseCase) subject(dtype DigestType) string {
	now := time.Now().UTC()
	switch dtype {
	case DigestTypeWeekly:
		return fmt.Sprintf("[OSV] Weekly Security Digest — Week of %s", now.Format("Jan 02, 2006"))
	default:
		return fmt.Sprintf("[OSV] Daily Security Digest — %s", now.Format("Jan 02, 2006"))
	}
}

func (uc *SendDigestUseCase) renderHTML(dtype DigestType, alerts []*AlertRecord) string {
	period := "last 24 hours"
	if dtype == DigestTypeWeekly {
		period = "last 7 days"
	}

	html := fmt.Sprintf(`<html><body>
<h1>Security Alert Digest</h1>
<p>Alerts from the <strong>%s</strong>:</p>
<table border="1" cellpadding="4" cellspacing="0">
<tr><th>Time</th><th>Product</th><th>Event</th><th>Title</th></tr>
`, period)

	for _, a := range alerts {
		html += fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			a.CreatedAt.Format(time.RFC3339),
			a.ProductName,
			a.EventType,
			a.Title,
		)
	}

	html += `</table>
<p>Log in to <a href="https://osv.example.com">OSV Platform</a> to review and take action.</p>
</body></html>`

	return html
}
