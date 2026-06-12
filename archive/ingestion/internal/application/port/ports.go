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

// Package port defines the application port interfaces for the Ingestion Service.
package port

import (
	"context"

	"github.com/osv/ingestion/internal/domain/event"
)

// EventPublisher publishes domain events to the messaging system (NATS JetStream).
type EventPublisher interface {
	// Publish sends a domain event to the appropriate topic.
	Publish(ctx context.Context, e event.DomainEvent) error
}

// BlobStore stores and retrieves full vulnerability JSON blobs.
type BlobStore interface {
	// Upload stores the full JSON blob for the given vuln ID.
	// Returns the GCS path (e.g., "gs://osv-vulnz/CVE-2024-001.json").
	Upload(ctx context.Context, vulnID string, content []byte) (string, error)

	// Download retrieves the full JSON blob for the given vuln ID.
	Download(ctx context.Context, vulnID string) ([]byte, error)
}

// IdempotencyStore provides content-hash-based deduplication.
type IdempotencyStore interface {
	// IsProcessed returns true if the given content hash has already been processed.
	IsProcessed(ctx context.Context, contentHash string) (bool, error)

	// MarkProcessed marks the content hash as processed with the given TTL.
	MarkProcessed(ctx context.Context, contentHash string) error
}
