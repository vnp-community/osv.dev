// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package nats provides NATS JetStream adapters for the Impact Analysis service.
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	natsgo "github.com/nats-io/nats.go"

	cmdpkg "github.com/osv/impact-analysis/internal/application/command/analyze_vulnerability"
)

const (
	subjectImpactCompleted = "osv.impact.analysis.completed"
	subjectVulnImported    = "osv.vuln.imported"
	durableConsumer        = "impact-analysis-vuln-imported"
)

// EventPublisher publishes ImpactAnalysisCompleted events to NATS JetStream.
type EventPublisher struct {
	js natsgo.JetStreamContext
}

// NewEventPublisher creates a new EventPublisher.
func NewEventPublisher(js natsgo.JetStreamContext) *EventPublisher {
	return &EventPublisher{js: js}
}

// PublishImpactAnalysisCompleted publishes the impact analysis result event.
func (p *EventPublisher) PublishImpactAnalysisCompleted(ctx context.Context, e cmdpkg.ImpactAnalysisCompleted) error {
	if e.EventID == "" {
		e.EventID = uuid.NewString()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}

	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("impact publisher: marshal: %w", err)
	}

	_, err = p.js.Publish(subjectImpactCompleted, data)
	if err != nil {
		return fmt.Errorf("impact publisher: publish %q: %w", subjectImpactCompleted, err)
	}
	return nil
}

// ─────────────────────────────────────────────
// VulnImportedConsumer
// ─────────────────────────────────────────────

// VulnImportedPayload mirrors the VulnImported NATS event from the Ingestion Service.
type VulnImportedPayload struct {
	VulnID      string          `json:"vuln_id"`
	ContentHash string          `json:"content_hash"`
	Affected    json.RawMessage `json:"affected"` // raw OSV affected[] JSON
}

// VulnImportedConsumer subscribes to osv.vuln.imported and triggers impact analysis.
type VulnImportedConsumer struct {
	js      natsgo.JetStreamContext
	handler *cmdpkg.Handler
}

// NewVulnImportedConsumer creates a new consumer.
func NewVulnImportedConsumer(js natsgo.JetStreamContext, h *cmdpkg.Handler) *VulnImportedConsumer {
	return &VulnImportedConsumer{js: js, handler: h}
}

// Start begins consuming messages (blocking — run in goroutine).
func (c *VulnImportedConsumer) Start(ctx context.Context) error {
	sub, err := c.js.PullSubscribe(
		subjectVulnImported,
		durableConsumer,
		natsgo.MaxDeliver(5),
		natsgo.AckWait(30*time.Minute),
	)
	if err != nil {
		return fmt.Errorf("vuln imported consumer subscribe: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msgs, err := sub.Fetch(10, natsgo.MaxWait(5*time.Second))
		if err != nil {
			if err == natsgo.ErrTimeout {
				continue
			}
			return fmt.Errorf("vuln imported consumer fetch: %w", err)
		}

		for _, msg := range msgs {
			if processErr := c.processMessage(ctx, msg); processErr != nil {
				_ = msg.Nak()
				continue
			}
			_ = msg.Ack()
		}
	}
}

func (c *VulnImportedConsumer) processMessage(ctx context.Context, msg *natsgo.Msg) error {
	var payload VulnImportedPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("vuln imported consumer: unmarshal: %w", err)
	}

	cmd := cmdpkg.Command{
		VulnID:      payload.VulnID,
		ContentHash: payload.ContentHash,
		Options: cmdpkg.AnalysisOptions{
			DetectCherryPicks: true,
		},
	}
	// Note: in production, Affected would be deserialized from payload.Affected.
	_, err := c.handler.Handle(ctx, cmd)
	return err
}
