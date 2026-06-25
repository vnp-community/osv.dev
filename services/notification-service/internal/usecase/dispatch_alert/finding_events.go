// Package dispatch — finding_events.go
// Adds SLA, Scan, and EPSS dispatch methods to DispatchUseCase.
//
// ADDITIVE: existing Execute(), deliverEmail(), etc. are unchanged.
// These new methods adapt external event payloads into the standard
// NotificationEvent format and call Execute().
package dispatch

import (
	"context"
	"fmt"

	"github.com/osv/notification-service/internal/domain/rule"
)

// ── Request types for new dispatch methods ───────────────────────────────────

// SLABreachRequest carries finding SLA breach data.
type SLABreachRequest struct {
	FindingID   string
	ProductID   string
	Severity    string
	DaysOverdue int
}

// SLADueSoonRequest carries finding SLA expiring-soon data.
type SLADueSoonRequest struct {
	FindingID     string
	DaysRemaining int
}

// ScanCompletedRequest carries scan completion data.
type ScanCompletedRequest struct {
	ScanID       string
	FindingCount int
	ProductID    string
}

// ScanFailedRequest carries scan failure data.
type ScanFailedRequest struct {
	ScanID string
	Reason string
}

// EPSSSpikeRequest carries EPSS score spike data.
type EPSSSpikeRequest struct {
	CVEID      string
	OldScore   float64
	NewScore   float64
	Percentile float64
}

// ── New dispatch methods ──────────────────────────────────────────────────────

// DispatchSLABreach notifies configured channels when a finding SLA is breached.
func (uc *DispatchUseCase) DispatchSLABreach(ctx context.Context, req SLABreachRequest) error {
	pid := req.ProductID
	return uc.Execute(ctx, &NotificationEvent{
		Type:      rule.EventSLABreach,
		ProductID: &pid,
		Title:     fmt.Sprintf("SLA Breach: %s finding (overdue %d days)", req.Severity, req.DaysOverdue),
		Description: fmt.Sprintf(
			"Finding %s has breached its SLA. Severity: %s. Days overdue: %d.",
			req.FindingID, req.Severity, req.DaysOverdue,
		),
		Metadata: map[string]interface{}{
			"finding_id":   req.FindingID,
			"product_id":   req.ProductID,
			"severity":     req.Severity,
			"days_overdue": req.DaysOverdue,
		},
	})
}

// DispatchSLADueSoon notifies configured channels when a finding SLA is expiring soon.
func (uc *DispatchUseCase) DispatchSLADueSoon(ctx context.Context, req SLADueSoonRequest) error {
	return uc.Execute(ctx, &NotificationEvent{
		Type:  rule.EventSLAExpiringSoon,
		Title: fmt.Sprintf("SLA Expiring Soon: %d days remaining", req.DaysRemaining),
		Description: fmt.Sprintf(
			"Finding %s SLA expires in %d days. Please take action.",
			req.FindingID, req.DaysRemaining,
		),
		Metadata: map[string]interface{}{
			"finding_id":     req.FindingID,
			"days_remaining": req.DaysRemaining,
		},
	})
}

// DispatchScanCompleted notifies configured channels when a scan job completes.
func (uc *DispatchUseCase) DispatchScanCompleted(ctx context.Context, req ScanCompletedRequest) error {
	pid := req.ProductID
	return uc.Execute(ctx, &NotificationEvent{
		Type:      rule.EventScanAdded,
		ProductID: &pid,
		Title:     fmt.Sprintf("Scan Completed: %d findings", req.FindingCount),
		Description: fmt.Sprintf(
			"Scan %s completed with %d findings for product %s.",
			req.ScanID, req.FindingCount, req.ProductID,
		),
		Metadata: map[string]interface{}{
			"scan_id":       req.ScanID,
			"finding_count": req.FindingCount,
			"product_id":    req.ProductID,
		},
	})
}

// DispatchScanFailed notifies configured channels when a scan job fails.
func (uc *DispatchUseCase) DispatchScanFailed(ctx context.Context, req ScanFailedRequest) error {
	return uc.Execute(ctx, &NotificationEvent{
		Type:        rule.EventScanAdded,
		Title:       fmt.Sprintf("Scan Failed: %s", req.ScanID),
		Description: fmt.Sprintf("Scan %s failed. Reason: %s", req.ScanID, req.Reason),
		Metadata: map[string]interface{}{
			"scan_id": req.ScanID,
			"reason":  req.Reason,
		},
	})
}

// DispatchEPSSSpike notifies configured channels when an EPSS score crosses 0.7.
// This is called only when the new score exceeds the configured threshold.
func (uc *DispatchUseCase) DispatchEPSSSpike(ctx context.Context, req EPSSSpikeRequest) error {
	return uc.Execute(ctx, &NotificationEvent{
		Type:  rule.EventFindingStatusChanged,
		Title: fmt.Sprintf("High EPSS Score: %s (%.2f%%)", req.CVEID, req.NewScore*100),
		Description: fmt.Sprintf(
			"CVE %s EPSS score rose from %.3f to %.3f (%.1f%% percentile). High exploitation likelihood.",
			req.CVEID, req.OldScore, req.NewScore, req.Percentile*100,
		),
		Metadata: map[string]interface{}{
			"cve_id":     req.CVEID,
			"old_score":  req.OldScore,
			"new_score":  req.NewScore,
			"percentile": req.Percentile,
		},
	})
}
