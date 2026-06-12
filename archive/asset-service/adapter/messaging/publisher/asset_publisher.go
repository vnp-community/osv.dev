// Package nats provides NATS JetStream publisher for asset lifecycle events.
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/osv/asset-service/internal/domain/entity"
	"github.com/rs/zerolog"
)

// AssetPublisher publishes asset lifecycle events to NATS JetStream.
// Subjects: asset.asset.created, asset.asset.updated, asset.vuln.linked
type AssetPublisher struct {
	js  nats.JetStreamContext
	log zerolog.Logger
}

func NewAssetPublisher(nc *nats.Conn, log zerolog.Logger) (*AssetPublisher, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("jetstream context: %w", err)
	}
	js.AddStream(&nats.StreamConfig{ //nolint:errcheck
		Name:     "ASSET",
		Subjects: []string{"asset.>"},
		Storage:  nats.FileStorage,
	})
	return &AssetPublisher{js: js, log: log}, nil
}

type AssetEvent struct {
	AssetID   uuid.UUID `json:"asset_id"`
	IPAddress string    `json:"ip_address"`
	Hostname  string    `json:"hostname"`
	OccurredAt time.Time `json:"occurred_at"`
}

type VulnLinkedEvent struct {
	AssetID    uuid.UUID       `json:"asset_id"`
	CVEID      string          `json:"cve_id"`
	Severity   entity.Severity `json:"severity"`
	CVSS       float64         `json:"cvss"`
	ScanID     uuid.UUID       `json:"scan_id"`
	OccurredAt time.Time       `json:"occurred_at"`
}

func (p *AssetPublisher) PublishAssetCreated(ctx context.Context, asset *entity.Asset) error {
	return p.publish("asset.asset.created", AssetEvent{
		AssetID: asset.ID, IPAddress: asset.IPAddress, Hostname: asset.Hostname,
		OccurredAt: time.Now().UTC(),
	})
}

func (p *AssetPublisher) PublishAssetUpdated(ctx context.Context, asset *entity.Asset) error {
	return p.publish("asset.asset.updated", AssetEvent{
		AssetID: asset.ID, IPAddress: asset.IPAddress, Hostname: asset.Hostname,
		OccurredAt: time.Now().UTC(),
	})
}

func (p *AssetPublisher) PublishVulnLinked(ctx context.Context, assetID uuid.UUID, cveID string, sev entity.Severity, cvss float64, scanID uuid.UUID) error {
	return p.publish("asset.vuln.linked", VulnLinkedEvent{
		AssetID: assetID, CVEID: cveID, Severity: sev, CVSS: cvss, ScanID: scanID,
		OccurredAt: time.Now().UTC(),
	})
}

func (p *AssetPublisher) publish(subject string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = p.js.Publish(subject, data)
	if err != nil {
		p.log.Warn().Err(err).Str("subject", subject).Msg("NATS publish failed")
	}
	return err
}
