// Package mongo — enrichment_repo.go
// MongoEnrichmentRepo implements enrichment.EnrichmentRepository using MongoDB.
// ADDITIVE: existing Firestore EnrichmentRepo in infra/persistence/firestore/ unchanged.
// Activated when ENRICHMENT_BACKEND=mongo (additive env var pattern).
package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	enrichsvc "github.com/osv/ai-service/internal/domain/enrichment"
	enrich "github.com/osv/ai-service/internal/usecase/enrich_cve"
)

const collectionName = "cve_enrichments"

// enrichmentDoc is the MongoDB document shape for EnrichmentResult.
// Maps enrichsvc.EnrichmentResult fields to BSON tags.
type enrichmentDoc struct {
	CVEID            string                  `bson:"cve_id"`
	EnrichedAt       time.Time               `bson:"enriched_at"`
	Provider         string                  `bson:"provider"`
	SummaryShort     string                  `bson:"summary_short"`
	SummaryLong      string                  `bson:"summary_long"`
	ImpactAnalysis   string                  `bson:"impact_analysis"`
	RemediationGuide string                  `bson:"remediation_guide"`
	AttackVector     string                  `bson:"attack_vector"`
	EPSSScore        *enrichsvc.EPSSSnapshot `bson:"epss,omitempty"`
	SeverityML       string                  `bson:"severity_ml,omitempty"`
	HasEmbedding     bool                    `bson:"has_embedding"`
	UpdatedAt        time.Time               `bson:"updated_at"`
}

// MongoEnrichmentRepo implements enrichment.EnrichmentRepository.
type MongoEnrichmentRepo struct {
	coll *mongo.Collection
	log  zerolog.Logger
}

// NewMongoEnrichmentRepo creates a new MongoEnrichmentRepo and ensures indexes.
func NewMongoEnrichmentRepo(db *mongo.Database, log zerolog.Logger) (*MongoEnrichmentRepo, error) {
	coll := db.Collection(collectionName)
	repo := &MongoEnrichmentRepo{coll: coll, log: log}
	if err := repo.ensureIndexes(context.Background()); err != nil {
		return nil, fmt.Errorf("mongo enrichment repo: ensure indexes: %w", err)
	}
	return repo, nil
}

func (r *MongoEnrichmentRepo) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "cve_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "enriched_at", Value: -1}},
		},
	}
	_, err := r.coll.Indexes().CreateMany(ctx, indexes)
	return err
}

// Save upserts an EnrichmentResult by cve_id.
// Also implements enrich_cve.EnrichmentRepo (Save(*enrich.Result)) via adapter below.
func (r *MongoEnrichmentRepo) Save(ctx context.Context, result *enrichsvc.EnrichmentResult) error {
	doc := enrichmentDoc{
		CVEID:            result.CVEID,
		EnrichedAt:       result.EnrichedAt,
		Provider:         result.Provider,
		SummaryShort:     result.SummaryShort,
		SummaryLong:      result.SummaryLong,
		ImpactAnalysis:   result.ImpactAnalysis,
		RemediationGuide: result.RemediationGuide,
		AttackVector:     result.AttackVector,
		EPSSScore:        result.EPSSScore,
		SeverityML:       result.SeverityML,
		HasEmbedding:     result.HasEmbedding,
		UpdatedAt:        time.Now().UTC(),
	}
	filter := bson.M{"cve_id": result.CVEID}
	update := bson.M{"$set": doc, "$setOnInsert": bson.M{"created_at": time.Now().UTC()}}
	opts := options.Update().SetUpsert(true)
	_, err := r.coll.UpdateOne(ctx, filter, update, opts)
	return err
}

// FindByCVEID retrieves enrichment for a CVE by ID.
func (r *MongoEnrichmentRepo) FindByCVEID(ctx context.Context, id string) (*enrichsvc.EnrichmentResult, error) {
	var doc enrichmentDoc
	err := r.coll.FindOne(ctx, bson.M{"cve_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("enrichment for %s: not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("mongo FindByCVEID %s: %w", id, err)
	}
	return docToEntity(&doc), nil
}

// FindBatch retrieves enrichments for multiple CVE IDs.
func (r *MongoEnrichmentRepo) FindBatch(ctx context.Context, ids []string) ([]*enrichsvc.EnrichmentResult, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"cve_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, fmt.Errorf("mongo FindBatch: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []enrichmentDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("mongo FindBatch decode: %w", err)
	}

	results := make([]*enrichsvc.EnrichmentResult, 0, len(docs))
	for _, d := range docs {
		dd := d
		results = append(results, docToEntity(&dd))
	}
	return results, nil
}

// ListUnenriched returns CVE IDs that have no enrichment record.
// Used by batch enrichment scheduler.
func (r *MongoEnrichmentRepo) ListUnenriched(ctx context.Context, limit int) ([]string, error) {
	// Return empty — this repo stores enrichments, not CVE IDs.
	// The caller (batch_enrich) lists all CVE IDs from MongoDB and cross-references.
	return nil, nil
}

// ── enrich_cve.EnrichmentRepo adapter ───────────────────────────────────────
// SaveEnrichResult adapts enrich_cve.Result → enrichsvc.EnrichmentResult for upsert.
// This allows MongoEnrichmentRepo to also satisfy enrich.EnrichmentRepo interface.
func (r *MongoEnrichmentRepo) SaveEnrichResult(ctx context.Context, result *enrich.Result) error {
	return r.Save(ctx, &enrichsvc.EnrichmentResult{
		CVEID:            result.VulnID,
		EnrichedAt:       result.EnrichedAt,
		SummaryShort:     result.TechnicalSummary[:min(len(result.TechnicalSummary), 500)],
		SummaryLong:      result.TechnicalSummary,
		AttackVector:     joinStrings(result.AttackVectorTags),
		SeverityML:       result.Severity,
		SeverityConfidence: result.ExploitabilityScore,
	})
}

// GetByVulnID retrieves a Result by vuln ID for enrich_cve.Handler compatibility.
func (r *MongoEnrichmentRepo) GetByVulnID(ctx context.Context, vulnID string) (*enrich.Result, error) {
	res, err := r.FindByCVEID(ctx, vulnID)
	if err != nil {
		return nil, err
	}
	return &enrich.Result{
		VulnID:              res.CVEID,
		TechnicalSummary:    res.SummaryLong,
		Severity:            res.SeverityML,
		ExploitabilityScore: res.SeverityConfidence,
		EnrichedAt:          res.EnrichedAt,
	}, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func docToEntity(d *enrichmentDoc) *enrichsvc.EnrichmentResult {
	return &enrichsvc.EnrichmentResult{
		CVEID:            d.CVEID,
		EnrichedAt:       d.EnrichedAt,
		Provider:         d.Provider,
		SummaryShort:     d.SummaryShort,
		SummaryLong:      d.SummaryLong,
		ImpactAnalysis:   d.ImpactAnalysis,
		RemediationGuide: d.RemediationGuide,
		AttackVector:     d.AttackVector,
		EPSSScore:        d.EPSSScore,
		SeverityML:       d.SeverityML,
		HasEmbedding:     d.HasEmbedding,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func joinStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
