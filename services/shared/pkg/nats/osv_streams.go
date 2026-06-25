// Package nats_streams provides OSV-platform-wide JetStream stream definitions.
// This file extends the existing NATS setup (setup.go handles DEFECTDOJO stream)
// by adding OSV vulnerability and scan event streams.
//
// Existing streams in setup.go (DEFECTDOJO) are NOT modified.
// This file adds: OSV_VULN, OSV_AI, OSV_SCAN streams.
package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// OSV-specific stream names — distinct from the existing DEFECTDOJO stream.
const (
	StreamOSVVuln = "OSV_VULN"
	StreamOSVAI   = "OSV_AI"
	StreamOSVScan = "OSV_SCAN"
)

// osvStreamConfigs defines all OSV-platform JetStream streams.
// The existing DEFECTDOJO stream (in setup.go) is NOT redefined here.
var osvStreamConfigs = []jetstream.StreamConfig{
	{
		Name:        StreamOSVVuln,
		Subjects:    []string{"osv.vuln.>"},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      7 * 24 * time.Hour, // 7-day retention
		MaxBytes:    10 * 1024 * 1024 * 1024,
		Replicas:    1,
		Discard:     jetstream.DiscardOld,
		Compression: jetstream.S2Compression,
	},
	{
		Name:        StreamOSVAI,
		Subjects:    []string{"osv.ai.>"},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      24 * time.Hour,
		MaxBytes:    1 * 1024 * 1024 * 1024,
		Replicas:    1,
		Discard:     jetstream.DiscardOld,
		Compression: jetstream.S2Compression,
	},
	{
		Name:        StreamOSVScan,
		Subjects:    []string{"osv.scan.>"},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      72 * time.Hour,
		MaxBytes:    5 * 1024 * 1024 * 1024,
		Replicas:    1,
		Discard:     jetstream.DiscardOld,
		Compression: jetstream.S2Compression,
	},
}

// SetupOSVStreams creates or updates all OSV-platform JetStream streams.
// Idempotent — safe to call on every service startup.
// Does NOT touch the existing DEFECTDOJO stream (handled by SetupStream in setup.go).
func SetupOSVStreams(ctx context.Context, js jetstream.JetStream) error {
	for _, cfg := range osvStreamConfigs {
		_, err := js.CreateOrUpdateStream(ctx, cfg)
		if err != nil {
			return fmt.Errorf("nats: setup stream %q: %w", cfg.Name, err)
		}
	}
	return nil
}

// SetupAllStreams sets up both the DEFECTDOJO stream and all OSV streams.
// Convenience wrapper for services that need the full event bus.
func SetupAllStreams(ctx context.Context, js jetstream.JetStream) error {
	// OSV streams (vuln, ai, scan)
	if err := SetupOSVStreams(ctx, js); err != nil {
		return err
	}

	// DEFECTDOJO stream (finding events) — using existing config from setup.go
	cfg := jetstream.StreamConfig{
		Name:        StreamName,                   // "DEFECTDOJO" — from setup.go constant
		Subjects:    []string{SubjectPrefix + ">"}, // "defectdojo.>" — from setup.go constant
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      7 * 24 * time.Hour,
		MaxBytes:    10 * 1024 * 1024 * 1024,
		Replicas:    1,
		Discard:     jetstream.DiscardOld,
		Compression: jetstream.S2Compression,
	}
	_, err := js.CreateOrUpdateStream(ctx, cfg)
	if err != nil {
		return fmt.Errorf("nats: setup DEFECTDOJO stream: %w", err)
	}
	return nil
}
