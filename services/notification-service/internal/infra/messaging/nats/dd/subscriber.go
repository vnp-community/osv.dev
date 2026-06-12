// Package event subscribes to NATS events and dispatches notifications.
package event

import (
	"context"

	natsutil "github.com/osv/shared/pkg/nats"
	"github.com/osv/notification-service/internal/domain/rule"
	dispatch "github.com/osv/notification-service/internal/usecase/dispatch_alert"
)

// RegisterHandlers registers NATS subscriptions for all notification-relevant events.
func RegisterHandlers(sub *natsutil.Subscriber, dispatchUC *dispatch.DispatchUseCase) error {
	type subjectEvent struct {
		subject   string
		eventType rule.EventType
	}

	mappings := []subjectEvent{
		{"defectdojo.scan.import.completed", rule.EventScanAdded},
		{"defectdojo.finding.batch_created", rule.EventFindingAdded},
		{"defectdojo.finding.status_changed", rule.EventFindingStatusChanged},
		{"defectdojo.sla.breached", rule.EventSLABreach},
		{"defectdojo.sla.expiring_soon", rule.EventSLAExpiringSoon},
		{"defectdojo.engagement.created", rule.EventEngagementAdded},
		{"defectdojo.engagement.closed", rule.EventEngagementClosed},
		{"defectdojo.jira.issue.created", rule.EventJIRAUpdate},
		{"defectdojo.jira.issue.updated", rule.EventJIRAUpdate},
	}

	for _, m := range mappings {
		et := m.eventType
		subj := m.subject
		if err := sub.Subscribe(context.Background(), subj, func(ctx context.Context, event *natsutil.CloudEvent) error {
			var meta map[string]interface{}
			_ = event.UnmarshalData(&meta)

			productID := extractString(meta, "product_id")
			notifEvent := &dispatch.NotificationEvent{
				Type:        et,
				ProductID:   nilableString(productID),
				Title:       buildTitle(et, meta),
				Description: buildDescription(et, meta),
				Metadata:    meta,
			}
			return dispatchUC.Execute(ctx, notifEvent)
		}); err != nil {
			return err
		}
	}
	return nil
}

func extractString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func nilableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func buildTitle(et rule.EventType, meta map[string]interface{}) string {
	switch et {
	case rule.EventSLABreach:
		return "SLA Breach Detected"
	case rule.EventSLAExpiringSoon:
		days := extractString(meta, "days_remaining")
		return "SLA Expiring in " + days + " days"
	case rule.EventFindingStatusChanged:
		return "Finding Status Changed"
	case rule.EventScanAdded:
		return "Scan Import Completed"
	case rule.EventFindingAdded:
		count := extractString(meta, "count")
		return count + " New Findings Imported"
	default:
		return string(et)
	}
}

func buildDescription(et rule.EventType, meta map[string]interface{}) string {
	severity := extractString(meta, "severity")
	findingID := extractString(meta, "finding_id")
	switch et {
	case rule.EventSLABreach:
		days := extractString(meta, "days_overdue")
		return "Finding " + findingID + " (Severity: " + severity + ") is " + days + " day(s) overdue"
	case rule.EventSLAExpiringSoon:
		days := extractString(meta, "days_remaining")
		return "Finding " + findingID + " (Severity: " + severity + ") expires in " + days + " day(s)"
	default:
		return ""
	}
}
