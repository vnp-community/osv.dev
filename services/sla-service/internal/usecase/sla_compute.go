// Package slacompute implements SLA expiration date computation and breach detection.
package slacompute

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// FindingForSLA represents a finding's data needed for SLA computation.
type FindingForSLA struct {
	ID         uuid.UUID
	ProductID  uuid.UUID
	Severity   string
	FoundDate  time.Time
}

// SLAConfigResolver resolves the SLA configuration for a product.
type SLAConfigResolver interface {
	// GetConfigForProduct returns the SLA config for the product.
	// Falls back to the default config if no product-specific config is assigned.
	GetConfigForProduct(ctx context.Context, productID uuid.UUID) (*SLAConfig, error)
}

// SLAConfig holds the resolved days per severity.
type SLAConfig struct {
	ID           uuid.UUID
	CriticalDays int
	HighDays     int
	MediumDays   int
	LowDays      int
}

// DaysForSeverity returns remediation days for the given severity string.
func (c *SLAConfig) DaysForSeverity(severity string) int {
	switch severity {
	case "Critical":
		return c.CriticalDays
	case "High":
		return c.HighDays
	case "Medium":
		return c.MediumDays
	case "Low":
		return c.LowDays
	default:
		return 0
	}
}

// SLAAssignmentRepository persists SLA expiry assignments.
type SLAAssignmentRepository interface {
	// Upsert creates or updates the SLA expiry for a finding.
	Upsert(ctx context.Context, a *SLAAssignment) error
	// ListExpiring returns findings with expiration_date <= today and is_breached=false.
	ListExpiring(ctx context.Context, today time.Time) ([]*SLAAssignment, error)
	// MarkBreached sets is_breached=true.
	MarkBreached(ctx context.Context, findingID uuid.UUID) error
}

// SLAAssignment is the computed SLA assignment for a single finding.
type SLAAssignment struct {
	FindingID          uuid.UUID
	ProductID          uuid.UUID
	Severity           string
	SLAConfigurationID uuid.UUID
	FoundDate          time.Time
	ExpirationDate     time.Time
	IsBreached         bool
	LastComputedAt     time.Time
}

// FindingServiceClient sends computed SLA dates back to finding-service.
type FindingServiceClient interface {
	BatchUpdateSLADates(ctx context.Context, updates []SLADateUpdate) error
}

// SLADateUpdate is a single SLA date assignment to push to finding-service.
type SLADateUpdate struct {
	FindingID      uuid.UUID
	ExpirationDate time.Time
}

// EventPublisher publishes NATS events.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload map[string]any) error
}

// ─── ComputeSLAUseCase ────────────────────────────────────────────────────────

// ComputeSLAUseCase computes and stores SLA expiration dates for given findings.
type ComputeSLAUseCase struct {
	configResolver SLAConfigResolver
	assignmentRepo SLAAssignmentRepository
	findingClient  FindingServiceClient
	eventPub       EventPublisher
}

// NewComputeSLA creates a new ComputeSLAUseCase.
func NewComputeSLA(
	cr SLAConfigResolver,
	ar SLAAssignmentRepository,
	fc FindingServiceClient,
	ep EventPublisher,
) *ComputeSLAUseCase {
	return &ComputeSLAUseCase{
		configResolver: cr,
		assignmentRepo: ar,
		findingClient:  fc,
		eventPub:       ep,
	}
}

// Execute computes SLA expiration dates for the given findings and
// pushes them to finding-service via gRPC.
func (uc *ComputeSLAUseCase) Execute(ctx context.Context, findings []FindingForSLA) error {
	if len(findings) == 0 {
		return nil
	}

	var updates []SLADateUpdate
	var failed int

	for _, f := range findings {
		cfg, err := uc.configResolver.GetConfigForProduct(ctx, f.ProductID)
		if err != nil {
			failed++
			continue
		}

		days := cfg.DaysForSeverity(f.Severity)
		if days == 0 {
			continue // no SLA for Info severity
		}

		expiry := f.FoundDate.Add(time.Duration(days) * 24 * time.Hour)

		assignment := &SLAAssignment{
			FindingID:          f.ID,
			ProductID:          f.ProductID,
			Severity:           f.Severity,
			SLAConfigurationID: cfg.ID,
			FoundDate:          f.FoundDate,
			ExpirationDate:     expiry,
			LastComputedAt:     time.Now().UTC(),
		}

		if err := uc.assignmentRepo.Upsert(ctx, assignment); err != nil {
			failed++
			continue
		}

		updates = append(updates, SLADateUpdate{
			FindingID:      f.ID,
			ExpirationDate: expiry,
		})
	}

	// Push SLA dates to finding-service in one batch call
	if len(updates) > 0 {
		if err := uc.findingClient.BatchUpdateSLADates(ctx, updates); err != nil {
			return fmt.Errorf("batch update SLA dates: %w", err)
		}
	}

	return nil
}

// ─── DetectBreachesUseCase ────────────────────────────────────────────────────

// DetectBreachesUseCase is run daily to find findings past their SLA deadline.
type DetectBreachesUseCase struct {
	assignmentRepo SLAAssignmentRepository
	eventPub       EventPublisher
}

// NewDetectBreaches creates a new DetectBreachesUseCase.
func NewDetectBreaches(ar SLAAssignmentRepository, ep EventPublisher) *DetectBreachesUseCase {
	return &DetectBreachesUseCase{assignmentRepo: ar, eventPub: ep}
}

// Execute marks breached findings and publishes sla.breach events.
func (uc *DetectBreachesUseCase) Execute(ctx context.Context) error {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	expiring, err := uc.assignmentRepo.ListExpiring(ctx, today)
	if err != nil {
		return err
	}

	for _, a := range expiring {
		if err := uc.assignmentRepo.MarkBreached(ctx, a.FindingID); err != nil {
			continue
		}

		daysOverdue := int(today.Sub(a.ExpirationDate).Hours() / 24)

		_ = uc.eventPub.Publish(ctx, "defectdojo.sla.breach", map[string]any{
			"finding_id":      a.FindingID.String(),
			"product_id":      a.ProductID.String(),
			"severity":        a.Severity,
			"expiration_date": a.ExpirationDate.Format("2006-01-02"),
			"days_overdue":    daysOverdue,
		})
	}
	return nil
}
