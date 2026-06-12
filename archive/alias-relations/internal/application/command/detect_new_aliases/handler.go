// application/command/detect_new_aliases/handler.go
package detectnewaliases

import (
	"context"
	"fmt"

	mergealiasgroup "github.com/osv/alias-relations/internal/application/command/merge_alias_group"
	"github.com/osv/alias-relations/internal/domain/repository"
	"github.com/osv/alias-relations/internal/domain/service"
	"github.com/rs/zerolog"
)

// Handler processes DetectNewAliases commands (AI-powered alias detection).
type Handler struct {
	detector  *service.SimilarityDetector
	merger    *mergealiasgroup.Handler
	groupRepo repository.AliasGroupRepository
	log       zerolog.Logger
}

// NewHandler creates a DetectNewAliases handler.
func NewHandler(
	detector *service.SimilarityDetector,
	merger *mergealiasgroup.Handler,
	groupRepo repository.AliasGroupRepository,
	log zerolog.Logger,
) *Handler {
	return &Handler{
		detector:  detector,
		merger:    merger,
		groupRepo: groupRepo,
		log:       log,
	}
}

// Handle executes a DetectNewAliases command triggered by AIEnrichmentCompleted event.
func (h *Handler) Handle(ctx context.Context, cmd Command) (*Result, error) {
	if cmd.VulnID == "" {
		return nil, fmt.Errorf("vuln_id is required")
	}
	if len(cmd.Embedding) == 0 {
		return &Result{VulnID: cmd.VulnID}, nil // no embedding, skip
	}

	candidates, err := h.detector.FindPotentialAliases(ctx, cmd.VulnID, cmd.Embedding)
	if err != nil {
		return nil, fmt.Errorf("find potential aliases for %s: %w", cmd.VulnID, err)
	}

	result := &Result{
		VulnID:     cmd.VulnID,
		Candidates: candidates,
	}

	if len(candidates) == 0 {
		h.log.Debug().Str("vuln_id", cmd.VulnID).Msg("no AI alias candidates found")
		return result, nil
	}

	h.log.Info().
		Str("vuln_id", cmd.VulnID).
		Strs("candidates", candidates).
		Msg("merging AI-detected alias candidates")

	// Merge candidates into alias group
	mergeResult, err := h.merger.Handle(ctx, mergealiasgroup.Command{
		VulnID:          cmd.VulnID,
		DeclaredAliases: candidates,
	})
	if err != nil {
		return nil, fmt.Errorf("merge AI alias candidates: %w", err)
	}

	result.Merged = true
	_ = mergeResult
	return result, nil
}
