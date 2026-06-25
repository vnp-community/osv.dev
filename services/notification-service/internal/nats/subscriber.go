package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nats-io/nats.go"
	"github.com/osv/notification-service/internal/broker"
	"github.com/osv/notification-service/internal/usecase"
	"github.com/rs/zerolog"
)

// Subscriber listens to NATS JetStream events and dispatches alerts.
type Subscriber struct {
	nc         *nats.Conn
	dispatcher *usecase.AlertDispatcher
	logger     zerolog.Logger
	broker     *broker.EventBroker
}

func NewSubscriber(nc *nats.Conn, dispatcher *usecase.AlertDispatcher, log zerolog.Logger, b *broker.EventBroker) *Subscriber {
	return &Subscriber{nc: nc, dispatcher: dispatcher, logger: log, broker: b}
}

// Start subscribes to kev.> and cve.> subjects.
func (s *Subscriber) Start(ctx context.Context) error {
	if os.Getenv("NATS_ENABLED") != "true" {
		s.logger.Info().Msg("NATS subscriber disabled")
		return nil
	}

	js, _ := s.nc.JetStream()

	// Subscribe to KEV events
	js.Subscribe("kev.new", func(msg *nats.Msg) {
		var payload map[string]interface{}
		json.Unmarshal(msg.Data, &payload)

		cveID := getString(payload, "cve_id")
		vendor := getString(payload, "vendor")
		product := getString(payload, "product")

		ev := usecase.CVEEvent{
			CVEID: cveID,
			IsKEV: true,
		}
		s.dispatcher.Dispatch(ctx, ev) //nolint:errcheck

		// push to all SSE clients
		s.broker.PushAll(broker.NotificationEvent{
			Type:       "kev.new",
			Title:      fmt.Sprintf("New KEV: %s %s (%s)", vendor, product, cveID),
			Message:    "Added to CISA KEV catalog",
			Severity:   "Critical",
			EntityType: "cve",
			EntityID:   cveID,
		})

		msg.Ack()
	})

	// Subscribe to SLA breach events
	js.Subscribe("defectdojo.sla.breached", func(msg *nats.Msg) {
		var event struct {
			FindingID      string `json:"finding_id"`
			AssignedUserID string `json:"assigned_user_id"`
			FindingTitle   string `json:"finding_title"`
			Severity       string `json:"severity"`
		}
		json.Unmarshal(msg.Data, &event)

		if event.AssignedUserID != "" {
			s.broker.Push(event.AssignedUserID, broker.NotificationEvent{
				Type:       "finding.sla.breached",
				Title:      "SLA Breach: " + event.FindingTitle,
				Message:    "This finding has exceeded its SLA deadline",
				Severity:   "High",
				EntityType: "finding",
				EntityID:   event.FindingID,
			})
		}
		msg.Ack()
	})

	s.logger.Info().Msg("NATS subscriber started")
	return nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
