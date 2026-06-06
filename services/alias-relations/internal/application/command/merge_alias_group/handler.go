// application/command/merge_alias_group/handler.go
package mergealiasgroup

import (
	"context"
	"fmt"

	"github.com/osv/alias-relations/internal/domain/repository"
	"github.com/osv/alias-relations/internal/domain/service"
	"github.com/rs/zerolog"
)

// EventPublisher publishes domain events after alias merge.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload interface{}) error
}

// Handler processes MergeAliasGroup commands.
type Handler struct {
	merger    *service.AliasMerger
	groupRepo repository.AliasGroupRepository
	publisher EventPublisher
	log       zerolog.Logger
}

// NewHandler creates a MergeAliasGroup handler.
func NewHandler(
	merger *service.AliasMerger,
	groupRepo repository.AliasGroupRepository,
	publisher EventPublisher,
	log zerolog.Logger,
) *Handler {
	return &Handler{
		merger:    merger,
		groupRepo: groupRepo,
		publisher: publisher,
		log:       log,
	}
}

// Handle executes a MergeAliasGroup command.
func (h *Handler) Handle(ctx context.Context, cmd Command) (*Result, error) {
	if cmd.VulnID == "" {
		return nil, fmt.Errorf("vuln_id is required")
	}

	if err := h.merger.ProcessVulnerability(ctx, cmd.VulnID, cmd.DeclaredAliases); err != nil {
		return nil, fmt.Errorf("process vulnerability aliases: %w", err)
	}

	// Retrieve the merged group for the result
	group, err := h.groupRepo.GetByMemberID(ctx, cmd.VulnID)
	if err != nil {
		return nil, fmt.Errorf("retrieve merged group: %w", err)
	}
	if group == nil {
		return nil, fmt.Errorf("merged group not found after save")
	}

	// Publish domain events
	for _, evt := range group.Events() {
		if err := h.publisher.Publish(ctx, "osv.alias.group.updated", evt); err != nil {
			h.log.Warn().Err(err).Msg("failed to publish AliasGroupUpdated event")
		}
	}

	return &Result{
		GroupID:     group.ID(),
		CanonicalID: group.CanonicalID(),
		AllIDs:      group.BugIDs(),
	}, nil
}
