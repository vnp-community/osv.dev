// Package mongo — upsert.go
// Adds Upsert method to mongoCVERepo for the ingest pipeline.
// ADDITIVE: cve_repo.go is not modified.
package mongo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/osv/shared/pkg/osvschema"
)

// Upsert inserts or replaces an OSV record in MongoDB by normalized ID.
// The raw JSON is re-parsed to extract metadata for indexed fields.
// mongoCVERepo satisfies the IngestRepository interface after this addition.
func (r *mongoCVERepo) Upsert(ctx context.Context, rawRecord []byte, id, source string) error {
	// Parse for indexed field extraction only
	var osv osvschema.Vulnerability
	if err := json.Unmarshal(rawRecord, &osv); err != nil {
		return fmt.Errorf("upsert: parse osv record: %w", err)
	}

	filter := bson.M{"id": id}
	update := bson.M{
		"$set": bson.M{
			"id":         id,
			"source":     source,
			"raw":        string(rawRecord),
			"modified":   osv.Modified,
			"published":  osv.Published,
			"summary":    osv.Summary,
			"aliases":    osv.Aliases,
			"updated_at": time.Now().UTC(),
		},
		"$setOnInsert": bson.M{
			"created_at": time.Now().UTC(),
		},
	}
	opts := options.Update().SetUpsert(true)
	_, err := r.col.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("upsert %s: %w", id, err)
	}
	return nil
}
