// Package nats provides NATS JetStream publisher for schedule trigger events.
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

// SchedulePublisher publishes trigger events for the schedule service.
type SchedulePublisher struct {
	js  nats.JetStreamContext
	log zerolog.Logger
}

func NewSchedulePublisher(nc *nats.Conn, log zerolog.Logger) (*SchedulePublisher, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("jetstream context: %w", err)
	}
	js.AddStream(&nats.StreamConfig{ //nolint:errcheck
		Name:     "SCHEDULE",
		Subjects: []string{"schedule.>"},
		Storage:  nats.FileStorage,
	})
	return &SchedulePublisher{js: js, log: log}, nil
}

// TriggerFiredEvent instructs scan-service to create a new scan.
type TriggerFiredEvent struct {
	ScheduleID string    `json:"schedule_id"`
	UserID     string    `json:"user_id"`
	Targets    []string  `json:"targets"`
	ScanType   string    `json:"scan_type"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (p *SchedulePublisher) PublishTriggerFired(ctx context.Context, scheduleID, userID string, targets []string, scanType string) error {
	return p.publish("schedule.trigger.fired", TriggerFiredEvent{
		ScheduleID: scheduleID, UserID: userID,
		Targets: targets, ScanType: scanType,
		OccurredAt: time.Now().UTC(),
	})
}

func (p *SchedulePublisher) publish(subject string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = p.js.Publish(subject, data)
	if err != nil {
		p.log.Warn().Err(err).Str("subject", subject).Msg("NATS publish failed")
	}
	return err
}
