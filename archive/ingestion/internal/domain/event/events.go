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

// Package event contains domain events published by the Ingestion Service.
package event

import (
	"time"

	"github.com/google/uuid"
)

// Topic constants for NATS JetStream subjects.
const (
	TopicVulnImported  = "osv.vuln.imported"
	TopicVulnUpdated   = "osv.vuln.updated"
	TopicVulnWithdrawn = "osv.vuln.withdrawn"
)

// DomainEvent is the interface implemented by all domain events.
type DomainEvent interface {
	EventID() string
	EventType() string
	OccurredAt() time.Time
	Topic() string
}

// baseEvent provides common fields for all domain events.
type baseEvent struct {
	id          string
	eventType   string
	topic       string
	occurredAt  time.Time
}

func newBase(eventType, topic string) baseEvent {
	return baseEvent{
		id:         uuid.New().String(),
		eventType:  eventType,
		topic:      topic,
		occurredAt: time.Now().UTC(),
	}
}

func (b baseEvent) EventID() string     { return b.id }
func (b baseEvent) EventType() string   { return b.eventType }
func (b baseEvent) OccurredAt() time.Time { return b.occurredAt }
func (b baseEvent) Topic() string       { return b.topic }

// VulnImported is emitted when a new vulnerability is ingested for the first time.
type VulnImported struct {
	baseEvent
	VulnID        string   `json:"vuln_id"`
	Source        string   `json:"source"`
	Ecosystems    []string `json:"ecosystems"`
	IsNew         bool     `json:"is_new"`
	ContentHash   string   `json:"content_hash"`
	SchemaVersion string   `json:"schema_version"`
}

// NewVulnImported creates a new VulnImported event.
func NewVulnImported(vulnID, source, contentHash, schemaVersion string, ecosystems []string, isNew bool) *VulnImported {
	e := &VulnImported{
		baseEvent:     newBase("VulnImported", TopicVulnImported),
		VulnID:        vulnID,
		Source:        source,
		Ecosystems:    ecosystems,
		IsNew:         isNew,
		ContentHash:   contentHash,
		SchemaVersion: schemaVersion,
	}
	return e
}

// VulnUpdated is emitted when an existing vulnerability record changes meaningfully.
type VulnUpdated struct {
	baseEvent
	VulnID      string   `json:"vuln_id"`
	Source      string   `json:"source"`
	ContentHash string   `json:"content_hash"`
	Ecosystems  []string `json:"ecosystems"`
}

// NewVulnUpdated creates a new VulnUpdated event.
func NewVulnUpdated(vulnID, source, contentHash string, ecosystems []string) *VulnUpdated {
	return &VulnUpdated{
		baseEvent:   newBase("VulnUpdated", TopicVulnUpdated),
		VulnID:      vulnID,
		Source:      source,
		ContentHash: contentHash,
		Ecosystems:  ecosystems,
	}
}

// VulnWithdrawn is emitted when a vulnerability is withdrawn from the database.
type VulnWithdrawn struct {
	baseEvent
	VulnID string `json:"vuln_id"`
	Source string `json:"source"`
	Reason string `json:"reason"`
}

// NewVulnWithdrawn creates a new VulnWithdrawn event.
func NewVulnWithdrawn(vulnID, source, reason string) *VulnWithdrawn {
	return &VulnWithdrawn{
		baseEvent: newBase("VulnWithdrawn", TopicVulnWithdrawn),
		VulnID:    vulnID,
		Source:    source,
		Reason:    reason,
	}
}
