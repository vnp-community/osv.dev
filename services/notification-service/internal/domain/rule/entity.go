// Package rule defines the NotificationRule entity — which channels receive which events.
package rule

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// EventType identifies the kind of event that triggered a notification.
type EventType string

const (
	EventScanAdded                EventType = "scan_added"
	EventTestAdded                EventType = "test_added"
	EventFindingAdded             EventType = "finding_added"
	EventFindingStatusChanged     EventType = "finding_status_changed"
	EventJIRAUpdate               EventType = "jira_update"
	EventEngagementAdded          EventType = "engagement_added"
	EventEngagementClosed         EventType = "engagement_closed"
	EventRiskAcceptanceExpiration EventType = "risk_acceptance_expiration"
	EventSLABreach                EventType = "sla_breach"
	EventSLAExpiringSoon          EventType = "sla_expiring_soon"
	EventProductAdded             EventType = "product_added"
	EventUserMentioned            EventType = "user_mentioned"
	EventClosedFindingRemoved     EventType = "closed_finding_removed"
	EventReviewRequested          EventType = "review_requested"
)

// Channel identifies the delivery channel for a notification.
type Channel string

const (
	ChannelEmail   Channel = "email"
	ChannelSlack   Channel = "slack"
	ChannelTeams   Channel = "msteams"
	ChannelWebhook Channel = "webhook"
	ChannelInApp   Channel = "inapp"
)

// NotificationRule defines which channels receive which event types.
type NotificationRule struct {
	ID        uuid.UUID
	UserID    *uuid.UUID // nil = system-wide rule
	ProductID *uuid.UUID // nil = applies to all products

	ScanAdded                []Channel
	TestAdded                []Channel
	FindingAdded             []Channel
	FindingStatusChanged     []Channel
	JIRAUpdate               []Channel
	EngagementAdded          []Channel
	EngagementClosed         []Channel
	RiskAcceptanceExpiration []Channel
	SLABreach                []Channel
	SLAExpiringSoon          []Channel
	ProductAdded             []Channel
	UserMentioned            []Channel
	ClosedFindingRemoved     []Channel
	ReviewRequested          []Channel

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ChannelsForEvent returns the configured channels for a given event type.
func (r *NotificationRule) ChannelsForEvent(et EventType) []Channel {
	switch et {
	case EventScanAdded:
		return r.ScanAdded
	case EventTestAdded:
		return r.TestAdded
	case EventFindingAdded:
		return r.FindingAdded
	case EventFindingStatusChanged:
		return r.FindingStatusChanged
	case EventJIRAUpdate:
		return r.JIRAUpdate
	case EventEngagementAdded:
		return r.EngagementAdded
	case EventEngagementClosed:
		return r.EngagementClosed
	case EventRiskAcceptanceExpiration:
		return r.RiskAcceptanceExpiration
	case EventSLABreach:
		return r.SLABreach
	case EventSLAExpiringSoon:
		return r.SLAExpiringSoon
	case EventProductAdded:
		return r.ProductAdded
	case EventUserMentioned:
		return r.UserMentioned
	case EventClosedFindingRemoved:
		return r.ClosedFindingRemoved
	case EventReviewRequested:
		return r.ReviewRequested
	default:
		return nil
	}
}

// Repository defines persistence for notification rules.
type Repository interface {
	FindMatchingRules(ctx context.Context, eventType EventType, productID *string) ([]*NotificationRule, error)
	GetSystemRule(ctx context.Context) (*NotificationRule, error)
	Create(ctx context.Context, rule *NotificationRule) error
	Update(ctx context.Context, rule *NotificationRule) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListForUser(ctx context.Context, userID uuid.UUID) ([]*NotificationRule, error)
}
