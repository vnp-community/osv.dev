// Package usecase contains SLA computation and breach-checking use cases.
package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	natsutil "github.com/defectdojo/pkg/nats"
	findingv1 "github.com/defectdojo/proto/finding/v1"
	"github.com/defectdojo/sla/internal/domain"
)

// FindingBatchCreatedPayload matches the NATS event from Finding Service.
type FindingBatchCreatedPayload struct {
	FindingIDs   []string `json:"finding_ids"`
	TestID       string   `json:"test_id"`
	EngagementID string   `json:"engagement_id"`
	ProductID    string   `json:"product_id"`
	Count        int      `json:"count"`
}

// ─── ComputeSLAUseCase ───────────────────────────────────────────────────────

// ComputeSLAUseCase is triggered by defectdojo.finding.batch_created.
type ComputeSLAUseCase struct {
	slaConfigRepo domain.SLAConfigRepository
	findingClient findingv1.FindingServiceClient
	eventPub      *natsutil.Publisher
}

func NewComputeSLA(repo domain.SLAConfigRepository, fc findingv1.FindingServiceClient, pub *natsutil.Publisher) *ComputeSLAUseCase {
	return &ComputeSLAUseCase{slaConfigRepo: repo, findingClient: fc, eventPub: pub}
}

// Execute resolves the SLA config for the product, computes expiry dates, and calls Finding gRPC.
func (uc *ComputeSLAUseCase) Execute(ctx context.Context, payload *FindingBatchCreatedPayload) error {
	productID, err := uuid.Parse(payload.ProductID)
	if err != nil {
		return fmt.Errorf("invalid product_id: %w", err)
	}

	// 1. Get SLA config (product → global → in-memory default)
	cfg, err := uc.slaConfigRepo.FindByProductID(ctx, productID)
	if err != nil {
		cfg, err = uc.slaConfigRepo.FindGlobal(ctx)
		if err != nil {
			cfg = domain.NewDefault()
		}
	}

	// 2. Fetch finding details from Finding Service
	resp, err := uc.findingClient.ListFindingsForSLACheck(ctx, &findingv1.ListFindingsForSLACheckRequest{
		FindingIds: payload.FindingIDs,
	})
	if err != nil {
		return fmt.Errorf("list_findings_for_sla_check: %w", err)
	}

	// 3. Compute expiration dates
	updates := make([]*findingv1.SLAUpdate, 0, len(resp.Findings))
	for _, f := range resp.Findings {
		findingDate := f.Date.AsTime()
		expDate := cfg.ComputeExpirationDate(f.Severity, findingDate)
		if expDate == nil {
			continue
		}
		updates = append(updates, &findingv1.SLAUpdate{
			FindingId:      f.Id,
			ExpirationDate: timestamppb.New(*expDate),
		})
	}

	if len(updates) == 0 {
		return nil
	}

	// 4. Push SLA dates to Finding Service
	_, err = uc.findingClient.BatchUpdateSLADates(ctx, &findingv1.BatchUpdateSLADatesRequest{
		Updates: updates,
	})
	return err
}

// ─── CheckBreachesUseCase ────────────────────────────────────────────────────

// CheckBreachesUseCase checks all active findings for SLA breaches. Runs hourly.
type CheckBreachesUseCase struct {
	findingClient findingv1.FindingServiceClient
	eventPub      *natsutil.Publisher
}

func NewCheckBreaches(fc findingv1.FindingServiceClient, pub *natsutil.Publisher) *CheckBreachesUseCase {
	return &CheckBreachesUseCase{findingClient: fc, eventPub: pub}
}

// Execute checks active findings with SLA dates and publishes breach/warning events.
func (uc *CheckBreachesUseCase) Execute(ctx context.Context) error {
	resp, err := uc.findingClient.ListFindingsForSLACheck(ctx, &findingv1.ListFindingsForSLACheckRequest{
		ActiveOnly: true,
		HasSlaDate: true,
	})
	if err != nil {
		return fmt.Errorf("check_breaches: list findings: %w", err)
	}

	now := time.Now().UTC()
	warnThreshold := now.Add(7 * 24 * time.Hour)

	for _, f := range resp.Findings {
		if f.SlaExpirationDate == nil {
			continue
		}
		exp := f.SlaExpirationDate.AsTime()

		if exp.Before(now) {
			daysOverdue := int(now.Sub(exp).Hours() / 24)
			_ = uc.eventPub.Publish(ctx, "defectdojo.sla.breached", map[string]interface{}{
				"finding_id":      f.Id,
				"product_id":      f.ProductId,
				"expiration_date": exp,
				"days_overdue":    daysOverdue,
				"severity":        f.Severity,
			})
		} else if exp.Before(warnThreshold) {
			daysRemaining := int(exp.Sub(now).Hours() / 24)
			_ = uc.eventPub.Publish(ctx, "defectdojo.sla.expiring_soon", map[string]interface{}{
				"finding_id":     f.Id,
				"product_id":     f.ProductId,
				"days_remaining": daysRemaining,
				"severity":       f.Severity,
			})
		}
	}
	return nil
}
