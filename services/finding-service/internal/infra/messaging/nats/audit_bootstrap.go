// Package nats provides the NATS event bootstrap for finding-service.
// This file adds the audit subscriber (wildcard defectdojo.>) on top of the
// existing finding/sla NATS consumers.
package nats

import (
	"context"

	"github.com/rs/zerolog"

	natsutil "github.com/osv/shared/pkg/nats"
	auditdomain "github.com/defectdojo/finding-service/internal/domain/audit"
	auditusecase "github.com/defectdojo/finding-service/internal/usecase/audit"
)

// BootstrapAudit subscribes to all DefectDojo events for immutable audit recording.
// Subject: "defectdojo.>" (wildcard — captures every DD event)
func BootstrapAudit(ctx context.Context, sub *natsutil.Subscriber, repo auditdomain.Repository, log zerolog.Logger) error {
	uc := auditusecase.NewRecordEvent(repo)

	if err := sub.Subscribe(ctx, "defectdojo.>", func(ctx context.Context, event *natsutil.CloudEvent) error {
		return uc.RecordEvent(ctx, event)
	}); err != nil {
		return err
	}
	log.Info().Msg("audit: subscribed to defectdojo.> (all events)")
	return nil
}
