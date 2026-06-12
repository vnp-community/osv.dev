// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package sync_source

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	natsjs "github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
)

// ─── Source Interface ─────────────────────────────────────────────────────────

// ChangedFile represents a file that changed in a source.
type ChangedFile struct {
	Path    string
	Content []byte
	Hash    string
}

// Source is implemented by each external source adapter (OSV GCS, GHSA, etc.).
type Source interface {
	// Name returns a unique identifier for this source.
	Name() string
	// FetchChanges returns files that changed since the given time.
	FetchChanges(ctx context.Context, since time.Time) ([]ChangedFile, error)
}

// ─── Aggregate Sync Handler ───────────────────────────────────────────────────

// SyncStats holds running statistics for the sync handler.
type SyncStats struct {
	LastSyncAt  time.Time
	TotalSynced int64
	Errors      int64
}

// AggregateHandler polls all registered sources and publishes change events.
type AggregateHandler struct {
	sources   []Source
	publisher *NATSPublisher
	log       zerolog.Logger

	lastSyncAt  time.Time
	totalSynced atomic.Int64
	errors      atomic.Int64
}

// NewHandler creates a new AggregateHandler.
func NewHandler(sources []Source, publisher *NATSPublisher, log zerolog.Logger) *AggregateHandler {
	return &AggregateHandler{
		sources:    sources,
		publisher:  publisher,
		log:        log,
		lastSyncAt: time.Now().Add(-24 * time.Hour), // default: last 24h
	}
}

// SyncAll runs FetchChanges for all sources and publishes events.
func (h *AggregateHandler) SyncAll(ctx context.Context) error {
	since := h.lastSyncAt
	h.lastSyncAt = time.Now()

	var lastErr error
	for _, src := range h.sources {
		changes, err := src.FetchChanges(ctx, since)
		if err != nil {
			h.errors.Add(1)
			h.log.Error().Err(err).Str("source", src.Name()).Msg("fetch changes failed")
			lastErr = err
			continue
		}

		for _, fc := range changes {
			event := ChangeEvent{
				SourceName: src.Name(),
				FilePath:   fc.Path,
				Hash:       fc.Hash,
				Content:    fc.Content,
				ChangedAt:  time.Now(),
			}
			if err := h.publisher.Publish(ctx, event); err != nil {
				h.log.Warn().Err(err).Str("file", fc.Path).Msg("publish failed (non-fatal)")
			}
			h.totalSynced.Add(1)
		}

		h.log.Info().
			Str("source", src.Name()).
			Int("changes", len(changes)).
			Msg("source sync complete")
	}
	return lastErr
}

// Stats returns current sync statistics.
func (h *AggregateHandler) Stats() SyncStats {
	return SyncStats{
		LastSyncAt:  h.lastSyncAt,
		TotalSynced: h.totalSynced.Load(),
		Errors:      h.errors.Load(),
	}
}

// ─── ChangeEvent ─────────────────────────────────────────────────────────────

// ChangeEvent is the NATS event published for each changed file.
type ChangeEvent struct {
	SourceName string    `json:"source_name"`
	FilePath   string    `json:"file_path"`
	Hash       string    `json:"hash"`
	Content    []byte    `json:"content,omitempty"` // inline for small files
	ChangedAt  time.Time `json:"changed_at"`
}

// ─── NATS Publisher ───────────────────────────────────────────────────────────

// NATSPublisher publishes ChangeEvents to NATS JetStream.
type NATSPublisher struct {
	js      natsjs.JetStream
	subject string
	log     zerolog.Logger
}

// NewNATSPublisher creates a publisher that sends events to "source.change.detected".
func NewNATSPublisher(js natsjs.JetStream, log zerolog.Logger) *NATSPublisher {
	return &NATSPublisher{
		js:      js,
		subject: "source.change.detected",
		log:     log,
	}
}

// Publish serialises the event and publishes it to NATS JetStream.
func (p *NATSPublisher) Publish(ctx context.Context, event ChangeEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if _, err := p.js.Publish(ctx, p.subject, data); err != nil {
		return fmt.Errorf("nats publish: %w", err)
	}
	return nil
}
