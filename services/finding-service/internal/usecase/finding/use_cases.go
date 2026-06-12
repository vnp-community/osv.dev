// Package finding_usecase implements all use cases for the finding management service.
package finding_usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	natsutil "github.com/osv/shared/pkg/nats"
	"github.com/defectdojo/finding-service/internal/domain/finding"
)

// ─── BatchCreateFindings ─────────────────────────────────────────────────────

// FindingInput is the normalized input for creating a single finding.
type FindingInput struct {
	Title            string
	Description      string
	Mitigation       string
	Impact           string
	References       string
	Severity         string
	CVE              string
	CWE              int
	VulnIDFromTool   string
	CVSSv3           string
	CVSSv3Score      *float64
	Active           bool
	Verified         bool
	ComponentName    string
	ComponentVersion string
	FilePath         string
	LineNumber       int
	Service          string
	HashCode         string
	Tags             []string
	Endpoints        []string
}

type BatchCreateInput struct {
	TestID       uuid.UUID
	EngagementID uuid.UUID
	ProductID    uuid.UUID
	Findings     []FindingInput
}

type BatchCreateOutput struct {
	FindingIDs []string
	Count      int
}

type BatchCreateFindingsUseCase struct {
	repo     finding.Repository
	eventPub *natsutil.Publisher
}

func NewBatchCreate(repo finding.Repository, pub *natsutil.Publisher) *BatchCreateFindingsUseCase {
	return &BatchCreateFindingsUseCase{repo: repo, eventPub: pub}
}

// Execute creates findings in batches of 100 and publishes a batch_created event.
func (uc *BatchCreateFindingsUseCase) Execute(ctx context.Context, in BatchCreateInput) (*BatchCreateOutput, error) {
	const batchSize = 100
	var allIDs []string

	for i := 0; i < len(in.Findings); i += batchSize {
		end := i + batchSize
		if end > len(in.Findings) {
			end = len(in.Findings)
		}
		batch := in.Findings[i:end]

		entities := make([]*finding.Finding, 0, len(batch))
		for _, fi := range batch {
			sev := finding.Severity(fi.Severity)
			f := finding.New(fi.Title, sev, in.TestID, in.EngagementID, in.ProductID)
			f.Description = fi.Description
			f.Mitigation = fi.Mitigation
			f.Impact = fi.Impact
			f.References = fi.References
			f.CVE = fi.CVE
			f.CWE = fi.CWE
			f.VulnIDFromTool = fi.VulnIDFromTool
			f.CVSSv3 = fi.CVSSv3
			f.CVSSv3Score = fi.CVSSv3Score
			f.Active = fi.Active
			f.Verified = fi.Verified
			f.ComponentName = fi.ComponentName
			f.ComponentVersion = fi.ComponentVersion
			f.FilePath = fi.FilePath
			f.LineNumber = fi.LineNumber
			f.Service = fi.Service
			f.HashCode = fi.HashCode
			f.Tags = fi.Tags
			entities = append(entities, f)
		}

		ids, err := uc.repo.BulkCreate(ctx, entities)
		if err != nil {
			return nil, fmt.Errorf("batch create findings (batch %d-%d): %w", i, end, err)
		}
		allIDs = append(allIDs, ids...)
	}

	_ = uc.eventPub.Publish(ctx, "defectdojo.finding.batch_created", map[string]interface{}{
		"finding_ids":   allIDs,
		"test_id":       in.TestID,
		"engagement_id": in.EngagementID,
		"product_id":    in.ProductID,
		"count":         len(allIDs),
	})

	return &BatchCreateOutput{FindingIDs: allIDs, Count: len(allIDs)}, nil
}

// ─── FindByHashCode ───────────────────────────────────────────────────────────

type FindByHashCodeInput struct {
	HashCode     string
	TestID       uuid.UUID
	EngagementID *uuid.UUID
	ProductID    *uuid.UUID
	OnEngagement bool
}

type FindByHashCodeOutput struct {
	FindingID *uuid.UUID
	Status    *string
}

type FindByHashCodeUseCase struct {
	repo finding.Repository
}

func NewFindByHashCode(repo finding.Repository) *FindByHashCodeUseCase {
	return &FindByHashCodeUseCase{repo: repo}
}

func (uc *FindByHashCodeUseCase) Execute(ctx context.Context, in FindByHashCodeInput) (*FindByHashCodeOutput, error) {
	f, err := uc.repo.FindByHashCode(ctx, in.HashCode, in.TestID, in.OnEngagement, in.EngagementID, in.ProductID)
	if err != nil {
		return &FindByHashCodeOutput{}, nil // not found = no duplicate
	}
	state := string(f.CurrentState())
	return &FindByHashCodeOutput{FindingID: &f.ID, Status: &state}, nil
}

// ─── CloseOldFindings ─────────────────────────────────────────────────────────

type CloseOldFindingsInput struct {
	TestID                    uuid.UUID
	EngagementID              uuid.UUID
	DeduplicationOnEngagement bool
	ExcludeFindingIDs         []uuid.UUID
	MitigatedByID             uuid.UUID
}

type CloseOldFindingsOutput struct {
	Closed int
}

type CloseOldFindingsUseCase struct {
	repo     finding.Repository
	eventPub *natsutil.Publisher
}

func NewCloseOldFindings(repo finding.Repository, pub *natsutil.Publisher) *CloseOldFindingsUseCase {
	return &CloseOldFindingsUseCase{repo: repo, eventPub: pub}
}

func (uc *CloseOldFindingsUseCase) Execute(ctx context.Context, in CloseOldFindingsInput) (*CloseOldFindingsOutput, error) {
	// Find all active findings in test not in the current import
	actives, err := uc.repo.FindActiveByTest(ctx, in.TestID, in.ExcludeFindingIDs)
	if err != nil {
		return nil, err
	}

	if len(actives) == 0 {
		return &CloseOldFindingsOutput{Closed: 0}, nil
	}

	ids := make([]uuid.UUID, len(actives))
	for i, f := range actives {
		ids[i] = f.ID
	}

	if err := uc.repo.BulkSetMitigated(ctx, ids, in.MitigatedByID); err != nil {
		return nil, err
	}

	_ = uc.eventPub.Publish(ctx, "defectdojo.finding.bulk_closed", map[string]interface{}{
		"test_id":      in.TestID,
		"closed_count": len(ids),
		"closed_by":    in.MitigatedByID,
	})

	return &CloseOldFindingsOutput{Closed: len(ids)}, nil
}

// ─── Status transitions ───────────────────────────────────────────────────────

type StatusTransitionUseCase struct {
	repo     finding.Repository
	eventPub *natsutil.Publisher
}

func NewStatusTransition(repo finding.Repository, pub *natsutil.Publisher) *StatusTransitionUseCase {
	return &StatusTransitionUseCase{repo: repo, eventPub: pub}
}

func (uc *StatusTransitionUseCase) Close(ctx context.Context, id uuid.UUID, mitigatedBy string) error {
	return uc.transition(ctx, id, finding.StateMitigated, func(f *finding.Finding) error {
		return f.Close(&mitigatedBy)
	})
}

func (uc *StatusTransitionUseCase) Reopen(ctx context.Context, id uuid.UUID) error {
	return uc.transition(ctx, id, finding.StateActive, func(f *finding.Finding) error {
		return f.Reopen()
	})
}

func (uc *StatusTransitionUseCase) MarkFalsePositive(ctx context.Context, id uuid.UUID) error {
	return uc.transition(ctx, id, finding.StateFalsePositive, func(f *finding.Finding) error {
		return f.MarkFalsePositive()
	})
}

func (uc *StatusTransitionUseCase) AcceptRisk(ctx context.Context, id uuid.UUID) error {
	return uc.transition(ctx, id, finding.StateRiskAccepted, func(f *finding.Finding) error {
		return f.AcceptRisk()
	})
}

func (uc *StatusTransitionUseCase) transition(ctx context.Context, id uuid.UUID, next finding.FindingState, mutate func(*finding.Finding) error) error {
	f, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("finding not found: %w", err)
	}
	oldState := f.CurrentState()
	if err := mutate(f); err != nil {
		return err
	}
	if err := uc.repo.Save(ctx, f); err != nil {
		return err
	}
	_ = uc.eventPub.Publish(ctx, "defectdojo.finding.status_changed", map[string]interface{}{
		"finding_id": id,
		"product_id": f.ProductID,
		"old_state":  oldState,
		"new_state":  next,
	})
	return nil
}

// ─── BatchUpdateSLADates ─────────────────────────────────────────────────────

type SLAUpdate struct {
	FindingID      uuid.UUID
	ExpirationDate time.Time
}

type BatchUpdateSLADatesInput struct {
	Updates []SLAUpdate
}

type BatchUpdateSLADatesUseCase struct {
	repo     finding.Repository
	eventPub *natsutil.Publisher
}

func NewBatchUpdateSLADates(repo finding.Repository, pub *natsutil.Publisher) *BatchUpdateSLADatesUseCase {
	return &BatchUpdateSLADatesUseCase{repo: repo, eventPub: pub}
}

func (uc *BatchUpdateSLADatesUseCase) Execute(ctx context.Context, in BatchUpdateSLADatesInput) error {
	updates := make([]finding.SLADateUpdate, len(in.Updates))
	for i, u := range in.Updates {
		updates[i] = finding.SLADateUpdate{FindingID: u.FindingID, ExpirationDate: u.ExpirationDate}
	}
	if err := uc.repo.BulkUpdateSLADates(ctx, updates); err != nil {
		return err
	}
	for _, u := range in.Updates {
		_ = uc.eventPub.Publish(ctx, "defectdojo.finding.sla_date_updated", map[string]interface{}{
			"finding_id":          u.FindingID,
			"sla_expiration_date": u.ExpirationDate,
		})
	}
	return nil
}

// ─── Integration Hooks ────────────────────────────────────────────────────────

// AuditRecorder is the port for recording finding lifecycle events to the audit log.
// Inject the audit.RecordEventUseCase implementation to wire audit recording.
type AuditRecorder interface {
	// RecordFindingEvent records a finding lifecycle event (created, status_changed, etc.).
	RecordFindingEvent(ctx context.Context, eventType string, findingID uuid.UUID, actorID string) error
}

// SLAComputer is the port for computing SLA expiration dates.
// Inject the sla.ComputeSLAUseCase implementation to auto-set SLA deadlines.
type SLAComputer interface {
	// ComputeDeadline returns the SLA expiration date for a given severity and product.
	ComputeDeadline(ctx context.Context, productID uuid.UUID, severity string) (*time.Time, error)
}
