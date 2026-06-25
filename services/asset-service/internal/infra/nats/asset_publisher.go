package nats

import (
	"encoding/json"
	"fmt"

	natsgo "github.com/nats-io/nats.go"
)

// AssetEventPublisher publishes asset domain events to NATS.
// Implements the ucasset.EventPublisher interface.
type AssetEventPublisher struct {
	nc *natsgo.Conn
}

// NewAssetEventPublisher creates a new AssetEventPublisher.
func NewAssetEventPublisher(nc *natsgo.Conn) *AssetEventPublisher {
	return &AssetEventPublisher{nc: nc}
}

// Publish publishes an event with subject "asset.<subject>".
func (p *AssetEventPublisher) Publish(subject string, payload map[string]any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("asset event marshal: %w", err)
	}
	return p.nc.Publish("asset."+subject, data)
}
