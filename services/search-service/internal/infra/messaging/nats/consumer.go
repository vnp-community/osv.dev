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

// Package nats provides NATS JetStream adapters for the Search service.
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	natsgo "github.com/nats-io/nats.go"

	"github.com/osv/search-service/internal/application/command/index_vulnerability"
	"github.com/osv/search-service/internal/domain/repository"
)

const (
	subjectVulnWildcard   = "osv.vuln.>"
	durableSearchConsumer = "search-vuln-events"
)

// VulnEventPayload is the minimal payload from VulnImported/Updated/Withdrawn events.
type VulnEventPayload struct {
	EventType   string          `json:"event_type"` // "imported" | "updated" | "withdrawn"
	VulnID      string          `json:"vuln_id"`
	ContentHash string          `json:"content_hash"`
	VulnJSON    json.RawMessage `json:"vuln,omitempty"`
}

// VulnEventConsumer subscribes to osv.vuln.> and keeps the search index in sync.
type VulnEventConsumer struct {
	js          natsgo.JetStreamContext
	indexRepo   repository.SearchIndexRepo
	indexCmd    *index_vulnerability.Handler
}

// NewVulnEventConsumer creates a new consumer.
func NewVulnEventConsumer(js natsgo.JetStreamContext, repo repository.SearchIndexRepo, idx *index_vulnerability.Handler) *VulnEventConsumer {
	return &VulnEventConsumer{js: js, indexRepo: repo, indexCmd: idx}
}

// Start begins consuming messages (blocking — run in goroutine).
// SLO: index within 30s of VulnImported/Updated event.
func (c *VulnEventConsumer) Start(ctx context.Context) error {
	sub, err := c.js.PullSubscribe(
		subjectVulnWildcard,
		durableSearchConsumer,
		natsgo.MaxDeliver(5),
		natsgo.AckWait(60*time.Second),
	)
	if err != nil {
		return fmt.Errorf("search vuln consumer subscribe: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msgs, err := sub.Fetch(20, natsgo.MaxWait(5*time.Second))
		if err != nil {
			if err == natsgo.ErrTimeout {
				continue
			}
			return fmt.Errorf("search consumer fetch: %w", err)
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

func (c *VulnEventConsumer) processMessage(ctx context.Context, msg *natsgo.Msg) error {
	var payload VulnEventPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("search consumer: unmarshal: %w", err)
	}

	switch payload.EventType {
	case "withdrawn":
		return c.indexRepo.MarkWithdrawn(ctx, payload.VulnID)
	case "imported", "updated":
		return c.indexCmd.Handle(ctx, index_vulnerability.Command{
			VulnID:   payload.VulnID,
			VulnJSON: payload.VulnJSON,
		})
	}
	return nil
}
