// Package finding_usecase — bulk.go
// BulkUpdateFindingsUseCase implements batch state transitions and tag operations.
package finding_usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	natsutil "github.com/osv/shared/pkg/nats"
	"github.com/osv/finding-service/internal/domain/finding"
)

// ErrBulkLimitExceeded is returned when trying to bulk-operate on more than 1000 findings.
var ErrBulkLimitExceeded = errors.New("bulk operation limited to 1000 findings")

// BulkOperation specifies the type of bulk mutation.
type BulkOperation string

const (
	BulkOpClose       BulkOperation = "close"
	BulkOpReopen      BulkOperation = "reopen"
	BulkOpFalsePos    BulkOperation = "false_positive"
	BulkOpOutOfScope  BulkOperation = "out_of_scope"
	BulkOpAcceptRisk  BulkOperation = "accept_risk"
	BulkOpDelete      BulkOperation = "delete"
	BulkOpAddTags     BulkOperation = "add_tags"
	BulkOpRemoveTags  BulkOperation = "remove_tags"
	BulkOpSetSeverity BulkOperation = "set_severity"
)

// BulkUpdateInput describes a batch finding mutation.
type BulkUpdateInput struct {
	FindingIDs  []uuid.UUID
	Operation   BulkOperation
	Tags        []string // for add_tags / remove_tags
	Severity    string   // for set_severity
	RequesterID uuid.UUID
	ProductID   uuid.UUID
}

// BulkUpdateResult summarizes the outcome.
type BulkUpdateResult struct {
	Updated int
	Failed  int
	Errors  []string
}

// BulkUpdateFindingsUseCase applies a single operation to many findings.
type BulkUpdateFindingsUseCase struct {
	repo     finding.Repository
	eventPub *natsutil.Publisher
}

// NewBulkUpdate creates a new BulkUpdateFindingsUseCase.
func NewBulkUpdate(repo finding.Repository, pub *natsutil.Publisher) *BulkUpdateFindingsUseCase {
	return &BulkUpdateFindingsUseCase{repo: repo, eventPub: pub}
}

// Execute runs the bulk operation against each finding ID.
// Partial failures are collected but do not abort the entire batch.
func (uc *BulkUpdateFindingsUseCase) Execute(ctx context.Context, in BulkUpdateInput) (*BulkUpdateResult, error) {
	if len(in.FindingIDs) == 0 {
		return &BulkUpdateResult{}, nil
	}
	if len(in.FindingIDs) > 1000 {
		return nil, ErrBulkLimitExceeded
	}

	result := &BulkUpdateResult{}

	for _, fid := range in.FindingIDs {
		var err error
		switch in.Operation {
		case BulkOpClose, BulkOpReopen, BulkOpFalsePos, BulkOpOutOfScope, BulkOpAcceptRisk:
			err = uc.applyStateTransition(ctx, fid, in.Operation, in.RequesterID)
		case BulkOpDelete:
			err = uc.repo.Delete(ctx, fid)
		case BulkOpAddTags, BulkOpRemoveTags, BulkOpSetSeverity:
			err = uc.applyMutation(ctx, fid, in)
		default:
			err = fmt.Errorf("unknown bulk operation: %s", in.Operation)
		}

		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", fid, err))
		} else {
			result.Updated++
		}
	}

	_ = uc.eventPub.Publish(ctx, "defectdojo.finding.bulk_updated", map[string]interface{}{
		"finding_ids": uuidsToStrings(in.FindingIDs),
		"operation":   string(in.Operation),
		"product_id":  in.ProductID,
		"updated":     result.Updated,
		"failed":      result.Failed,
	})

	return result, nil
}

func (uc *BulkUpdateFindingsUseCase) applyStateTransition(ctx context.Context, id uuid.UUID, op BulkOperation, requesterID uuid.UUID) error {
	f, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("finding not found: %w", err)
	}
	switch op {
	case BulkOpClose:
		err = f.Close(ptr(requesterID.String()))
	case BulkOpReopen:
		err = f.Reopen()
	case BulkOpFalsePos:
		err = f.MarkFalsePositive()
	case BulkOpOutOfScope:
		err = f.MarkOutOfScope()
	case BulkOpAcceptRisk:
		err = f.AcceptRisk()
	}
	if err != nil {
		return err
	}
	return uc.repo.Save(ctx, f)
}

func (uc *BulkUpdateFindingsUseCase) applyMutation(ctx context.Context, id uuid.UUID, in BulkUpdateInput) error {
	f, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("finding not found: %w", err)
	}
	switch in.Operation {
	case BulkOpAddTags:
		f.Tags = appendUnique(f.Tags, in.Tags)
	case BulkOpRemoveTags:
		f.Tags = removeAll(f.Tags, in.Tags)
	case BulkOpSetSeverity:
		if in.Severity != "" {
			f.Severity = finding.Severity(in.Severity)
			f.NumericalSeverity = f.Severity.Numerical()
		}
	}
	return uc.repo.Save(ctx, f)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func ptr(s string) *string { return &s }

func uuidsToStrings(ids []uuid.UUID) []string {
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}
	return strs
}

func appendUnique(existing, extra []string) []string {
	set := make(map[string]struct{}, len(existing))
	for _, t := range existing {
		set[t] = struct{}{}
	}
	for _, t := range extra {
		if _, ok := set[t]; !ok {
			existing = append(existing, t)
			set[t] = struct{}{}
		}
	}
	return existing
}

func removeAll(existing, toRemove []string) []string {
	rm := make(map[string]struct{}, len(toRemove))
	for _, t := range toRemove {
		rm[t] = struct{}{}
	}
	out := existing[:0]
	for _, t := range existing {
		if _, ok := rm[t]; !ok {
			out = append(out, t)
		}
	}
	return out
}
