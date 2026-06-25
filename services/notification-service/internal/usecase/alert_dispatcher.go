package usecase

import (
	"context"

	"github.com/osv/notification-service/internal/domain/repository"
	entity "github.com/osv/notification-service/internal/domain/webhook"
)

// CVEEvent represents an incoming CVE notification event.
type CVEEvent struct {
	CVEID       string
	Severity    string // "CRITICAL"|"HIGH"|"MEDIUM"|"LOW"
	EPSS        float64
	Vendors     []string
	Products    []string
	IsKEV       bool
	IsExploit   bool
	Description string
}

// AlertDispatcher matches CVE events to subscribers + webhooks and dispatches alerts.
type AlertDispatcher struct {
	webhookRepo   repository.WebhookRepository
	subscriptRepo repository.SubscriptionRepository
	deliverer     *WebhookDeliverer
}

func NewAlertDispatcher(
	webhookRepo repository.WebhookRepository,
	subscriptRepo repository.SubscriptionRepository,
	deliverer *WebhookDeliverer,
) *AlertDispatcher {
	return &AlertDispatcher{
		webhookRepo:   webhookRepo,
		subscriptRepo: subscriptRepo,
		deliverer:     deliverer,
	}
}

// Dispatch analyzes a CVE event and fires webhooks for matching subscribers.
func (d *AlertDispatcher) Dispatch(ctx context.Context, ev CVEEvent) error {
	events := d.computeEventTypes(ev)
	for _, eventType := range events {
		webhooks, err := d.webhookRepo.FindByEvent(ctx, eventType)
		if err != nil {
			continue
		}
		for _, wh := range webhooks {
			d.deliverer.Deliver(ctx, DeliveryInput{ //nolint:errcheck
				WebhookID: wh.ID(),
				EventType: eventType,
				CVEID:     ev.CVEID,
				Payload:   d.buildEventPayload(ev),
			})
		}
	}

	// Subscription-based alerts (vendor/product match)
	if err := d.dispatchSubscriptionAlerts(ctx, ev); err != nil {
		return err
	}
	return nil
}

// computeEventTypes determines which event types apply to this CVE event.
func (d *AlertDispatcher) computeEventTypes(ev CVEEvent) []entity.EventType {
	var events []entity.EventType

	if ev.IsKEV {
		events = append(events, entity.EventNewKEV)
	}
	switch ev.Severity {
	case "CRITICAL":
		events = append(events, entity.EventNewCritical)
	case "HIGH":
		events = append(events, entity.EventNewHigh)
	}
	if ev.EPSS >= 0.9 {
		events = append(events, entity.EventHighEPSS)
	}
	if len(ev.Vendors) > 0 {
		events = append(events, entity.EventVendorCVE)
	}
	if len(ev.Products) > 0 {
		events = append(events, entity.EventProductCVE)
	}
	return events
}

func (d *AlertDispatcher) buildEventPayload(ev CVEEvent) map[string]interface{} {
	return map[string]interface{}{
		"cve_id":      ev.CVEID,
		"severity":    ev.Severity,
		"epss":        ev.EPSS,
		"vendors":     ev.Vendors,
		"products":    ev.Products,
		"is_kev":      ev.IsKEV,
		"is_exploit":  ev.IsExploit,
		"description": ev.Description,
	}
}

func (d *AlertDispatcher) dispatchSubscriptionAlerts(ctx context.Context, ev CVEEvent) error {
	for _, vendor := range ev.Vendors {
		subs, _ := d.subscriptRepo.FindByVendor(ctx, vendor)
		for _, sub := range subs {
			if !sub.IsActive {
				continue
			}
			if !meetsSeverityFilter(ev.Severity, sub.MinSeverity) {
				continue
			}
			if sub.MinEPSS != nil && ev.EPSS < *sub.MinEPSS {
				continue
			}

			webhooks, _ := d.webhookRepo.FindByOwner(ctx, sub.OwnerID)
			for _, wh := range webhooks {
				d.deliverer.Deliver(ctx, DeliveryInput{ //nolint:errcheck
					WebhookID: wh.ID(),
					EventType: entity.EventVendorCVE,
					CVEID:     ev.CVEID,
					Payload:   d.buildEventPayload(ev),
				})
			}
		}
	}
	return nil
}

// meetsSeverityFilter returns true if actual severity >= minSeverity.
func meetsSeverityFilter(actual, minSeverity string) bool {
	order := map[string]int{"LOW": 0, "MEDIUM": 1, "HIGH": 2, "CRITICAL": 3}
	return order[actual] >= order[minSeverity]
}
