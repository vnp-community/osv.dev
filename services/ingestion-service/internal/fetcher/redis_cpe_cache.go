// Package fetcher — Redis CPE cache updater.
// Populates Redis vendor/product sets from MongoDB CPE collection.
// Mirrors Python: lib/DatabaseLayer.py CPERedisBrowser.update()
package fetcher

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// RedisCPECacheUpdater populates Redis CPE vendor/product sets from MongoDB.
//
// Redis key schema (mirrors Python CPERedisBrowser):
//
//	"a" → set of application vendors (from cpe:2.3:a:...)
//	"o" → set of OS vendors (from cpe:2.3:o:...)
//	"h" → set of hardware vendors (from cpe:2.3:h:...)
//	"v:{vendor}" → set of products for that vendor
//	"p:{product}" → set of full CPE name strings for that product
type RedisCPECacheUpdater struct {
	db    *mongo.Database
	redis *redis.Client
}

// NewRedisCPECacheUpdater creates a cache updater.
func NewRedisCPECacheUpdater(db *mongo.Database, redisClient *redis.Client) *RedisCPECacheUpdater {
	return &RedisCPECacheUpdater{db: db, redis: redisClient}
}

func (u *RedisCPECacheUpdater) Name() string { return "redis-cpe-cache" }

// FetchAndStore repopulates all Redis CPE sets from MongoDB CPE collection.
// This is idempotent — runs SADD which won't create duplicates.
func (u *RedisCPECacheUpdater) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
	log.Info().Msg("Starting Redis CPE cache refresh from MongoDB")

	col := u.db.Collection("cpe")
	cursor, err := col.Find(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("read CPE collection: %w", err)
	}
	defer cursor.Close(ctx)

	pipe := u.redis.Pipeline()
	count := 0
	batchSize := 1000

	for cursor.Next(ctx) {
		var doc struct {
			CPEName string `bson:"cpeName"`
			Vendor  string `bson:"vendor"`
			Product string `bson:"product"`
			CPEType string `bson:"cpeType"` // "a", "o", "h"
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		if doc.CPEName == "" {
			continue
		}

		// Parse CPE name if vendor/product not already set
		vendor, product, cpeType := doc.Vendor, doc.Product, doc.CPEType
		if vendor == "" || product == "" {
			parts := strings.Split(doc.CPEName, ":")
			if len(parts) >= 5 {
				cpeType = parts[2]
				vendor = parts[3]
				product = parts[4]
			}
		}

		if vendor == "" || vendor == "*" || vendor == "-" {
			continue
		}

		// Add to type set ("a", "o", "h")
		if cpeType != "" {
			pipe.SAdd(ctx, cpeType, vendor)
		}

		// Add to vendor→product mapping
		if product != "" && product != "*" && product != "-" {
			pipe.SAdd(ctx, "v:"+vendor, product)
			// Add CPE name to product set
			pipe.SAdd(ctx, "p:"+product, doc.CPEName)
		}

		count++

		// Flush every batchSize items
		if count%batchSize == 0 {
			if _, err := pipe.Exec(ctx); err != nil {
				log.Warn().Err(err).Int("batch", count/batchSize).Msg("Redis pipeline error")
			}
			pipe = u.redis.Pipeline()
			log.Info().Int("processed", count).Msg("Redis CPE cache progress")
		}
	}

	if err := cursor.Err(); err != nil {
		return count, fmt.Errorf("cursor error: %w", err)
	}

	// Flush remaining items
	if count%batchSize != 0 {
		if _, err := pipe.Exec(ctx); err != nil {
			log.Warn().Err(err).Msg("Redis final pipeline error")
		}
	}

	// Set expiry on type sets to avoid stale data (24h TTL)
	expiry := 25 * time.Hour
	for _, key := range []string{"a", "o", "h"} {
		u.redis.Expire(ctx, key, expiry) //nolint:errcheck
	}

	log.Info().Int("total", count).Msg("Redis CPE cache refresh complete")
	return count, nil
}

// RefreshRedisFromCVEs also builds vendor sets from the CVEs collection
// (vendors/products arrays stored directly on CVE documents).
func (u *RedisCPECacheUpdater) RefreshRedisFromCVEs(ctx context.Context) (int, error) {
	col := u.db.Collection("cves")

	// Use aggregation to get distinct vendors and their products
	cursor, err := col.Find(ctx, bson.M{
		"vendors": bson.M{"$exists": true, "$ne": bson.A{}},
	})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	pipe := u.redis.Pipeline()
	count := 0

	for cursor.Next(ctx) {
		var doc struct {
			Vendors  []string `bson:"vendors"`
			Products []string `bson:"products"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		for _, vendor := range doc.Vendors {
			if vendor == "" || vendor == "*" {
				continue
			}
			pipe.SAdd(ctx, "a", vendor) // assume "a" (application) for CVE vendors
			for _, product := range doc.Products {
				if product != "" && product != "*" {
					pipe.SAdd(ctx, "v:"+vendor, product)
				}
			}
			count++
		}

		if count%1000 == 0 {
			pipe.Exec(ctx) //nolint:errcheck
			pipe = u.redis.Pipeline()
		}
	}

	if count%1000 != 0 {
		pipe.Exec(ctx) //nolint:errcheck
	}

	return count, nil
}
