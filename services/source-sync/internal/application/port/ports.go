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

// Package port defines port interfaces for the Source Sync Service.
package port

import (
	"context"

	"github.com/osv/source-sync/internal/domain/aggregate/source_repository"
	"github.com/osv/source-sync/internal/domain/valueobject"
)

// ChangeSet is an alias for valueobject.ChangeSet for backward compatibility.
type ChangeSet = valueobject.ChangeSet

// FileChange is an alias for valueobject.FileChange for backward compatibility.
type FileChange = valueobject.FileChange

// SourceFetcher detects changes in an external source and returns a ChangeSet.
type SourceFetcher interface {
	// DetectChanges compares the current state of the source against the last
	// synced state and returns the set of changed/deleted files.
	DetectChanges(ctx context.Context, source *source_repository.SourceRepository, forceResync bool) (*ChangeSet, error)
}

// EventPublisher publishes SourceChangeDetected events to NATS.
type EventPublisher interface {
	// PublishChange publishes a SourceChangeDetected event.
	PublishChange(ctx context.Context, event SourceChangeDetected) error
}

// SourceChangeDetected is the event emitted when a source file changes.
type SourceChangeDetected struct {
	EventID     string
	SourceName  string
	FilePath    string
	ContentHash string
	IsDeleted   bool
	RawContent  []byte // inline for small files (<100KB)
	GCSPath     string // for large files stored in GCS
}
