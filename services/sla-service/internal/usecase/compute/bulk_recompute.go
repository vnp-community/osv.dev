// Package compute provides SLA bulk recomputation for all findings in a product.
package compute

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// ─── Interfaces ────────────────────────────────────────────────────────────────

// FindingStreamClient streams active findings for SLA computation.
type FindingStreamClient interface {
	// ListFindingsForSLACheck streams findings matching the request.
	// Returns an iterator; call Next() until io.EOF.
	ListFindingsForSLACheck(ctx context.Context, req *SLACheckRequest) (FindingStream, error)
	// BatchUpdateSLADates pushes computed SLA dates back to finding-service.
	BatchUpdateSLADates(ctx context.Context, updates []BatchSLAUpdate) error
}

// FindingStream is a server-streaming iterator.
type FindingStream interface {
	Next() (*FindingForSLA, error) // returns io.EOF when done
}

// SLACheckRequest filters findings for SLA computation.
type SLACheckRequest struct {
	ProductID  *uuid.UUID
	ActiveOnly bool
}

// FindingForSLA is the minimal finding data for SLA computation.
type FindingForSLA struct {
	ID        uuid.UUID
	Severity  string
	FoundDate time.Time
}

// BatchSLAUpdate is a single expiry date update pushed to finding-service.
type BatchSLAUpdate struct {
	FindingID         uuid.UUID
	SLAExpirationDate time.Time
}

// SLAConfigProvider resolves the SLA config for a product.
type SLAConfigProvider interface {
	// DaysForSeverity returns the configured SLA days for a severity.
	DaysForSeverity(severity string) int
}

// ─── BulkRecomputeUseCase ─────────────────────────────────────────────────────

// BulkRecomputeUseCase recomputes SLA dates for all active findings in a product.
// Triggered when: SLA config assigned/changed, or by admin request.
type BulkRecomputeUseCase struct {
	findingClient FindingStreamClient
}

// NewBulkRecompute creates a new BulkRecomputeUseCase.
func NewBulkRecompute(fc FindingStreamClient) *BulkRecomputeUseCase {
	return &BulkRecomputeUseCase{findingClient: fc}
}

// Execute recomputes SLA expiration dates for all active findings in a product.
// Uses batch size of 500 findings per gRPC call to avoid timeouts.
func (uc *BulkRecomputeUseCase) Execute(ctx context.Context, productID uuid.UUID, cfgProvider SLAConfigProvider) error {
	slog.InfoContext(ctx, "starting SLA bulk recompute", "product_id", productID)

	stream, err := uc.findingClient.ListFindingsForSLACheck(ctx, &SLACheckRequest{
		ProductID:  &productID,
		ActiveOnly: true,
	})
	if err != nil {
		return err
	}

	const batchSize = 500
	batchUpdates := make([]BatchSLAUpdate, 0, batchSize)
	processed := 0
	failed := 0

	for {
		finding, err := stream.Next()
		if err != nil {
			break // io.EOF or stream error
		}

		days := cfgProvider.DaysForSeverity(finding.Severity)
		if days == 0 {
			continue // Info severity: no SLA
		}

		expiry := finding.FoundDate.UTC().Truncate(24 * time.Hour).AddDate(0, 0, days)
		batchUpdates = append(batchUpdates, BatchSLAUpdate{
			FindingID:         finding.ID,
			SLAExpirationDate: expiry,
		})
		processed++

		// Flush batch every 500 findings
		if len(batchUpdates) >= batchSize {
			if err := uc.findingClient.BatchUpdateSLADates(ctx, batchUpdates); err != nil {
				slog.ErrorContext(ctx, "batch SLA update failed", "error", err)
				failed += len(batchUpdates)
			}
			batchUpdates = batchUpdates[:0]
		}
	}

	// Flush remaining
	if len(batchUpdates) > 0 {
		if err := uc.findingClient.BatchUpdateSLADates(ctx, batchUpdates); err != nil {
			failed += len(batchUpdates)
		}
	}

	slog.InfoContext(ctx, "SLA bulk recompute completed",
		"product_id", productID,
		"processed", processed,
		"failed", failed,
	)
	return nil
}
