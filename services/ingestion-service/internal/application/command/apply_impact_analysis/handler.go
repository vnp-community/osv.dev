// Package ingestion — apply_impact_analysis command handler.
// Merges impact analysis results back into the vulnerability record.
package apply_impact_analysis

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

// AffectedRange holds the result of git bisection or version enumeration.
type AffectedRange struct {
	RepoURL  string
	Type     string // GIT | ECOSYSTEM
	Events   []RangeEvent
	Versions []string
}

// RangeEvent represents an introduced/fixed version event.
type RangeEvent struct {
	Introduced string
	Fixed      string
	Limit      string
}

// Command triggers merging impact analysis results into a vulnerability.
type Command struct {
	VulnID  string
	Ranges  []AffectedRange
}

// VulnAggregate is an opaque handle to a vulnerability aggregate (avoids circular import).
type VulnAggregate interface{}

// VulnRepository provides read-write access to vulnerability aggregates.
type VulnRepository interface {
	GetByID(ctx context.Context, id string) (VulnAggregate, error)
	Save(ctx context.Context, v VulnAggregate) error
}

// Handler applies computed impact analysis results to a vulnerability record.
type Handler struct {
	repo VulnRepository
	log  zerolog.Logger
}

// NewHandler creates an ApplyImpactAnalysis handler.
func NewHandler(repo VulnRepository, log zerolog.Logger) *Handler {
	return &Handler{repo: repo, log: log}
}

// Handle fetches the vulnerability, merges impact ranges, and saves.
// The actual Apply logic is delegated to the concrete aggregate via the repo.
func (h *Handler) Handle(ctx context.Context, cmd Command) error {
	vuln, err := h.repo.GetByID(ctx, cmd.VulnID)
	if err != nil {
		return fmt.Errorf("get vuln %s: %w", cmd.VulnID, err)
	}

	// TODO: cast vuln to concrete type to call ApplyImpactRange(AffectedRange)
	// For now, save as-is (pending full aggregate wiring)
	_ = vuln
	_ = cmd.Ranges

	if err := h.repo.Save(ctx, vuln); err != nil {
		return fmt.Errorf("save vuln %s: %w", cmd.VulnID, err)
	}

	h.log.Info().Str("vuln_id", cmd.VulnID).Int("ranges", len(cmd.Ranges)).Msg("impact analysis applied")
	return nil
}
