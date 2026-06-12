// domain/service/similarity_detector.go
package service

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

// AIEnrichmentClient is the port for communicating with the AI Enrichment service.
type AIEnrichmentClient interface {
	// FindSimilar returns vuln IDs with embedding similarity >= minScore.
	FindSimilar(ctx context.Context, req *SimilaritySearchRequest) ([]*SimilarResult, error)
}

// SimilaritySearchRequest is sent to the AI Enrichment service.
type SimilaritySearchRequest struct {
	Embedding []float32
	TopK      int32
	MinScore  float32
	ExcludeID string
}

// SimilarResult represents a single similarity match.
type SimilarResult struct {
	VulnID string
	Score  float32
}

// SimilarityDetector finds potential aliases using embedding cosine similarity.
// Secondary method — supplements declared aliases[] with AI-detected matches.
type SimilarityDetector struct {
	aiClient  AIEnrichmentClient
	threshold float32
	log       zerolog.Logger
}

// DefaultSimilarityThreshold is the cosine similarity cutoff for alias detection.
const DefaultSimilarityThreshold = float32(0.95)

// NewSimilarityDetector creates a SimilarityDetector.
func NewSimilarityDetector(client AIEnrichmentClient, log zerolog.Logger) *SimilarityDetector {
	return &SimilarityDetector{
		aiClient:  client,
		threshold: DefaultSimilarityThreshold,
		log:       log,
	}
}

// WithThreshold sets a custom similarity threshold.
func (d *SimilarityDetector) WithThreshold(t float32) *SimilarityDetector {
	d.threshold = t
	return d
}

// FindPotentialAliases returns vuln IDs that have high embedding similarity to the given vuln.
// Returns IDs with cosine similarity >= threshold (default 0.95).
func (d *SimilarityDetector) FindPotentialAliases(ctx context.Context, vulnID string, embedding []float32) ([]string, error) {
	if len(embedding) == 0 {
		return nil, fmt.Errorf("embedding is empty for vuln %s", vulnID)
	}

	d.log.Debug().
		Str("vuln_id", vulnID).
		Float32("threshold", d.threshold).
		Msg("searching for similar vulnerabilities")

	similar, err := d.aiClient.FindSimilar(ctx, &SimilaritySearchRequest{
		Embedding: embedding,
		TopK:      20,
		MinScore:  d.threshold,
		ExcludeID: vulnID,
	})
	if err != nil {
		return nil, fmt.Errorf("find similar for %s: %w", vulnID, err)
	}

	ids := make([]string, 0, len(similar))
	for _, r := range similar {
		if r.Score >= d.threshold {
			ids = append(ids, r.VulnID)
			d.log.Debug().
				Str("candidate", r.VulnID).
				Float32("score", r.Score).
				Msg("potential alias found")
		}
	}

	return ids, nil
}
