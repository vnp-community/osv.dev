// Package dispatch implements the DispatchNotification use case.
// Routes events to the correct delivery channel (Email, Slack, Teams, Webhook, InApp).
package dispatch

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/defectdojo/notification/internal/domain/rule"
	"github.com/defectdojo/notification/internal/domain/alert"
	"github.com/defectdojo/notification/internal/domain/delivery"
)

// NotificationEvent is the normalized input to the dispatch use case.
type NotificationEvent struct {
	Type        rule.EventType
	ProductID   *string
	UserIDs     []string // specific users to notify (nil = notify product members)
	Title       string
	Description string
	URL         string
	Metadata    map[string]interface{}
}

// Sender interfaces — implemented by infrastructure packages.
type EmailSender interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
}
type SlackSender interface {
	Send(ctx context.Context, channelID, title, description, viewURL string) error
}
type TeamsSender interface {
	Send(ctx context.Context, webhookURL, title, description string) error
}
type WebhookSender interface {
	Send(ctx context.Context, url, secret string, payload interface{}) error
}

// DispatchUseCase routes notification events to configured channels.
type DispatchUseCase struct {
	ruleRepo     rule.Repository
	alertRepo    alert.Repository
	deliveryRepo delivery.Repository
	email        EmailSender
	slack        SlackSender
	teams        TeamsSender
	webhook      WebhookSender
}

func New(
	ruleRepo rule.Repository,
	alertRepo alert.Repository,
	deliveryRepo delivery.Repository,
	email EmailSender,
	slack SlackSender,
	teams TeamsSender,
	webhook WebhookSender,
) *DispatchUseCase {
	return &DispatchUseCase{
		ruleRepo:     ruleRepo,
		alertRepo:    alertRepo,
		deliveryRepo: deliveryRepo,
		email:        email,
		slack:        slack,
		teams:        teams,
		webhook:      webhook,
	}
}

// Execute finds matching rules and delivers notifications to all configured channels.
func (uc *DispatchUseCase) Execute(ctx context.Context, event *NotificationEvent) error {
	rules, err := uc.ruleRepo.FindMatchingRules(ctx, event.Type, event.ProductID)
	if err != nil || len(rules) == 0 {
		return err
	}

	for _, r := range rules {
		channels := r.ChannelsForEvent(event.Type)
		for _, ch := range channels {
			rec := &delivery.Record{
				ID:        uuid.New(),
				EventType: string(event.Type),
				Channel:   string(ch),
				Status:    delivery.StatusPending,
				CreatedAt: time.Now().UTC(),
			}

			switch ch {
			case rule.ChannelEmail:
				go uc.deliverEmail(ctx, rec, event)
			case rule.ChannelSlack:
				go uc.deliverSlack(ctx, rec, event)
			case rule.ChannelTeams:
				go uc.deliverTeams(ctx, rec, event)
			case rule.ChannelWebhook:
				go uc.deliverWebhook(ctx, rec, event)
			case rule.ChannelInApp:
				// In-app alerts: synchronous, no network call
				if r.UserID != nil {
					_ = uc.alertRepo.Create(ctx, &alert.Alert{
						ID:          uuid.New(),
						UserID:      *r.UserID,
						EventType:   string(event.Type),
						Title:       event.Title,
						Description: event.Description,
						URL:         event.URL,
						CreatedAt:   time.Now().UTC(),
					})
				}
			}
		}
	}
	return nil
}

func (uc *DispatchUseCase) deliverEmail(ctx context.Context, rec *delivery.Record, event *NotificationEvent) {
	rec.Attempts++
	now := time.Now().UTC()
	rec.LastAttemptAt = &now

	err := uc.email.Send(ctx, rec.Recipient, event.Title,
		"<p>"+event.Description+"</p><p><a href=\""+event.URL+"\">View in DefectDojo</a></p>")
	if err != nil {
		rec.Status = delivery.StatusFailed
		rec.ErrorMessage = err.Error()
	} else {
		rec.Status = delivery.StatusSent
	}
	_ = uc.deliveryRepo.Save(ctx, rec)
}

func (uc *DispatchUseCase) deliverSlack(ctx context.Context, rec *delivery.Record, event *NotificationEvent) {
	rec.Attempts++
	now := time.Now().UTC()
	rec.LastAttemptAt = &now

	err := uc.slack.Send(ctx, rec.Recipient, event.Title, event.Description, event.URL)
	if err != nil {
		rec.Status = delivery.StatusFailed
		rec.ErrorMessage = err.Error()
	} else {
		rec.Status = delivery.StatusSent
	}
	_ = uc.deliveryRepo.Save(ctx, rec)
}

func (uc *DispatchUseCase) deliverTeams(ctx context.Context, rec *delivery.Record, event *NotificationEvent) {
	rec.Attempts++
	now := time.Now().UTC()
	rec.LastAttemptAt = &now

	err := uc.teams.Send(ctx, rec.Recipient, event.Title, event.Description)
	if err != nil {
		rec.Status = delivery.StatusFailed
		rec.ErrorMessage = err.Error()
	} else {
		rec.Status = delivery.StatusSent
	}
	_ = uc.deliveryRepo.Save(ctx, rec)
}

func (uc *DispatchUseCase) deliverWebhook(ctx context.Context, rec *delivery.Record, event *NotificationEvent) {
	rec.Attempts++
	now := time.Now().UTC()
	rec.LastAttemptAt = &now

	err := uc.webhook.Send(ctx, rec.Recipient, "", event)
	if err != nil {
		rec.Status = delivery.StatusFailed
		rec.ErrorMessage = err.Error()
	} else {
		rec.Status = delivery.StatusSent
	}
	_ = uc.deliveryRepo.Save(ctx, rec)
}
