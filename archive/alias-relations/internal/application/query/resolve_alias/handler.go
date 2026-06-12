// application/query/resolve_alias/handler.go
package resolvealias

import (
	"context"
	"fmt"

	"github.com/osv/alias-relations/internal/domain/repository"
)

// Handler resolves any vuln ID to its canonical alias group.
type Handler struct {
	groupRepo repository.AliasGroupRepository
}

// NewHandler creates a ResolveAlias handler.
func NewHandler(repo repository.AliasGroupRepository) *Handler {
	return &Handler{groupRepo: repo}
}

// Handle looks up the alias group for the given vuln ID.
func (h *Handler) Handle(ctx context.Context, q Query) (*Result, error) {
	if q.VulnID == "" {
		return nil, fmt.Errorf("vuln_id is required")
	}

	group, err := h.groupRepo.GetByMemberID(ctx, q.VulnID)
	if err != nil {
		return nil, fmt.Errorf("lookup alias group for %s: %w", q.VulnID, err)
	}

	if group == nil {
		// Not found — return single-member result
		return &Result{
			CanonicalID: q.VulnID,
			AllIDs:      []string{q.VulnID},
			Found:       false,
		}, nil
	}

	return &Result{
		CanonicalID:  group.CanonicalID(),
		AllIDs:       group.BugIDs(),
		GroupID:      group.ID(),
		LastModified: group.LastModified().Format("2006-01-02T15:04:05Z"),
		Found:        true,
	}, nil
}
