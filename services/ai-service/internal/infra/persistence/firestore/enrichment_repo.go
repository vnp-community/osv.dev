// Package firestore implements EnrichmentRepo backed by Cloud Firestore.
package firestore

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"
	"github.com/osv/ai-service/internal/usecase/enrich_cve"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const collectionEnrichments = "ai_enrichments"

// EnrichmentRepo persists AI enrichment results in Firestore.
type EnrichmentRepo struct {
	client *firestore.Client
	log    zerolog.Logger
}

// NewEnrichmentRepo creates a Firestore-backed enrichment repository.
func NewEnrichmentRepo(client *firestore.Client, log zerolog.Logger) *EnrichmentRepo {
	return &EnrichmentRepo{client: client, log: log}
}

// Save persists an enrichment result. Uses VulnID as document ID (idempotent upsert).
func (r *EnrichmentRepo) Save(ctx context.Context, result *enrich_vulnerability.Result) error {
	doc := map[string]interface{}{
		"vuln_id":              result.VulnID,
		"severity":             result.Severity,
		"technical_summary":    result.TechnicalSummary,
		"remediation_advice":   result.RemediationAdvice,
		"attack_vector_tags":   result.AttackVectorTags,
		"exploitability_score": result.ExploitabilityScore,
		"enriched_at":          result.EnrichedAt,
		// Note: embedding is stored separately in Redis / vector store
		// to avoid Firestore's 1MB document limit.
	}

	_, err := r.client.Collection(collectionEnrichments).Doc(result.VulnID).Set(ctx, doc)
	if err != nil {
		return fmt.Errorf("save enrichment %s: %w", result.VulnID, err)
	}

	r.log.Debug().Str("vuln_id", result.VulnID).Msg("enrichment saved")
	return nil
}

// GetByVulnID retrieves an enrichment result by vulnerability ID.
func (r *EnrichmentRepo) GetByVulnID(ctx context.Context, vulnID string) (*enrich_vulnerability.Result, error) {
	doc, err := r.client.Collection(collectionEnrichments).Doc(vulnID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil // not enriched yet
		}
		return nil, fmt.Errorf("get enrichment %s: %w", vulnID, err)
	}

	data := doc.Data()

	result := &enrich_vulnerability.Result{
		VulnID:            vulnID,
		Severity:          stringField(data, "severity"),
		TechnicalSummary:  stringField(data, "technical_summary"),
		RemediationAdvice: stringField(data, "remediation_advice"),
	}

	if tags, ok := data["attack_vector_tags"].([]interface{}); ok {
		for _, t := range tags {
			if s, ok := t.(string); ok {
				result.AttackVectorTags = append(result.AttackVectorTags, s)
			}
		}
	}

	return result, nil
}

func stringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
