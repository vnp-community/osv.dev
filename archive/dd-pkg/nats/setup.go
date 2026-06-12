package nats

import (
	"context"
	"fmt"
	"time"

	natspkg "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// StreamName is the single JetStream stream for all DefectDojo events.
const StreamName = "DEFECTDOJO"

// SubjectPrefix is the root subject prefix for all DefectDojo events.
const SubjectPrefix = "defectdojo."

// Connect creates a NATS connection with automatic reconnect logic.
func Connect(url string) (*natspkg.Conn, error) {
	return natspkg.Connect(url,
		natspkg.RetryOnFailedConnect(true),
		natspkg.MaxReconnects(-1),           // reconnect forever
		natspkg.ReconnectWait(2*time.Second),
		natspkg.DisconnectErrHandler(func(_ *natspkg.Conn, err error) {
			if err != nil {
				// caller can observe via Conn.Status()
				_ = err
			}
		}),
	)
}

// SetupStream creates or updates the DEFECTDOJO JetStream stream.
// Safe to call on every service startup — the call is idempotent.
func SetupStream(ctx context.Context, nc *natspkg.Conn) (jetstream.JetStream, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("nats: create jetstream: %w", err)
	}

	cfg := jetstream.StreamConfig{
		Name:        StreamName,
		Subjects:    []string{SubjectPrefix + ">"},  // wildcard: all DD events
		MaxAge:      7 * 24 * time.Hour,             // 7-day retention
		MaxBytes:    10 * 1024 * 1024 * 1024,        // 10 GB max
		Storage:     jetstream.FileStorage,
		Replicas:    1,                              // increase to 3 in production clusters
		Retention:   jetstream.LimitsPolicy,
		Discard:     jetstream.DiscardOld,
		Compression: jetstream.S2Compression,
	}

	_, err = js.CreateOrUpdateStream(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("nats: create stream %q: %w", StreamName, err)
	}

	return js, nil
}
