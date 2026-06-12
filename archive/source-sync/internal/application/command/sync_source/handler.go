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

// Package sync_source contains the SyncSource command handler — the core of Source Sync.
package sync_source

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/osv/source-sync/internal/application/port"
	"github.com/osv/source-sync/internal/domain/repository"
	"github.com/osv/source-sync/internal/domain/valueobject"
)

// Command instructs the handler to sync a named source.
type Command struct {
	SourceName  string
	ForceResync bool // if true, ignore lastSyncedHash
}

// Result summarises the outcome of a sync operation.
type Result struct {
	SourceName   string
	Modified     int
	Deleted      int
	NewSyncHash  string
	Duration     time.Duration
}

// Handler executes the SyncSource command.
type Handler struct {
	sourceRepo repository.SourceRepositoryRepo
	fetchers   map[valueobject.SourceType]port.SourceFetcher
	publisher  port.EventPublisher
}

// NewSingleSourceHandler creates a new single-source sync Handler.
// For the aggregate multi-source handler used by main.go, use NewHandler.
func NewSingleSourceHandler(
	sourceRepo repository.SourceRepositoryRepo,
	gitFetcher port.SourceFetcher,
	bucketFetcher port.SourceFetcher,
	restFetcher port.SourceFetcher,
	publisher port.EventPublisher,
) *Handler {
	return &Handler{
		sourceRepo: sourceRepo,
		fetchers: map[valueobject.SourceType]port.SourceFetcher{
			valueobject.SourceTypeGit:          gitFetcher,
			valueobject.SourceTypeBucket:       bucketFetcher,
			valueobject.SourceTypeRESTEndpoint: restFetcher,
		},
		publisher: publisher,
	}
}

// Handle executes the sync for the given source.
func (h *Handler) Handle(ctx context.Context, cmd Command) (*Result, error) {
	logger := log.Ctx(ctx).With().Str("source", cmd.SourceName).Logger()
	start := time.Now()

	// 1. Load SourceRepository from Firestore
	source, err := h.sourceRepo.GetByName(ctx, cmd.SourceName)
	if err != nil {
		return nil, fmt.Errorf("sync %q: load source: %w", cmd.SourceName, err)
	}

	// 2. Select change detector by source type
	fetcher, ok := h.fetchers[source.SourceType()]
	if !ok {
		return nil, fmt.Errorf("sync %q: unsupported source type %q", cmd.SourceName, source.SourceType())
	}

	// 3. Detect changes
	changeSet, err := fetcher.DetectChanges(ctx, source, cmd.ForceResync)
	if err != nil {
		return nil, fmt.Errorf("sync %q: detect changes: %w", cmd.SourceName, err)
	}

	logger.Info().
		Int("modified", len(changeSet.Modified)).
		Int("deleted", len(changeSet.Deleted)).
		Int("total", changeSet.TotalCount).
		Msg("change detection complete")

	// 4. Deletion safety check
	if err := source.CheckDeletionSafety(len(changeSet.Deleted), changeSet.TotalCount); err != nil {
		logger.Error().Err(err).Msg("deletion safety check failed, aborting sync")
		return nil, fmt.Errorf("sync %q: %w", cmd.SourceName, err)
	}

	// 5. Publish SourceChangeDetected for each modified file
	for _, fc := range changeSet.Modified {
		event := port.SourceChangeDetected{
			EventID:     uuid.New().String(),
			SourceName:  cmd.SourceName,
			FilePath:    fc.Path,
			ContentHash: fc.Hash,
			IsDeleted:   false,
			RawContent:  inlineIfSmall(fc.Content),
		}
		if err := h.publisher.PublishChange(ctx, event); err != nil {
			// Non-fatal: log and continue
			logger.Warn().Err(err).Str("file", fc.Path).Msg("publish change event failed (non-fatal)")
		}
	}

	// 6. Publish SourceChangeDetected{IsDeleted=true} for each deleted file
	for _, fc := range changeSet.Deleted {
		event := port.SourceChangeDetected{
			EventID:    uuid.New().String(),
			SourceName: cmd.SourceName,
			FilePath:   fc.Path,
			IsDeleted:  true,
		}
		if err := h.publisher.PublishChange(ctx, event); err != nil {
			logger.Warn().Err(err).Str("file", fc.Path).Msg("publish delete event failed (non-fatal)")
		}
	}

	// 7. Mark synced → save to Firestore
	source.MarkSynced(changeSet.NewSyncHash, time.Now().UTC())
	if err := h.sourceRepo.Save(ctx, source); err != nil {
		return nil, fmt.Errorf("sync %q: save sync state: %w", cmd.SourceName, err)
	}

	return &Result{
		SourceName:  cmd.SourceName,
		Modified:    len(changeSet.Modified),
		Deleted:     len(changeSet.Deleted),
		NewSyncHash: changeSet.NewSyncHash,
		Duration:    time.Since(start),
	}, nil
}

// inlineIfSmall returns content if it is small enough to inline (< 100KB), else nil.
func inlineIfSmall(content []byte) []byte {
	const maxInlineBytes = 100 * 1024 // 100KB
	if len(content) <= maxInlineBytes {
		return content
	}
	return nil
}
