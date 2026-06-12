// Package mongo provides MongoDB L2 fallback for browse-service.
package mongo

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// CPERepo implements repository.CPERepository using MongoDB distinct queries.
type CPERepo struct {
	db *mongo.Database
}

// NewCPERepo creates a MongoDB-backed CPE repository.
func NewCPERepo(db *mongo.Database) *CPERepo {
	return &CPERepo{db: db}
}

// ListVendors returns all distinct vendors for a CPE type from MongoDB.
// cpeType: "a" (application), "o" (OS), "h" (hardware)
// Queries the "cpe" collection for cpeName entries matching cpe:2.3:{cpeType}:...
func (r *CPERepo) ListVendors(ctx context.Context, cpeType string) ([]string, error) {
	col := r.db.Collection("cpe")
	filter := bson.M{
		"cpeName": bson.M{
			"$regex":   fmt.Sprintf(`^cpe:2\.3:%s:`, regexp.QuoteMeta(cpeType)),
			"$options": "i",
		},
	}
	values, err := col.Distinct(ctx, "vendor", filter)
	if err != nil {
		return nil, fmt.Errorf("ListVendors: %w", err)
	}
	result := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok && s != "" && s != "*" {
			result = append(result, s)
		}
	}
	sort.Strings(result)
	return result, nil
}

// ListProducts returns all distinct products for a vendor.
func (r *CPERepo) ListProducts(ctx context.Context, vendor string) ([]string, error) {
	col := r.db.Collection("cpe")
	values, err := col.Distinct(ctx, "product", bson.M{"vendor": vendor})
	if err != nil {
		return nil, fmt.Errorf("ListProducts(%s): %w", vendor, err)
	}
	result := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok && s != "" && s != "*" {
			result = append(result, s)
		}
	}
	sort.Strings(result)
	return result, nil
}

// SearchVendors returns vendors whose name contains the query string (case-insensitive).
func (r *CPERepo) SearchVendors(ctx context.Context, query string) ([]string, error) {
	col := r.db.Collection("cpe")
	filter := bson.M{
		"vendor": bson.M{
			"$regex":   regexp.QuoteMeta(query),
			"$options": "i",
		},
	}
	values, err := col.Distinct(ctx, "vendor", filter)
	if err != nil {
		return nil, fmt.Errorf("SearchVendors(%s): %w", query, err)
	}
	result := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok && s != "" {
			result = append(result, s)
		}
	}
	sort.Strings(result)
	return result, nil
}
