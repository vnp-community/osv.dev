// Package sync provides the JIRA → Finding status synchronization use case.
package sync

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
)

// ─── Status mapping ───────────────────────────────────────────────────────────

// jiraStatusToAction maps normalized JIRA status names to finding actions.
// JIRA statuses are lowercase-matched.
var jiraStatusToAction = map[string]string{
	"done":        "close",
	"resolved":    "close",
	"closed":      "close",
	"won't fix":   "close",
	"wont fix":    "close",
	"in progress": "reopen",
	"open":        "reopen",
	"reopened":    "reopen",
	"to do":       "reopen",
}

// ─── Interfaces ────────────────────────────────────────────────────────────────

// MappingRepository provides finding ↔ JIRA issue lookups.
type MappingRepository interface {
	FindByJIRAKey(ctx context.Context, jiraKey string) (*JIRAIssueMapping, error)
	UpdateStatus(ctx context.Context, jiraKey, status string) error
}

// JIRAIssueMapping is the stored link between a finding and a JIRA issue.
type JIRAIssueMapping struct {
	FindingID  uuid.UUID
	JIRAKey    string
	JIRAStatus string
}

// FindingServiceClient provides finding state operations via gRPC.
type FindingServiceClient interface {
	CloseFinding(ctx context.Context, findingID uuid.UUID) error
	ReopenFinding(ctx context.Context, findingID uuid.UUID) error
	AddFindingNote(ctx context.Context, findingID uuid.UUID, author, content string) error
}

// EventPublisher publishes NATS events.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload map[string]any) error
}

// ─── PullStatusUseCase ────────────────────────────────────────────────────────

// PullStatusInput holds data from the JIRA webhook for a status change.
type PullStatusInput struct {
	JIRAKey       string
	JIRAID        string
	NewJIRAStatus string
}

// PullStatusUseCase syncs JIRA issue status back to finding-service.
type PullStatusUseCase struct {
	mappingRepo   MappingRepository
	findingClient FindingServiceClient
	eventPub      EventPublisher
}

// New creates a new PullStatusUseCase.
func New(mr MappingRepository, fc FindingServiceClient, ep EventPublisher) *PullStatusUseCase {
	return &PullStatusUseCase{mappingRepo: mr, findingClient: fc, eventPub: ep}
}

// Execute syncs the JIRA status change to the corresponding finding.
// No-ops if: no mapping found, status unchanged, or unknown JIRA status.
func (uc *PullStatusUseCase) Execute(ctx context.Context, in *PullStatusInput) error {
	// 1. Find mapping by JIRA key
	mapping, err := uc.mappingRepo.FindByJIRAKey(ctx, in.JIRAKey)
	if err != nil {
		return nil // no mapping — ignore (JIRA issue not linked to a finding)
	}
	if mapping == nil {
		slog.DebugContext(ctx, "jira webhook: no mapping found", "jira_key", in.JIRAKey)
		return nil
	}

	// 2. Update stored JIRA status
	if err := uc.mappingRepo.UpdateStatus(ctx, in.JIRAKey, in.NewJIRAStatus); err != nil {
		slog.WarnContext(ctx, "failed to update JIRA status in mapping", "error", err)
	}

	// 3. Determine action from status
	action := jiraStatusToAction[strings.ToLower(in.NewJIRAStatus)]
	if action == "" {
		slog.DebugContext(ctx, "jira webhook: unknown status, no action",
			"jira_key", in.JIRAKey, "status", in.NewJIRAStatus)
		return nil
	}

	// 4. Apply action to finding via finding-service gRPC
	switch action {
	case "close":
		if err := uc.findingClient.CloseFinding(ctx, mapping.FindingID); err != nil {
			return fmt.Errorf("closing finding %s: %w", mapping.FindingID, err)
		}
		slog.InfoContext(ctx, "finding closed via JIRA webhook",
			"finding_id", mapping.FindingID, "jira_key", in.JIRAKey)
	case "reopen":
		if err := uc.findingClient.ReopenFinding(ctx, mapping.FindingID); err != nil {
			return fmt.Errorf("reopening finding %s: %w", mapping.FindingID, err)
		}
		slog.InfoContext(ctx, "finding reopened via JIRA webhook",
			"finding_id", mapping.FindingID, "jira_key", in.JIRAKey)
	}

	// 5. Publish NATS sync event
	_ = uc.eventPub.Publish(ctx, "defectdojo.jira.synced", map[string]any{
		"finding_id": mapping.FindingID.String(),
		"jira_key":   in.JIRAKey,
		"jira_status": in.NewJIRAStatus,
		"action":     action,
		"_service":   "jira-service",
	})
	return nil
}

// SyncComment syncs a JIRA comment to a finding note with "[JIRA]" prefix.
func (uc *PullStatusUseCase) SyncComment(ctx context.Context, jiraKey string, commentBody string) {
	mapping, _ := uc.mappingRepo.FindByJIRAKey(ctx, jiraKey)
	if mapping == nil {
		return
	}
	content := fmt.Sprintf("[JIRA] %s", commentBody)
	if err := uc.findingClient.AddFindingNote(ctx, mapping.FindingID, "system", content); err != nil {
		slog.WarnContext(ctx, "failed to sync JIRA comment to finding note",
			"finding_id", mapping.FindingID, "error", err)
	}
}
