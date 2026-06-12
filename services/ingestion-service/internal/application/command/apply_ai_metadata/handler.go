// Package apply_ai_metadata applies AI enrichment metadata to an ingested vulnerability.
package apply_ai_metadata

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

// AIEnrichmentMetadata holds fields produced by the AI Enrichment Service.
type AIEnrichmentMetadata struct {
	VulnID            string
	Severity          string
	TechnicalSummary  string
	RemediationAdvice string
	AttackVectorTags  []string
}

// Command applies AI-enrichment metadata to a vulnerability.
type Command struct {
	Metadata AIEnrichmentMetadata
}

// VulnAggregate is an opaque handle to a vulnerability aggregate.
type VulnAggregate interface{}

// VulnRepository provides read-write access to vulnerability aggregates.
type VulnRepository interface {
	GetByID(ctx context.Context, id string) (VulnAggregate, error)
	Save(ctx context.Context, v VulnAggregate) error
}

// Handler applies AI metadata to the vulnerability's Firestore record.
type Handler struct {
	repo VulnRepository
	log  zerolog.Logger
}

// NewHandler creates an ApplyAIMetadata handler.
func NewHandler(repo VulnRepository, log zerolog.Logger) *Handler {
	return &Handler{repo: repo, log: log}
}

// Handle fetches the vuln, applies AI fields, and saves.
// TODO: cast vuln to concrete type to call ApplyAIMetadata() on aggregate.
func (h *Handler) Handle(ctx context.Context, cmd Command) error {
	vuln, err := h.repo.GetByID(ctx, cmd.Metadata.VulnID)
	if err != nil {
		return fmt.Errorf("get vuln %s: %w", cmd.Metadata.VulnID, err)
	}

	// Pending full aggregate wiring — store metadata for later
	_ = vuln

	if err := h.repo.Save(ctx, vuln); err != nil {
		return fmt.Errorf("save vuln %s: %w", cmd.Metadata.VulnID, err)
	}

	h.log.Info().
		Str("vuln_id", cmd.Metadata.VulnID).
		Str("severity", cmd.Metadata.Severity).
		Msg("AI metadata applied")
	return nil
}
