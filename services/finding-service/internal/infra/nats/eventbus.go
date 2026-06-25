package nats

import (
	"context"

	"github.com/nats-io/nats.go/jetstream"
	natsutil "github.com/osv/shared/pkg/nats"
)

// noopJetStream implements only the Publish method needed by natsutil.Publisher.
// All published messages are silently discarded.
type noopJetStream struct {
	jetstream.JetStream // embed to satisfy full interface
}

func (n *noopJetStream) Publish(_ context.Context, _ string, _ []byte, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	return &jetstream.PubAck{}, nil
}

// NewNoopPublisher creates a Publisher that silently drops all events.
// Used when NATS is unavailable in embedded/monolith mode.
func NewNoopPublisher() *natsutil.Publisher {
	return natsutil.NewPublisher(&noopJetStream{}, "finding-service/noop")
}
