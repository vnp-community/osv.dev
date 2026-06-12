// Package ingestion — apply_impact_analysis command handler.
// Merges impact analysis results back into the vulnerability record.
package apply_impact_analysis

import (
	"context"
	"fmt"

	"github.com/osv/ingestion/internal/domain/aggregate/vulnerability"
	"github.com/osv/ingestion/internal/domain/repository"
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

// VulnRepository provides read-write access to vulnerability aggregates.
type VulnRepository interface {
	GetByID(ctx context.Context, id string) (*vulnerability.VulnerabilityAggregate, error)
	Save(ctx context.Context, v *vulnerability.VulnerabilityAggregate) error
}

// Handler applies computed impact analysis results to a vulnerability record.
type Handler struct {
	repo repository.VulnerabilityRepository
	log  zerolog.Logger
}

// NewHandler creates an ApplyImpactAnalysis handler.
func NewHandler(repo repository.VulnerabilityRepository, log zerolog.Logger) *Handler {
	return &Handler{repo: repo, log: log}
}

// Handle fetches the vulnerability, merges impact ranges, and saves.
func (h *Handler) Handle(ctx context.Context, cmd Command) error {
	vuln, err := h.repo.GetByID(ctx, cmd.VulnID)
	if err != nil {
		return fmt.Errorf("get vuln %s: %w", cmd.VulnID, err)
	}

	for _, r := range cmd.Ranges {
		affRange := vulnerability.AffectedRange{
			RepoURL:  r.RepoURL,
			Type:     r.Type,
			Versions: r.Versions,
		}
		for _, e := range r.Events {
			affRange.Events = append(affRange.Events, vulnerability.RangeEvent{
				Introduced: e.Introduced,
				Fixed:      e.Fixed,
				Limit:      e.Limit,
			})
		}
		vuln.ApplyImpactRange(affRange)
	}

	if err := h.repo.Save(ctx, vuln); err != nil {
		return fmt.Errorf("save vuln %s: %w", cmd.VulnID, err)
	}

	h.log.Info().Str("vuln_id", cmd.VulnID).Int("ranges", len(cmd.Ranges)).Msg("impact analysis applied")
	return nil
}
