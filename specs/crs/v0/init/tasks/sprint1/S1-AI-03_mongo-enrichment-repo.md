# S1-AI-03 — Thêm MongoDB Enrichment Repo (ai-service)

## ✅ Execution Status: COMPLETED
- **Executed**: 2026-06-13
- **Result**: `go build` PASSED


## Metadata
- **Task ID**: S1-AI-03
- **Service**: ai-service
- **Sprint**: 1 (P0)
- **Ước tính**: 2 giờ
- **Dependencies**: S1-AI-01 (EnrichmentResult entity)
- **Spec nguồn**: `specs/develop/06_ai-service-upgrade.md` § "P0 — Thêm: MongoDB Enrichment Repo"

## Context

```bash
# Đọc Firestore implementation để hiểu interface:
cat services/ai-service/internal/infra/persistence/firestore/enrichment_repo.go

# Đọc MongoDB pattern từ identity-service:
cat services/identity-service/internal/adapter/repository/mongo/user_repo.go

# Đọc go.mod để confirm mongo driver:
cat services/ai-service/go.mod | grep mongo
```

## Files to Create

### File 1: `services/ai-service/internal/infra/persistence/mongo/enrichment_repo.go`

```go
package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"github.com/rs/zerolog"

	"github.com/osv/ai-service/internal/domain/enrichment"
)

const (
	collectionName = "cve_enrichments"
)

// enrichmentDocument is the MongoDB BSON representation.
type enrichmentDocument struct {
	ID           primitive.ObjectID         `bson:"_id,omitempty"`
	CVEID        string                     `bson:"cve_id"`
	EnrichedAt   time.Time                  `bson:"enriched_at"`
	Provider     string                     `bson:"provider"`
	ModelVersion string                     `bson:"model_version"`
	SummaryShort string                     `bson:"summary_short"`
	SummaryLong  string                     `bson:"summary_long"`
	ImpactAnalysis string                   `bson:"impact_analysis"`
	Remediation  string                     `bson:"remediation_guide"`
	AttackVector string                     `bson:"attack_vector"`
	EPSS         *enrichment.EPSSSnapshot   `bson:"epss,omitempty"`
	SeverityML   string                     `bson:"severity_ml,omitempty"`
	HasEmbedding bool                       `bson:"has_embedding"`
	UpdatedAt    time.Time                  `bson:"updated_at"`
}

// MongoEnrichmentRepo implements enrichment.EnrichmentRepository using MongoDB.
type MongoEnrichmentRepo struct {
	coll *mongo.Collection
	log  zerolog.Logger
}

// NewEnrichmentRepo creates a new MongoEnrichmentRepo.
// Ensures required indexes on first call.
func NewEnrichmentRepo(db *mongo.Database, log zerolog.Logger) (*MongoEnrichmentRepo, error) {
	coll := db.Collection(collectionName)

	repo := &MongoEnrichmentRepo{coll: coll, log: log}
	if err := repo.ensureIndexes(context.Background()); err != nil {
		return nil, err
	}
	return repo, nil
}

// ensureIndexes creates MongoDB indexes if they don't exist.
func (r *MongoEnrichmentRepo) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "cve_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "enriched_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "epss.score", Value: -1}},
		},
	}

	_, err := r.coll.Indexes().CreateMany(ctx, indexes)
	return err
}

// Save creates or updates an enrichment result.
func (r *MongoEnrichmentRepo) Save(ctx context.Context, result *enrichment.EnrichmentResult) error {
	doc := toDocument(result)
	doc.UpdatedAt = time.Now()

	filter := bson.M{"cve_id": result.CVEID}
	update := bson.M{"$set": doc}
	opts := options.Update().SetUpsert(true)

	_, err := r.coll.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		r.log.Error().Err(err).Str("cve_id", result.CVEID).Msg("mongo: save enrichment failed")
	}
	return err
}

// FindByCVEID retrieves enrichment for a specific CVE.
func (r *MongoEnrichmentRepo) FindByCVEID(ctx context.Context, id string) (*enrichment.EnrichmentResult, error) {
	var doc enrichmentDocument
	err := r.coll.FindOne(ctx, bson.M{"cve_id": id}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // not found
		}
		return nil, err
	}
	return fromDocument(&doc), nil
}

// FindBatch retrieves enrichment for multiple CVE IDs.
func (r *MongoEnrichmentRepo) FindBatch(ctx context.Context, ids []string) ([]*enrichment.EnrichmentResult, error) {
	filter := bson.M{"cve_id": bson.M{"$in": ids}}
	opts := options.Find().SetLimit(int64(len(ids)))

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*enrichment.EnrichmentResult
	for cursor.Next(ctx) {
		var doc enrichmentDocument
		if err := cursor.Decode(&doc); err != nil {
			continue // skip malformed docs
		}
		results = append(results, fromDocument(&doc))
	}
	return results, cursor.Err()
}

// ListUnenriched returns CVE IDs that have no enrichment record yet.
// Used for batch enrichment jobs.
func (r *MongoEnrichmentRepo) ListUnenriched(ctx context.Context, limit int) ([]string, error) {
	// This is tricky — we need to know ALL CVE IDs from data-service.
	// For now: return CVEs enriched more than 30 days ago (stale enrichments).
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	filter := bson.M{"enriched_at": bson.M{"$lt": thirtyDaysAgo}}
	opts := options.Find().
		SetLimit(int64(limit)).
		SetProjection(bson.M{"cve_id": 1})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var ids []string
	for cursor.Next(ctx) {
		var doc struct {
			CVEID string `bson:"cve_id"`
		}
		if err := cursor.Decode(&doc); err == nil {
			ids = append(ids, doc.CVEID)
		}
	}
	return ids, cursor.Err()
}

// toDocument converts domain entity to BSON document.
func toDocument(e *enrichment.EnrichmentResult) *enrichmentDocument {
	return &enrichmentDocument{
		CVEID:          e.CVEID,
		EnrichedAt:     e.EnrichedAt,
		Provider:       e.Provider,
		ModelVersion:   e.ModelVersion,
		SummaryShort:   e.SummaryShort,
		SummaryLong:    e.SummaryLong,
		ImpactAnalysis: e.ImpactAnalysis,
		Remediation:    e.RemediationGuide,
		AttackVector:   e.AttackVector,
		EPSS:           e.EPSSScore,
		SeverityML:     e.SeverityML,
		HasEmbedding:   e.HasEmbedding,
	}
}

// fromDocument converts BSON document to domain entity.
func fromDocument(doc *enrichmentDocument) *enrichment.EnrichmentResult {
	return &enrichment.EnrichmentResult{
		CVEID:            doc.CVEID,
		EnrichedAt:       doc.EnrichedAt,
		Provider:         doc.Provider,
		ModelVersion:     doc.ModelVersion,
		SummaryShort:     doc.SummaryShort,
		SummaryLong:      doc.SummaryLong,
		ImpactAnalysis:   doc.ImpactAnalysis,
		RemediationGuide: doc.Remediation,
		AttackVector:     doc.AttackVector,
		EPSSScore:        doc.EPSS,
		SeverityML:       doc.SeverityML,
		HasEmbedding:     doc.HasEmbedding,
	}
}
```

### File 2: `services/ai-service/internal/config/storage_config.go`

```go
package config

// EnrichmentBackend defines the storage backend for enrichment results.
type EnrichmentBackend string

const (
	// EnrichmentFirestore uses Google Firestore (current default).
	EnrichmentFirestore EnrichmentBackend = "firestore"

	// EnrichmentMongo uses MongoDB (new addition).
	EnrichmentMongo EnrichmentBackend = "mongo"
)

// StorageConfig holds backend selection for ai-service storage.
type StorageConfig struct {
	// EnrichmentBackend selects where enrichment results are persisted.
	// Env: ENRICHMENT_BACKEND (default: "firestore")
	EnrichmentBackend EnrichmentBackend `env:"ENRICHMENT_BACKEND" default:"firestore"`

	// MongoURI is the MongoDB connection string.
	// Env: MONGO_URI
	MongoURI string `env:"MONGO_URI"`

	// MongoDatabase is the MongoDB database name.
	// Env: MONGO_DATABASE (default: "ai_service")
	MongoDatabase string `env:"MONGO_DATABASE" default:"ai_service"`
}
```

## Files to Extend

### Extend: `services/ai-service/cmd/server/main.go`

```go
// Thêm switch cho enrichment backend:
var enrichmentRepo enrichment.EnrichmentRepository

switch cfg.Storage.EnrichmentBackend {
case config.EnrichmentMongo:
    mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.Storage.MongoURI))
    if err != nil {
        log.Fatal().Err(err).Msg("failed to connect to MongoDB")
    }
    db := mongoClient.Database(cfg.Storage.MongoDatabase)
    enrichmentRepo, err = mongo_infra.NewEnrichmentRepo(db, logger)
    if err != nil {
        log.Fatal().Err(err).Msg("failed to create mongo enrichment repo")
    }
default:
    // Giữ existing Firestore (existing init code)
    enrichmentRepo = firestore_infra.NewEnrichmentRepo(firestoreClient)
}
```

## Verification

```bash
cd services/ai-service && go build ./...

# Test với MongoDB:
export ENRICHMENT_BACKEND=mongo
export MONGO_URI=mongodb://localhost:27017
./ai-service
# Check logs: should see "using MongoDB enrichment repo"

# Test với Firestore (default):
unset ENRICHMENT_BACKEND
./ai-service
# Check logs: should see "using Firestore enrichment repo"
```
