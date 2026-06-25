// Package riskacceptance_uc implements use cases for risk acceptance management.
package riskacceptance_uc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/member"
	"github.com/osv/finding-service/internal/domain/riskacceptance"
	"github.com/osv/finding-service/internal/domain/finding"
)

var (
	ErrNotOwner           = errors.New("only product Owner or Maintainer can manage risk acceptances")
	ErrFindingNotInProduct = errors.New("finding does not belong to this product")
)

// EventPublisher publishes NATS events.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload map[string]any) error
}

// ─── Create ───────────────────────────────────────────────────────────────────

// CreateRiskAcceptanceInput is the request for creating a risk acceptance.
type CreateRiskAcceptanceInput struct {
	Name                     string
	ProductID                uuid.UUID
	RequesterUserID          uuid.UUID
	FindingIDs               []uuid.UUID
	ExpirationDate           *time.Time
	Notes                    string
	ProofFileKey             string
	ReactivateExpired        bool
	ReactivateNoteText       string
	RestartSLAOnReactivation bool
}

// CreateRiskAcceptanceUseCase creates a risk acceptance and transitions findings.
type CreateRiskAcceptanceUseCase struct {
	raRepo      riskacceptance.Repository
	memberRepo  member.ProductMemberRepository
	findingRepo finding.Repository
	eventPub    EventPublisher
}

// NewCreate creates a new CreateRiskAcceptanceUseCase.
func NewCreate(
	ra riskacceptance.Repository,
	m member.ProductMemberRepository,
	f finding.Repository,
	ep EventPublisher,
) *CreateRiskAcceptanceUseCase {
	return &CreateRiskAcceptanceUseCase{raRepo: ra, memberRepo: m, findingRepo: f, eventPub: ep}
}

// Execute creates a risk acceptance and marks all findings as RiskAccepted.
func (uc *CreateRiskAcceptanceUseCase) Execute(ctx context.Context, in CreateRiskAcceptanceInput) (*riskacceptance.RiskAcceptance, error) {
	// 1. Check requester is Owner or Maintainer
	role, err := uc.memberRepo.GetRole(ctx, in.ProductID, in.RequesterUserID)
	if err != nil || role == nil {
		return nil, ErrNotOwner
	}
	if *role != member.RoleOwner && *role != member.RoleMaintainer {
		return nil, ErrNotOwner
	}

	// 2. Create the risk acceptance record
	ra := riskacceptance.New(in.ProductID, in.RequesterUserID, in.Name)
	ra.ExpirationDate = in.ExpirationDate
	ra.Notes = in.Notes
	ra.ProofFileKey = in.ProofFileKey
	ra.ReactivateExpired = in.ReactivateExpired
	ra.ReactivateNoteText = in.ReactivateNoteText
	ra.RestartSLAOnReactivation = in.RestartSLAOnReactivation
	ra.FindingIDs = in.FindingIDs

	if err := uc.raRepo.Save(ctx, ra); err != nil {
		return nil, err
	}

	// 3. Link each finding to the RA and transition to RiskAccepted state
	for _, fid := range in.FindingIDs {
		f, err := uc.findingRepo.FindByID(ctx, fid)
		if err != nil {
			continue
		}
		if err := f.AcceptRisk(); err != nil {
			continue // already in non-transitionable state, skip
		}
		_ = uc.findingRepo.Save(ctx, f)
		_ = uc.raRepo.AddFinding(ctx, ra.ID, fid)
	}

	// 4. Publish event (eventPub may be nil when NATS is not configured)
	if uc.eventPub != nil {
		_ = uc.eventPub.Publish(ctx, "defectdojo.risk_acceptance.created", map[string]any{
			"risk_acceptance_id": ra.ID.String(),
			"product_id":         in.ProductID.String(),
			"finding_count":      len(in.FindingIDs),
			"expiration_date":    formatDate(in.ExpirationDate),
		})
	}

	return ra, nil
}

// ─── ExpireRiskAcceptances ────────────────────────────────────────────────────

// ExpireRiskAcceptancesUseCase runs daily and expires RAs past their expiration date.
type ExpireRiskAcceptancesUseCase struct {
	raRepo      riskacceptance.Repository
	findingRepo finding.Repository
	eventPub    EventPublisher
}

// NewExpire creates a new ExpireRiskAcceptancesUseCase.
func NewExpire(ra riskacceptance.Repository, f finding.Repository, ep EventPublisher) *ExpireRiskAcceptancesUseCase {
	return &ExpireRiskAcceptancesUseCase{raRepo: ra, findingRepo: f, eventPub: ep}
}

// Execute expires all RAs with expiration_date <= today.
// If reactivate_expired=true, findings are transitioned back to Active.
func (uc *ExpireRiskAcceptancesUseCase) Execute(ctx context.Context) error {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	expiring, err := uc.raRepo.ListExpiring(ctx, today)
	if err != nil {
		return err
	}

	for _, ra := range expiring {
		if err := uc.raRepo.MarkExpired(ctx, ra.ID); err != nil {
			continue
		}

		_ = uc.eventPub.Publish(ctx, "defectdojo.risk_acceptance.expired", map[string]any{
			"risk_acceptance_id": ra.ID.String(),
			"product_id":         ra.ProductID.String(),
			"finding_ids":        uuidsToStrings(ra.FindingIDs),
			"reactivate":         ra.ReactivateExpired,
		})

		if !ra.ReactivateExpired {
			continue
		}

		// Reactivate each finding
		for _, fid := range ra.FindingIDs {
			f, err := uc.findingRepo.FindByID(ctx, fid)
			if err != nil {
				continue
			}
			if err := f.Reopen(); err != nil {
				continue
			}
			_ = uc.findingRepo.Save(ctx, f)
		}
	}
	return nil
}

// ─── RemoveFinding ────────────────────────────────────────────────────────────

// RemoveFindingInput is the request for removing a finding from a risk acceptance.
type RemoveFindingInput struct {
	RAID            uuid.UUID
	FindingID       uuid.UUID
	RequesterUserID uuid.UUID
}

// RemoveFindingFromRAUseCase removes a finding from a risk acceptance.
type RemoveFindingFromRAUseCase struct {
	raRepo      riskacceptance.Repository
	memberRepo  member.ProductMemberRepository
	findingRepo finding.Repository
}

// NewRemoveFinding creates a new RemoveFindingFromRAUseCase.
func NewRemoveFinding(ra riskacceptance.Repository, m member.ProductMemberRepository, f finding.Repository) *RemoveFindingFromRAUseCase {
	return &RemoveFindingFromRAUseCase{raRepo: ra, memberRepo: m, findingRepo: f}
}

// Execute removes a finding from a risk acceptance. Only Owner/Maintainer may do this.
func (uc *RemoveFindingFromRAUseCase) Execute(ctx context.Context, in RemoveFindingInput) error {
	ra, err := uc.raRepo.FindByID(ctx, in.RAID)
	if err != nil {
		return errors.New("risk acceptance not found")
	}

	role, _ := uc.memberRepo.GetRole(ctx, ra.ProductID, in.RequesterUserID)
	if role == nil || (*role != member.RoleOwner && *role != member.RoleMaintainer) {
		return ErrNotOwner
	}

	if err := uc.raRepo.RemoveFinding(ctx, in.RAID, in.FindingID); err != nil {
		return err
	}

	// Reopen the finding (transition back to Active)
	f, err := uc.findingRepo.FindByID(ctx, in.FindingID)
	if err == nil {
		_ = f.Reopen()
		_ = uc.findingRepo.Save(ctx, f)
	}
	return nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func formatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func uuidsToStrings(ids []uuid.UUID) []string {
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}
	return strs
}
