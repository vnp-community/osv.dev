// Package mongo provides MongoDB-backed repositories for cve-service.
package mongo

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/osv/data-service/internal/domain/entity"
	"github.com/osv/data-service/internal/domain/repository"
)

type mongoCVERepo struct {
	col *mongo.Collection
}

// NewMongoCVERepository creates a new MongoDB-backed CVERepository.
// Operates on the "cves" collection of the provided database.
func NewMongoCVERepository(db *mongo.Database) repository.MongoDBCVERepository {
	return &mongoCVERepo{col: db.Collection("cves")}
}

// FindByID looks up a CVE by its NVD ID (e.g. "CVE-2021-44228").
func (r *mongoCVERepo) FindByID(ctx context.Context, id string) (*entity.CVE, error) {
	var cve entity.CVE
	err := r.col.FindOne(ctx, bson.M{"id": id}).Decode(&cve)
	if err == mongo.ErrNoDocuments {
		return nil, fmt.Errorf("CVE %s: %w", id, repository.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("FindByID %s: %w", id, err)
	}
	return &cve, nil
}

// FindByCPE searches for CVEs matching a CPE string using three strategies:
//   - StrictVendorProduct: exact regex match on vendors + products arrays
//   - Lax: match vendor+product extracted from CPE 2.3 FS (ignore version)
//   - Default: prefix regex match on vulnerable_configuration array
func (r *mongoCVERepo) FindByCPE(ctx context.Context, opts repository.CVESearchOptions) ([]*entity.CVE, error) {
	filter := buildCPEFilter(opts)
	findOpts := options.Find().SetSort(bson.D{{Key: "cvss3", Value: -1}})
	if opts.Limit > 0 {
		findOpts.SetLimit(int64(opts.Limit))
	}

	cursor, err := r.col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("FindByCPE: %w", err)
	}
	defer cursor.Close(ctx)

	var results []*entity.CVE
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("FindByCPE decode: %w", err)
	}
	return results, nil
}

// buildCPEFilter constructs a MongoDB filter for CPE-based search.
// Mirrors Python lib/Query.py::cveForCPE().
func buildCPEFilter(opts repository.CVESearchOptions) bson.M {
	if opts.StrictVendorProduct {
		// vendor:product exact match (from raw "vendor:product" string)
		parts := strings.SplitN(opts.CPE, ":", 2)
		if len(parts) == 2 {
			return bson.M{
				"vendors":  bson.M{"$regex": "^" + regexp.QuoteMeta(parts[0]) + "$", "$options": "i"},
				"products": bson.M{"$regex": "^" + regexp.QuoteMeta(parts[1]) + "$", "$options": "i"},
			}
		}
	}

	if opts.Lax {
		// Extract vendor + product from CPE 2.3 FS: cpe:2.3:a:{vendor}:{product}:...
		parts := strings.Split(opts.CPE, ":")
		if len(parts) >= 5 {
			return bson.M{
				"vendors":  parts[3],
				"products": parts[4],
			}
		}
	}

	// Default: prefix regex match on vulnerable_configuration
	// Strip trailing wildcard segments: "cpe:2.3:a:apache:log4j:*:*:*:*:*:*:*:*" → "cpe:2.3:a:apache:log4j"
	cpePrefix := regexp.MustCompile(`[:\*]+$`).ReplaceAllString(opts.CPE, "")
	cpePrefix = strings.TrimRight(cpePrefix, ":")
	return bson.M{
		"vulnerable_configuration": bson.M{
			"$elemMatch": bson.M{
				"$regex":   "^" + regexp.QuoteMeta(cpePrefix),
				"$options": "i",
			},
		},
	}
}

// FindLast returns the n most recently modified CVEs.
func (r *mongoCVERepo) FindLast(ctx context.Context, n int) ([]*entity.CVE, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "modified", Value: -1}}).
		SetLimit(int64(n))
	cursor, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("FindLast: %w", err)
	}
	defer cursor.Close(ctx)

	var results []*entity.CVE
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("FindLast decode: %w", err)
	}
	return results, nil
}

// FindRecent returns CVEs with modified >= since, sorted by modified desc.
func (r *mongoCVERepo) FindRecent(ctx context.Context, since time.Time, limit int) ([]*entity.CVE, error) {
	opts := options.Find().SetSort(bson.D{{Key: "modified", Value: -1}})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}
	cursor, err := r.col.Find(ctx, bson.M{"modified": bson.M{"$gte": since}}, opts)
	if err != nil {
		return nil, fmt.Errorf("FindRecent: %w", err)
	}
	defer cursor.Close(ctx)

	var results []*entity.CVE
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("FindRecent decode: %w", err)
	}
	return results, nil
}
