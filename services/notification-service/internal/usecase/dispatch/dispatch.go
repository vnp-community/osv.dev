// Package dispatch implements the core notification dispatch use case.
// It matches events to rules, creates in-app alerts, and delivers to channels with retry.
package dispatch

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/osv/notification-service/internal/domain/rule"
)

// ─── Types ────────────────────────────────────────────────────────────────────

// NotificationEvent is an event that may trigger notifications.
type NotificationEvent struct {
	Type         rule.EventType
	ProductID    *string
	FindingID    *string
	EngagementID *string
	Title        string
	Description  string
	URL          string
	Severity     *string
	Metadata     map[string]interface{}
}

// Recipient holds the target user for notification delivery.
type Recipient struct {
	UserID    uuid.UUID
	Email     string
	FirstName string
}

// ─── Interfaces ────────────────────────────────────────────────────────────────

// IdentityClient fetches product members.
type IdentityClient interface {
	GetUsersForProduct(ctx context.Context, productID string) ([]Recipient, error)
}

// ChannelSender delivers a notification to a specific channel.
type ChannelSender interface {
	Send(ctx context.Context, recipient string, payload map[string]interface{}) error
}

// TemplateRenderer renders event+channel into delivery payload.
type TemplateRenderer interface {
	Render(et rule.EventType, ch rule.Channel, data *TemplateData) (map[string]interface{}, error)
}

// TemplateData is the context passed to templates.
type TemplateData struct {
	Event     *NotificationEvent
	Recipient Recipient
}

// AlertRepository saves in-app alerts.
type AlertRepository interface {
	Save(ctx context.Context, a *Alert) error
}

// Alert is an in-app notification stored per user.
type Alert struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	EventType   string
	Title       string
	Description string
	URL         string
	IsRead      bool
	CreatedAt   time.Time
}

// DeliveryRecord tracks the delivery status of a channel notification.
type DeliveryRecord struct {
	ID            uuid.UUID
	EventType     string
	Channel       string
	Recipient     string
	Status        string // pending|retrying|sent|failed
	Attempts      int
	LastAttemptAt *time.Time
	ErrorMessage  string
	Payload       map[string]interface{}
	CreatedAt     time.Time
}

// DeliveryRepository persists delivery records.
type DeliveryRepository interface {
	Save(ctx context.Context, r *DeliveryRecord) error
	Update(ctx context.Context, r *DeliveryRecord) error
}

// ─── DispatchUseCase ──────────────────────────────────────────────────────────

// DispatchUseCase dispatches a NotificationEvent to all matching rule recipients.
type DispatchUseCase struct {
	ruleRepo       rule.Repository
	alertRepo      AlertRepository
	deliveryRepo   DeliveryRepository
	identityClient IdentityClient
	channelSenders map[rule.Channel]ChannelSender
	tmplRenderer   TemplateRenderer
}

// New creates a new DispatchUseCase.
func New(
	rr rule.Repository,
	ar AlertRepository,
	dr DeliveryRepository,
	ic IdentityClient,
	senders map[rule.Channel]ChannelSender,
	tmpl TemplateRenderer,
) *DispatchUseCase {
	return &DispatchUseCase{
		ruleRepo:       rr,
		alertRepo:      ar,
		deliveryRepo:   dr,
		identityClient: ic,
		channelSenders: senders,
		tmplRenderer:   tmpl,
	}
}

// Execute dispatches the event: matches rules → creates alerts → delivers to channels.
func (uc *DispatchUseCase) Execute(ctx context.Context, event *NotificationEvent) error {
	// 1. Load matching rules
	rules, err := uc.ruleRepo.FindMatchingRules(ctx, event.Type, event.ProductID)
	if err != nil {
		return fmt.Errorf("loading notification rules: %w", err)
	}
	if len(rules) == 0 {
		slog.DebugContext(ctx, "no notification rules matched", "event", event.Type)
		return nil
	}

	// 2. Load recipients
	var recipients []Recipient
	if event.ProductID != nil {
		members, err := uc.identityClient.GetUsersForProduct(ctx, *event.ProductID)
		if err != nil {
			slog.WarnContext(ctx, "failed to get product members", "product_id", *event.ProductID, "error", err)
		} else {
			recipients = members
		}
	}
	// For system-level events, use system rules which send to subscribed users
	if len(recipients) == 0 {
		recipients = extractSystemRecipients(rules)
	}

	// 3. Dispatch to each recipient
	for _, recipient := range recipients {
		recipient := recipient

		// 3a. Always create in-app alert
		alert := &Alert{
			ID:          uuid.New(),
			UserID:      recipient.UserID,
			EventType:   string(event.Type),
			Title:       event.Title,
			Description: event.Description,
			URL:         event.URL,
			IsRead:      false,
			CreatedAt:   time.Now().UTC(),
		}
		if err := uc.alertRepo.Save(ctx, alert); err != nil {
			slog.WarnContext(ctx, "failed to save in-app alert", "user_id", recipient.UserID, "error", err)
		}

		// 3b. Deliver to each configured channel (async)
		channels := collectChannels(rules, event.Type)
		for _, channel := range channels {
			channel := channel
			payload, err := uc.tmplRenderer.Render(event.Type, channel, &TemplateData{
				Event:     event,
				Recipient: recipient,
			})
			if err != nil {
				slog.WarnContext(ctx, "template render failed", "channel", channel, "error", err)
				continue
			}

			record := &DeliveryRecord{
				ID:        uuid.New(),
				EventType: string(event.Type),
				Channel:   string(channel),
				Recipient: recipient.Email,
				Status:    "pending",
				Payload:   payload,
				CreatedAt: time.Now().UTC(),
			}
			if err := uc.deliveryRepo.Save(ctx, record); err != nil {
				slog.WarnContext(ctx, "failed to save delivery record", "error", err)
			}

			go uc.deliverWithRetry(record, channel, payload)
		}
	}
	return nil
}

// deliverWithRetry delivers to the channel with exponential backoff: 30s, 60s, 120s.
func (uc *DispatchUseCase) deliverWithRetry(record *DeliveryRecord, channel rule.Channel, payload map[string]interface{}) {
	backoffs := []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second}
	ctx := context.Background()

	sender, ok := uc.channelSenders[channel]
	if !ok {
		slog.Error("no sender registered for channel", "channel", channel)
		return
	}

	for attempt := 0; attempt <= len(backoffs); attempt++ {
		err := sender.Send(ctx, record.Recipient, payload)
		if err == nil {
			record.Status = "sent"
			record.Attempts = attempt + 1
			_ = uc.deliveryRepo.Update(ctx, record)
			slog.Info("notification delivered", "channel", channel, "recipient", record.Recipient, "attempts", record.Attempts)
			return
		}

		slog.Warn("delivery attempt failed",
			"channel", channel, "attempt", attempt+1, "error", err)

		if attempt < len(backoffs) {
			now := time.Now().UTC()
			record.Status = "retrying"
			record.Attempts = attempt + 1
			record.LastAttemptAt = &now
			record.ErrorMessage = err.Error()
			_ = uc.deliveryRepo.Update(ctx, record)
			time.Sleep(backoffs[attempt])
		}
	}

	// All retries exhausted
	record.Status = "failed"
	_ = uc.deliveryRepo.Update(ctx, record)
	slog.Error("all delivery retries exhausted",
		"channel", channel, "recipient", record.Recipient, "event", record.EventType)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func collectChannels(rules []*rule.NotificationRule, et rule.EventType) []rule.Channel {
	seen := make(map[rule.Channel]bool)
	var result []rule.Channel
	for _, r := range rules {
		for _, ch := range r.ChannelsForEvent(et) {
			if !seen[ch] {
				seen[ch] = true
				result = append(result, ch)
			}
		}
	}
	return result
}

func extractSystemRecipients(_ []*rule.NotificationRule) []Recipient {
	// System rules (UserID=nil) — in minimal impl, recipients come from rule config
	// Full impl would query all users subscribed via system rules
	return nil
}
