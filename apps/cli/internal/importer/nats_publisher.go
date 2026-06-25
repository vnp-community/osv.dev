// Package importer — nats_publisher.go provides a NATS JetStream alternative
// to the existing GCP Pub/Sub publisher (clients.GCPPublisher).
//
// Activated when CLI_BACKEND=microservices in cmd/importer.
// Existing importer.go and all GCP code are NOT modified.
//
// Flow: importer discovers changed vulnerability → NATSPublisher.PublishVuln()
// → NATS JetStream "OSV_VULN" stream → subject "osv.vuln.imported"
// → data-service CVEImportConsumer ingests the record.
package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	natspkg "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// vulnImportedPayload is the event body published to "osv.vuln.imported".
// Mirrors shared/pkg/events.VulnImportedEvent — duplicated intentionally
// to avoid introducing a shared/pkg dependency in the CLI module.
type vulnImportedPayload struct {
	ID         string    `json:"id"`
	Source     string    `json:"source"`
	ImportedAt time.Time `json:"imported_at"`
	OSVData    []byte    `json:"osv_data"`
	TraceID    string    `json:"trace_id,omitempty"`
}

const (
	natsSubjectVulnImported = "osv.vuln.imported"
	natsStreamOSVVuln       = "OSV_VULN"
)

// NATSPublisher publishes vulnerability import events to NATS JetStream.
// It is a drop-in alternative to clients.GCPPublisher for the microservices backend.
type NATSPublisher struct {
	nc  *natspkg.Conn
	js  jetstream.JetStream
	url string
}

// NewNATSPublisher connects to NATS and ensures the OSV_VULN stream exists.
// natsURL example: "nats://localhost:4222"
func NewNATSPublisher(natsURL string) (*NATSPublisher, error) {
	nc, err := natspkg.Connect(natsURL,
		natspkg.Name("osv-cli-importer"),
		natspkg.RetryOnFailedConnect(true),
		natspkg.MaxReconnects(10),
		natspkg.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect %s: %w", natsURL, err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("nats jetstream: %w", err)
	}

	// Ensure OSV_VULN stream exists (idempotent).
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        natsStreamOSVVuln,
		Subjects:    []string{"osv.vuln.>"},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      7 * 24 * time.Hour,
		MaxBytes:    10 * 1024 * 1024 * 1024,
		Replicas:    1,
		Discard:     jetstream.DiscardOld,
		Compression: jetstream.S2Compression,
	})
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("nats ensure stream OSV_VULN: %w", err)
	}

	return &NATSPublisher{nc: nc, js: js, url: natsURL}, nil
}

// PublishVuln publishes a single vulnerability import event.
// id: CVE/OSV ID (e.g. "CVE-2021-44228")
// source: feed name (e.g. "nvd", "ghsa", "ossfuzz")
// osvJSON: JSON-encoded OSV schema Vulnerability record
func (p *NATSPublisher) PublishVuln(ctx context.Context, id, source string, osvJSON []byte) error {
	payload := vulnImportedPayload{
		ID:         id,
		Source:     source,
		ImportedAt: time.Now().UTC(),
		OSVData:    osvJSON,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal vuln event: %w", err)
	}
	_, err = p.js.Publish(ctx, natsSubjectVulnImported, body)
	if err != nil {
		return fmt.Errorf("nats publish %s: %w", natsSubjectVulnImported, err)
	}
	return nil
}

// Close releases the NATS connection.
func (p *NATSPublisher) Close() {
	if p.nc != nil {
		p.nc.Drain() //nolint:errcheck
		p.nc.Close()
	}
}
