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

// Package nats provides a NATS JetStream event publisher for the Ingestion Service.
package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/osv/ingestion-service/internal/domain/event"
)

// EventPublisher publishes domain events to NATS JetStream.
type EventPublisher struct {
	js jetstream.JetStream
}

// NewEventPublisher creates a new NATS JetStream event publisher.
func NewEventPublisher(nc *nats.Conn) (*EventPublisher, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("nats publisher: jetstream init: %w", err)
	}
	return &EventPublisher{js: js}, nil
}

// Publish serializes and publishes a domain event to its topic.
func (p *EventPublisher) Publish(ctx context.Context, e event.DomainEvent) error {
	payload, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("nats publish: marshal event %q: %w", e.EventType(), err)
	}

	_, err = p.js.Publish(ctx, e.Topic(), payload)
	if err != nil {
		return fmt.Errorf("nats publish: subject %q: %w", e.Topic(), err)
	}
	return nil
}

// SourceChangeDetected is the event consumed from source-sync service.
type SourceChangeDetected struct {
	EventID     string `json:"event_id"`
	SourceName  string `json:"source_name"`
	FilePath    string `json:"file_path"`
	ContentHash string `json:"content_hash"`
	IsDeleted   bool   `json:"is_deleted"`
	RawContent  []byte `json:"raw_content,omitempty"`
	GCSPath     string `json:"gcs_path,omitempty"`
}
