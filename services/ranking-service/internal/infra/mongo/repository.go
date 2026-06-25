// Package mongo — MongoDB implementation of RankingRepository.
package mongo

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/osv/ranking-service/internal/domain"
)

type mongoRankingRepo struct {
	col *mongo.Collection
}

// NewMongoRankingRepo creates a MongoDB-backed RankingRepository.
func NewMongoRankingRepo(db *mongo.Database) domain.RankingRepository {
	return &mongoRankingRepo{col: db.Collection("ranking")}
}

// FindByCPE performs loosy CPE ranking lookup.
// Tries each meaningful CPE part as a regex against the "cpe" field.
// Returns first match; skips parts with no match.
// Mirrors Python: lib/CVEs.py::getranking(cpeid, loosy=True)
func (r *mongoRankingRepo) FindByCPE(ctx context.Context, cpe string) (*domain.LookupResult, error) {
	parts := domain.SplitCPEParts(cpe)

	for _, part := range parts {
		var entry domain.RankingEntry
		err := r.col.FindOne(ctx, bson.M{
			"cpe": bson.M{
				"$regex":   regexp.QuoteMeta(part),
				"$options": "i",
			},
		}).Decode(&entry)

		if err == mongo.ErrNoDocuments {
			continue // try next part
		}
		if err != nil {
			return nil, fmt.Errorf("ranking lookup: %w", err)
		}

		return &domain.LookupResult{
			CPE:         entry.CPE,
			Ranks:       entry.Rank,
			MatchedPart: part,
		}, nil
	}

	return nil, fmt.Errorf("no ranking found for CPE: %s", cpe)
}

// FindExact returns an exact-match ranking entry by CPE string.
func (r *mongoRankingRepo) FindExact(ctx context.Context, cpe string) (*domain.RankingEntry, error) {
	var entry domain.RankingEntry
	err := r.col.FindOne(ctx, bson.M{"cpe": strings.ToLower(cpe)}).Decode(&entry)
	if err == mongo.ErrNoDocuments {
		return nil, fmt.Errorf("ranking not found for: %s", cpe)
	}
	return &entry, err
}

// List returns paginated ranking entries, sorted by CPE.
func (r *mongoRankingRepo) List(ctx context.Context, limit, skip int) ([]*domain.RankingEntry, int64, error) {
	total, _ := r.col.CountDocuments(ctx, bson.M{})

	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "cpe", Value: 1}})

	cursor, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list rankings: %w", err)
	}
	defer cursor.Close(ctx)

	var entries []*domain.RankingEntry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, 0, fmt.Errorf("decode rankings: %w", err)
	}
	return entries, total, nil
}

// Save upserts a ranking entry by CPE (idempotent create-or-update).
func (r *mongoRankingRepo) Save(ctx context.Context, entry *domain.RankingEntry) (*domain.RankingEntry, error) {
	filter := bson.M{"cpe": strings.ToLower(entry.CPE)}
	update := bson.M{"$set": bson.M{
		"cpe":  strings.ToLower(entry.CPE),
		"rank": entry.Rank,
	}}
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var result domain.RankingEntry
	if err := r.col.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result); err != nil {
		return nil, fmt.Errorf("save ranking: %w", err)
	}
	return &result, nil
}

// Delete removes a ranking entry by MongoDB ObjectID hex string.
func (r *mongoRankingRepo) Delete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid id format: %w", err)
	}

	res, err := r.col.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return fmt.Errorf("delete ranking: %w", err)
	}
	if res.DeletedCount == 0 {
		return fmt.Errorf("ranking %s not found", id)
	}
	return nil
}
