// Package nats provides NATS JetStream publisher for agent lifecycle events.
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/osv/agent-service/internal/domain/entity"
	"github.com/rs/zerolog"
)

// AgentPublisher publishes agent events to NATS JetStream.
// Subjects: agent.agent.registered, agent.report.submitted
type AgentPublisher struct {
	js  nats.JetStreamContext
	log zerolog.Logger
}

func NewAgentPublisher(nc *nats.Conn, log zerolog.Logger) (*AgentPublisher, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("jetstream context: %w", err)
	}
	js.AddStream(&nats.StreamConfig{ //nolint:errcheck
		Name:     "AGENT",
		Subjects: []string{"agent.>"},
		Storage:  nats.FileStorage,
	})
	return &AgentPublisher{js: js, log: log}, nil
}

type AgentRegisteredEvent struct {
	AgentID   uuid.UUID `json:"agent_id"`
	Hostname  string    `json:"hostname"`
	IPAddress string    `json:"ip_address"`
	OccurredAt time.Time `json:"occurred_at"`
}

type ReportSubmittedEvent struct {
	AgentID    uuid.UUID        `json:"agent_id"`
	ReportID   uuid.UUID        `json:"report_id"`
	Packages   []entity.Package `json:"unenriched_packages"` // packages without CVEs
	OccurredAt time.Time        `json:"occurred_at"`
}

func (p *AgentPublisher) PublishAgentRegistered(ctx context.Context, agent *entity.Agent) error {
	return p.publish("agent.agent.registered", AgentRegisteredEvent{
		AgentID: agent.ID, Hostname: agent.Hostname, IPAddress: agent.IPAddress,
		OccurredAt: time.Now().UTC(),
	})
}

func (p *AgentPublisher) PublishReportSubmitted(ctx context.Context, agentID, reportID uuid.UUID, unenrichedPkgs []entity.Package) error {
	return p.publish("agent.report.submitted", ReportSubmittedEvent{
		AgentID: agentID, ReportID: reportID, Packages: unenrichedPkgs,
		OccurredAt: time.Now().UTC(),
	})
}

func (p *AgentPublisher) publish(subject string, v any) error {
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
