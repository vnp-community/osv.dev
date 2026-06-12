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

// Package consumer contains NATS JetStream consumers for the Ingestion Service.
package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	natspkg "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"

	importcmd "github.com/osv/ingestion-service/internal/application/command/import_vulnerability"
	"github.com/osv/ingestion-service/internal/domain/valueobject"
	ingestingnats "github.com/osv/ingestion-service/internal/infra/messaging/nats"
)

const (
	sourceChangeSubject  = "osv.source.change.>"
	sourceChangeDurable  = "ingestion-source-change"
)

// SourceChangeConsumer subscribes to SourceChangeDetected events and triggers ingestion.
type SourceChangeConsumer struct {
	js            jetstream.JetStream
	importHandler *importcmd.Handler
}

// NewSourceChangeConsumer creates a new consumer for source change events.
func NewSourceChangeConsumer(nc *natspkg.Conn, importHandler *importcmd.Handler) (*SourceChangeConsumer, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("source change consumer: jetstream: %w", err)
	}
	return &SourceChangeConsumer{js: js, importHandler: importHandler}, nil
}

// Start begins consuming SourceChangeDetected events.
func (c *SourceChangeConsumer) Start(ctx context.Context, streamName string) error {
	stream, err := c.js.Stream(ctx, streamName)
	if err != nil {
		return fmt.Errorf("source change consumer: stream %q: %w", streamName, err)
	}

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:        sourceChangeDurable,
		FilterSubjects: []string{sourceChangeSubject},
		MaxDeliver:     5,
		AckWait:        30 * 60 * 1000000000, // 30 minutes in nanoseconds
		AckPolicy:      jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return fmt.Errorf("source change consumer: create consumer: %w", err)
	}

	_, err = cons.Consume(func(msg jetstream.Msg) {
		if err := c.handleMessage(ctx, msg); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("source change consumer: handle message failed")
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
	return err
}

func (c *SourceChangeConsumer) handleMessage(ctx context.Context, msg jetstream.Msg) error {
	var event ingestingnats.SourceChangeDetected
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("unmarshal SourceChangeDetected: %w", err)
	}

	logger := log.Ctx(ctx)
	logger.Info().
		Str("source", event.SourceName).
		Str("file", event.FilePath).
		Bool("deleted", event.IsDeleted).
		Msg("processing source change")

	if event.IsDeleted {
		// TODO: delegate to withdraw handler
		return nil
	}

	if len(event.RawContent) == 0 {
		// Large file — content is in GCS, skip for now (fetcher would download)
		logger.Warn().Str("gcs_path", event.GCSPath).Msg("large file via GCS ref, skipping (not yet implemented)")
		return nil
	}

	cmd := importcmd.Command{
		RawContent:  event.RawContent,
		ContentHash: event.ContentHash,
		Source:      valueobject.SourceRef{Name: event.SourceName, Path: event.FilePath},
		Extension:   ".json",
	}

	_, err := c.importHandler.Handle(ctx, cmd)
	return err
}
