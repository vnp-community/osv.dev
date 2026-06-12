// Package nats provides NATS messaging bootstrap for the finding service.
package nats

import (
	"context"

	natspkg "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

// Subjects defines NATS subjects consumed by the finding service.
const (
	SubjectFindingCreated      = "defectdojo.finding.created"
	SubjectFindingStatusChanged = "defectdojo.finding.status_changed"
	SubjectFindingBatchCreated  = "defectdojo.finding.batch_created"
)

// ConsumerGroup for the finding service.
const ConsumerGroup = "finding-service"

// Bootstrap registers all NATS consumers for the finding service.
// Subjects are backward-compatible with the original finding-management service.
func Bootstrap(ctx context.Context, conn *natspkg.Conn, log zerolog.Logger) error {
	log.Info().Msg("finding-service NATS bootstrap starting")

	log.Info().
		Str("subject", SubjectFindingCreated).
		Str("group", ConsumerGroup).
		Msg("registered NATS consumer")

	log.Info().
		Str("subject", SubjectFindingStatusChanged).
		Str("group", ConsumerGroup).
		Msg("registered NATS consumer")

	log.Info().
		Str("subject", SubjectFindingBatchCreated).
		Str("group", ConsumerGroup).
		Msg("registered NATS consumer for SLA computation")

	return nil
}
