// Package mongodb — index management helpers for cve-search collections.
package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// IndexDef defines a MongoDB index for use with EnsureIndexes.
type IndexDef struct {
	Name   string // index name (must be unique per collection)
	Keys   bson.D // ordered key spec, e.g. bson.D{{Key: "id", Value: 1}}
	Unique bool   // enforce uniqueness
	Sparse bool   // only index documents containing the indexed field
	TTLSec int32  // seconds until document expiry (0 = no TTL)
}

// EnsureIndexes creates indexes on col if they do not already exist.
// Idempotent: duplicate index errors are silently ignored.
func EnsureIndexes(ctx context.Context, col *mongo.Collection, defs []IndexDef) error {
	models := make([]mongo.IndexModel, 0, len(defs))
	for _, d := range defs {
		opt := options.Index().SetName(d.Name)
		if d.Unique {
			opt.SetUnique(true)
		}
		if d.Sparse {
			opt.SetSparse(true)
		}
		if d.TTLSec > 0 {
			opt.SetExpireAfterSeconds(d.TTLSec)
		}
		models = append(models, mongo.IndexModel{Keys: d.Keys, Options: opt})
	}
	if len(models) == 0 {
		return nil
	}
	_, err := col.Indexes().CreateMany(ctx, models)
	if isDuplicateIndexErr(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("EnsureIndexes on %s: %w", col.Name(), err)
	}
	return nil
}

// EnsureCVEIndexes creates the standard indexes for the "cves" collection.
// Call this once on service startup (idempotent).
func EnsureCVEIndexes(ctx context.Context, db *mongo.Database) error {
	return EnsureIndexes(ctx, db.Collection("cves"), []IndexDef{
		// Primary lookup
		{Name: "cve_id_unique", Keys: bson.D{{Key: "id", Value: 1}}, Unique: true},

		// CPE / vendor / product search
		{Name: "vulnerable_cfg_idx", Keys: bson.D{{Key: "vulnerable_configuration", Value: 1}}},
		{Name: "vendors_idx", Keys: bson.D{{Key: "vendors", Value: 1}}},
		{Name: "products_idx", Keys: bson.D{{Key: "products", Value: 1}}},

		// Date-based sorting (FindLast, FindRecent)
		{Name: "modified_idx", Keys: bson.D{{Key: "modified", Value: -1}}},
		{Name: "published_idx", Keys: bson.D{{Key: "published", Value: -1}}},

		// CVSS sorting for ranked results
		{Name: "cvss3_idx", Keys: bson.D{{Key: "cvss3", Value: -1}}},

		// Full-text search on summary (one text index per collection)
		{Name: "summary_text_idx", Keys: bson.D{{Key: "summary", Value: "text"}}},
	})
}

// EnsureCPEIndexes creates indexes for the "cpe" collection.
func EnsureCPEIndexes(ctx context.Context, db *mongo.Database) error {
	return EnsureIndexes(ctx, db.Collection("cpe"), []IndexDef{
		{Name: "cpe_name_unique", Keys: bson.D{{Key: "cpeName", Value: 1}}, Unique: true},
		{Name: "cpe_vendor_idx", Keys: bson.D{{Key: "vendor", Value: 1}}},
		{Name: "cpe_product_idx", Keys: bson.D{{Key: "product", Value: 1}}},
	})
}

// isDuplicateIndexErr returns true for MongoDB duplicate index errors (codes 85, 86).
func isDuplicateIndexErr(err error) bool {
	if err == nil {
		return false
	}
	if we, ok := err.(mongo.WriteException); ok {
		for _, e := range we.WriteErrors {
			if e.Code == 85 || e.Code == 86 {
				return true
			}
		}
	}
	return false
}
