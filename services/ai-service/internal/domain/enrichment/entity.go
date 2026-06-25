// Package service — entity.go
// EnrichmentResult is the unified aggregation entity for all AI enrichment stages.
// Sub-domain types (exploit, mitretagger, threatintel) are reused from existing packages.
//
// ADDITIVE: existing types in enrich_cve/handler.go (enrich_vulnerability.Result)
// remain unchanged. This is a separate domain aggregate for the repository interface.
package service

import (
	"context"
	"time"
)

// EnrichmentResult is the unified output of all enrichment stages for a CVE.
// Sub-domain types (exploit, mitretagger, etc.) are defined in their own files.
// This struct aggregates results for storage and API responses.
type EnrichmentResult struct {
	// Core fields
	CVEID        string    `json:"cve_id"         bson:"cve_id"`
	EnrichedAt   time.Time `json:"enriched_at"    bson:"enriched_at"`
	Provider     string    `json:"provider"       bson:"provider"`       // AI provider that succeeded
	ModelVersion string    `json:"model_version"  bson:"model_version"`

	// AI-generated narrative
	SummaryShort     string `json:"summary_short"     bson:"summary_short"`
	SummaryLong      string `json:"summary_long"      bson:"summary_long"`
	ImpactAnalysis   string `json:"impact_analysis"   bson:"impact_analysis"`
	RemediationGuide string `json:"remediation_guide" bson:"remediation_guide"`
	AttackVector     string `json:"attack_vector"     bson:"attack_vector"`

	// Structured enrichment (pointers allow partial enrichment)
	EPSSScore          *EPSSSnapshot `json:"epss,omitempty"       bson:"epss,omitempty"`
	MITRETags          []MITRETag   `json:"mitre_tags,omitempty" bson:"mitre_tags,omitempty"`
	SeverityML         string        `json:"severity_ml,omitempty" bson:"severity_ml,omitempty"`
	SeverityConfidence float64       `json:"severity_confidence"  bson:"severity_confidence"`
	ExploitAvailable   bool          `json:"exploit_available"    bson:"exploit_available"`

	// Embedding tracking (vectors stored in pgvector or Redis)
	HasEmbedding   bool   `json:"has_embedding"           bson:"has_embedding"`
	EmbeddingModel string `json:"embedding_model,omitempty" bson:"embedding_model,omitempty"`
}

// EPSSSnapshot captures a point-in-time EPSS score.
type EPSSSnapshot struct {
	Score      float64   `json:"score"      bson:"score"`
	Percentile float64   `json:"percentile" bson:"percentile"`
	FetchedAt  time.Time `json:"fetched_at" bson:"fetched_at"`
}

// MITRETag represents a MITRE ATT&CK technique mapping.
// Lightweight version — detailed ATTCKTag lives in mitretagger package.
type MITRETag struct {
	TechniqueID   string  `json:"technique_id"   bson:"technique_id"`
	TechniqueName string  `json:"technique_name" bson:"technique_name"`
	TacticID      string  `json:"tactic_id"      bson:"tactic_id"`
	Confidence    float64 `json:"confidence"     bson:"confidence"`
}

// EnrichmentRepository defines the storage interface for EnrichmentResult.
// Implemented by:
//   - infra/persistence/firestore/ (existing EnrichmentRepo — different method names)
//   - infra/persistence/mongo/ (new — S1-AI-03)
type EnrichmentRepository interface {
	Save(ctx context.Context, result *EnrichmentResult) error
	FindByCVEID(ctx context.Context, id string) (*EnrichmentResult, error)
	FindBatch(ctx context.Context, ids []string) ([]*EnrichmentResult, error)
	// ListUnenriched returns CVE IDs that have no enrichment yet
	ListUnenriched(ctx context.Context, limit int) ([]string, error)
}
