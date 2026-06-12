// Package nats provides NATS JetStream publisher for scan lifecycle events.
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/osv/scan-service/internal/domain/entity"
	"github.com/rs/zerolog"
)

// ScanPublisher publishes scan events to NATS JetStream subjects.
// Subject format: scan.{event_type}
type ScanPublisher struct {
	js  nats.JetStreamContext
	log zerolog.Logger
}

// NewScanPublisher creates a ScanPublisher backed by JetStream.
func NewScanPublisher(nc *nats.Conn, log zerolog.Logger) (*ScanPublisher, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("jetstream context: %w", err)
	}
	// Ensure stream exists (idempotent)
	js.AddStream(&nats.StreamConfig{ //nolint:errcheck
		Name:     "SCAN",
		Subjects: []string{"scan.>"},
		Storage:  nats.FileStorage,
		Retention: nats.WorkQueuePolicy,
	})
	return &ScanPublisher{js: js, log: log}, nil
}

// ScanCreatedEvent is published when a new scan is created.
type ScanCreatedEvent struct {
	ScanID   uuid.UUID        `json:"scan_id"`
	UserID   uuid.UUID        `json:"user_id"`
	Targets  []string         `json:"targets"`
	ScanType entity.ScanType  `json:"scan_type"`
	Priority int              `json:"priority"`
	OccurredAt time.Time      `json:"occurred_at"`
}

// ScanStatusEvent is published for scan state transitions.
type ScanStatusEvent struct {
	ScanID      uuid.UUID `json:"scan_id"`
	Status      string    `json:"status"`
	FindingCount int      `json:"finding_count,omitempty"`
	ErrorMsg    string    `json:"error_msg,omitempty"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// ProgressEvent is published for scan progress updates.
type ProgressEvent struct {
	ScanID     uuid.UUID `json:"scan_id"`
	Progress   int       `json:"progress"`
	OccurredAt time.Time `json:"occurred_at"`
}

// FindingEvent is published when a new finding is discovered.
type FindingEvent struct {
	ScanID    uuid.UUID `json:"scan_id"`
	IPAddress string    `json:"ip_address"`
	Hostname  string    `json:"hostname"`
	CVEIDs    []string  `json:"cve_ids"`
	Severity  string    `json:"severity"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (p *ScanPublisher) PublishScanCreated(ctx context.Context, scan *entity.Scan) error {
	return p.publish("scan.scan.created", ScanCreatedEvent{
		ScanID: scan.ID, UserID: scan.UserID,
		Targets: scan.Targets, ScanType: scan.ScanType, Priority: scan.Priority,
		OccurredAt: time.Now().UTC(),
	})
}

func (p *ScanPublisher) PublishScanStarted(ctx context.Context, scanID uuid.UUID) error {
	return p.publish("scan.scan.started", ScanStatusEvent{
		ScanID: scanID, Status: "running", OccurredAt: time.Now().UTC(),
	})
}

func (p *ScanPublisher) PublishScanCompleted(ctx context.Context, scanID uuid.UUID, findingCount int) error {
	return p.publish("scan.scan.completed", ScanStatusEvent{
		ScanID: scanID, Status: "completed", FindingCount: findingCount, OccurredAt: time.Now().UTC(),
	})
}

func (p *ScanPublisher) PublishScanFailed(ctx context.Context, scanID uuid.UUID, errMsg string) error {
	return p.publish("scan.scan.failed", ScanStatusEvent{
		ScanID: scanID, Status: "failed", ErrorMsg: errMsg, OccurredAt: time.Now().UTC(),
	})
}

func (p *ScanPublisher) PublishFindingDiscovered(ctx context.Context, f *entity.Finding) error {
	return p.publish("scan.finding.discovered", FindingEvent{
		ScanID: f.ScanID, IPAddress: f.IPAddress, Hostname: f.Hostname,
		CVEIDs: f.CVEIDs, Severity: string(f.Severity), OccurredAt: time.Now().UTC(),
	})
}

func (p *ScanPublisher) PublishProgress(ctx context.Context, scanID uuid.UUID, progress int) error {
	return p.publish("scan.scan.progress", ProgressEvent{
		ScanID: scanID, Progress: progress, OccurredAt: time.Now().UTC(),
	})
}

func (p *ScanPublisher) publish(subject string, v any) error {
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
