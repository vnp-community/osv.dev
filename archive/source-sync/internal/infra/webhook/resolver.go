// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

// ─── SourceResolver Implementation ───────────────────────────────────────────

// ConfigSourceResolver maps repository URLs to source names using a static config.
// Source URLs are loaded from sources.yaml at startup.
type ConfigSourceResolver struct {
	urlToSource map[string]string // normalized URL → source name
}

// SourceURLConfig holds URL configurations for a single source.
type SourceURLConfig struct {
	Name    string
	URLs    []string // All known URLs for this source (HTTPS, SSH, etc.)
}

// NewConfigSourceResolver builds a resolver from a list of source URL configs.
func NewConfigSourceResolver(sources []SourceURLConfig) *ConfigSourceResolver {
	m := make(map[string]string, len(sources)*2)
	for _, s := range sources {
		for _, u := range s.URLs {
			key := normalizeURL(u)
			m[key] = s.Name
		}
	}
	return &ConfigSourceResolver{urlToSource: m}
}

// ResolveByRepoURL returns the source name for a given repository URL.
// Returns ("", false) if no matching source is configured.
func (r *ConfigSourceResolver) ResolveByRepoURL(repoURL string) (string, bool) {
	key := normalizeURL(repoURL)
	name, ok := r.urlToSource[key]
	return name, ok
}

// normalizeURL strips trailing slashes, .git suffix, and lowercases the URL.
func normalizeURL(u string) string {
	u = strings.ToLower(strings.TrimSpace(u))
	u = strings.TrimSuffix(u, ".git")
	u = strings.TrimRight(u, "/")
	return u
}

// ─── SyncTrigger Implementation via NATS ─────────────────────────────────────

const syncRequestSubject = "source.sync.requested"

// SyncRequest is the event payload published when a sync is triggered.
type SyncRequest struct {
	SourceName  string    `json:"source_name"`
	Reason      string    `json:"reason"`
	TriggeredAt time.Time `json:"triggered_at"`
}

// NATSSyncTrigger triggers source syncs by publishing events to NATS.
type NATSSyncTrigger struct {
	nc      *natsgo.Conn
	log     zerolog.Logger
	subject string
}

// NewNATSSyncTrigger creates a SyncTrigger backed by NATS.
func NewNATSSyncTrigger(nc *natsgo.Conn, log zerolog.Logger) *NATSSyncTrigger {
	return &NATSSyncTrigger{
		nc:      nc,
		log:     log,
		subject: syncRequestSubject,
	}
}

// TriggerSync publishes a sync request event to NATS.
func (t *NATSSyncTrigger) TriggerSync(sourceName string, reason string) error {
	payload, err := json.Marshal(SyncRequest{
		SourceName:  sourceName,
		Reason:      reason,
		TriggeredAt: time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("marshal sync request: %w", err)
	}

	if err := t.nc.Publish(t.subject, payload); err != nil {
		return fmt.Errorf("nats publish sync request: %w", err)
	}

	t.log.Debug().
		Str("source", sourceName).
		Str("reason", reason).
		Str("subject", t.subject).
		Msg("sync request published")

	return nil
}

// SyncRequestSubscriber listens for sync request events and calls the handler.
// Register this subscriber in main.go to process webhook-triggered syncs.
type SyncRequestSubscriber struct {
	nc       *natsgo.Conn
	onSync   func(ctx context.Context, sourceName, reason string) error
	log      zerolog.Logger
}

// NewSyncRequestSubscriber creates a subscriber for sync request events.
func NewSyncRequestSubscriber(nc *natsgo.Conn, onSync func(ctx context.Context, sourceName, reason string) error, log zerolog.Logger) *SyncRequestSubscriber {
	return &SyncRequestSubscriber{nc: nc, onSync: onSync, log: log}
}

// Subscribe starts listening for sync request events.
func (s *SyncRequestSubscriber) Subscribe(ctx context.Context) error {
	_, err := s.nc.Subscribe(syncRequestSubject, func(msg *natsgo.Msg) {
		var req SyncRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			s.log.Error().Err(err).Msg("sync subscriber: failed to parse message")
			return
		}
		s.log.Info().
			Str("source", req.SourceName).
			Str("reason", req.Reason).
			Msg("sync subscriber: received sync request")

		if err := s.onSync(ctx, req.SourceName, req.Reason); err != nil {
			s.log.Error().Err(err).Str("source", req.SourceName).Msg("sync subscriber: sync failed")
		}
	})
	return err
}
