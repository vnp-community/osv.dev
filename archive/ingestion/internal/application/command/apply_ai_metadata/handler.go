// Package apply_ai_metadata applies AI enrichment metadata to an ingested vulnerability.
package apply_ai_metadata

import (
	"context"
	"fmt"

	"github.com/osv/ingestion/internal/domain/repository"
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

// Handler applies AI metadata to the vulnerability's Firestore record.
type Handler struct {
	repo repository.VulnerabilityRepository
	log  zerolog.Logger
}

// NewHandler creates an ApplyAIMetadata handler.
func NewHandler(repo repository.VulnerabilityRepository, log zerolog.Logger) *Handler {
	return &Handler{repo: repo, log: log}
}

// Handle fetches the vuln, applies AI fields, and saves.
func (h *Handler) Handle(ctx context.Context, cmd Command) error {
	vuln, err := h.repo.GetByID(ctx, cmd.Metadata.VulnID)
	if err != nil {
		return fmt.Errorf("get vuln %s: %w", cmd.Metadata.VulnID, err)
	}

	vuln.ApplyAIMetadata(
		cmd.Metadata.Severity,
		cmd.Metadata.TechnicalSummary,
		cmd.Metadata.RemediationAdvice,
		cmd.Metadata.AttackVectorTags,
	)

	if err := h.repo.Save(ctx, vuln); err != nil {
		return fmt.Errorf("save vuln %s: %w", cmd.Metadata.VulnID, err)
	}

	h.log.Info().Str("vuln_id", cmd.Metadata.VulnID).Str("severity", cmd.Metadata.Severity).Msg("AI metadata applied")
	return nil
}
