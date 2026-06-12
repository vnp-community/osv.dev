// Package main — ingest-service entry point.
// Orchestrates data ingestion from NVD, MITRE, EPSS into MongoDB.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	natspkg "github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/osv/ingest-service/internal/fetcher"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	mongoURI  := envDefault("MONGO_URI", "mongodb://localhost:27017")
	mongoDB   := envDefault("MONGO_DB", "cvedb")
	nvdAPIKey := envDefault("NVD_API_KEY", "")
	port      := envDefault("PORT", "8080")
	natsURL   := envDefault("NATS_URL", "")
	redisAddr := envDefault("REDIS_ADDR", "")
	intervalH := 24 // hours

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := mongo.Connect(ctx, mongoopts.Client().ApplyURI(mongoURI).SetAppName("ingest-service"))
	if err != nil {
		log.Fatal().Err(err).Msg("MongoDB connect failed")
	}
	pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		log.Fatal().Err(err).Msg("MongoDB ping failed")
	}
	pingCancel()
	defer client.Disconnect(ctx) //nolint:errcheck

	db := client.Database(mongoDB)

	// Distributed lock collection
	lockCol := db.Collection("updaterLocks")
	infoCol := db.Collection("info")

	// Initialize fetchers — core
	fetchers := map[string]fetcher.Fetcher{
		"cve":   fetcher.NewNVDCVEFetcher(db, nvdAPIKey, 2002),
		"cpe":   fetcher.NewNVDCPEFetcher(db, nvdAPIKey),
		"cwe":   fetcher.NewMITRECWEFetcher(db),
		"capec": fetcher.NewMITRECAPECFetcher(db),
		"epss":  fetcher.NewEPSSFetcher(db),
	}

	// Optional: Redis CPE cache updater (runs after CPE fetch)
	var redisCacheUpdater *fetcher.RedisCPECacheUpdater
	if redisAddr != "" {
		redisClient := redis.NewClient(&redis.Options{
			Addr: redisAddr,
			DB:   10, // REDIS_VENDOR_DB
		})
		redisCacheUpdater = fetcher.NewRedisCPECacheUpdater(db, redisClient)
		log.Info().Str("redis", redisAddr).Msg("Redis CPE cache updater enabled")
	}

	// Optional: NATS publisher for cve.batch.updated events
	var natsConn *natspkg.Conn
	if natsURL != "" {
		natsConn, err = natspkg.Connect(natsURL,
			natspkg.RetryOnFailedConnect(true),
			natspkg.MaxReconnects(10),
			natspkg.ReconnectWait(2*time.Second),
		)
		if err != nil {
			log.Warn().Err(err).Msg("NATS connect failed — events will not be published")
			natsConn = nil
		} else {
			log.Info().Str("nats", natsURL).Msg("NATS connected")
			defer natsConn.Close()
		}
	}

	// Start scheduler
	go runScheduler(ctx, db, lockCol, infoCol, fetchers, redisCacheUpdater, natsConn, time.Duration(intervalH)*time.Hour)

	// HTTP API
	r := chi.NewRouter()
	r.Use(middleware.RealIP, middleware.Recoverer, middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "ingest-service"})
	})

	// GET /status — ingest status for all sources
	r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		cursor, err := infoCol.Find(r.Context(), bson.M{})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer cursor.Close(r.Context())
		var infos []bson.M
		cursor.All(r.Context(), &infos) //nolint:errcheck
		writeJSON(w, http.StatusOK, map[string]interface{}{"sources": infos})
	})

	// POST /trigger — manual update trigger
	r.Post("/trigger", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Sources []string `json:"sources"`
			Days    int      `json:"days"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if len(req.Sources) == 0 {
			req.Sources = []string{"cve"}
		}
		days := req.Days
		if days <= 0 {
			days = 1
		}

		// Run in background
		go func() {
			trigCtx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
			defer cancel()
			for _, src := range req.Sources {
				f, ok := fetchers[src]
				if !ok {
					log.Warn().Str("source", src).Msg("unknown source, skipping")
					continue
				}
				acquired := tryAcquireLock(trigCtx, lockCol, src, 4*time.Hour)
				if !acquired {
					log.Info().Str("source", src).Msg("already locked, skipping")
					continue
				}
				count, err := f.FetchAndStore(trigCtx, fetcher.FetchOptions{ManualDays: days})
				releaseLock(trigCtx, lockCol, src)
				if err != nil {
					log.Error().Err(err).Str("source", src).Msg("fetch failed")
					continue
				}
				log.Info().Str("source", src).Int("count", count).Msg("fetch complete")
				setLastModified(trigCtx, infoCol, src, time.Now().UTC())
				// Publish NATS event for CVE updates
				if natsConn != nil && src == "cve" && count > 0 {
					event := fmt.Sprintf(`{"source":"%s","count":%d,"timestamp":"%s"}`,
						src, count, time.Now().UTC().Format(time.RFC3339))
					natsConn.Publish("cve.batch.updated", []byte(event)) //nolint:errcheck
				}
				// Refresh Redis cache for CPE updates
				if redisCacheUpdater != nil && src == "cpe" {
					go redisCacheUpdater.FetchAndStore(context.Background(), fetcher.FetchOptions{}) //nolint:errcheck
				}
			}
		}()

		writeJSON(w, http.StatusAccepted, map[string]string{
			"status":  "triggered",
			"sources": strings.Join(req.Sources, ","),
		})
	})

	// POST /import — full import (admin)
	r.Post("/import", func(w http.ResponseWriter, r *http.Request) {
		go func() {
			importCtx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
			defer cancel()
			for name, f := range fetchers {
				count, err := f.FetchAndStore(importCtx, fetcher.FetchOptions{ManualDays: 0})
				if err != nil {
					log.Error().Err(err).Str("source", name).Msg("full import failed")
					continue
				}
				log.Info().Str("source", name).Int("count", count).Msg("full import complete")
				setLastModified(importCtx, infoCol, name, time.Now().UTC())
			}
		}()
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "full import triggered"})
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Info().Str("port", port).Msg("ingest-service started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	srv.Shutdown(shutCtx) //nolint:errcheck
}

// runScheduler runs the update loop on a configurable interval.
func runScheduler(ctx context.Context, db *mongo.Database, lockCol, infoCol *mongo.Collection,
	fetchers map[string]fetcher.Fetcher, cacheUpdater *fetcher.RedisCPECacheUpdater,
	nc *natspkg.Conn, interval time.Duration) {

	log.Info().Dur("interval", interval).Msg("ingest scheduler started")
	runUpdate(ctx, lockCol, infoCol, fetchers, cacheUpdater, nc, 1) // initial run

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			runUpdate(ctx, lockCol, infoCol, fetchers, cacheUpdater, nc, 1)
		case <-ctx.Done():
			return
		}
	}
}

func runUpdate(ctx context.Context, lockCol, infoCol *mongo.Collection,
	fetchers map[string]fetcher.Fetcher, cacheUpdater *fetcher.RedisCPECacheUpdater,
	nc *natspkg.Conn, manualDays int) {
	for name, f := range fetchers {
		if !tryAcquireLock(ctx, lockCol, name, 4*time.Hour) {
			continue
		}
		count, err := f.FetchAndStore(ctx, fetcher.FetchOptions{ManualDays: manualDays})
		releaseLock(ctx, lockCol, name)
		if err != nil {
			log.Error().Err(err).Str("source", name).Msg("scheduled update failed")
			continue
		}
		log.Info().Str("source", name).Int("count", count).Msg("scheduled update complete")
		setLastModified(ctx, infoCol, name, time.Now().UTC())

		// Publish NATS event after CVE batch update
		if nc != nil && name == "cve" && count > 0 {
			event := fmt.Sprintf(`{"source":"%s","count":%d,"timestamp":"%s"}`,
				name, count, time.Now().UTC().Format(time.RFC3339))
			if err := nc.Publish("cve.batch.updated", []byte(event)); err != nil {
				log.Warn().Err(err).Msg("NATS publish cve.batch.updated failed")
			} else {
				log.Info().Str("subject", "cve.batch.updated").Int("count", count).Msg("NATS event published")
			}
		}

		// Refresh Redis CPE cache after CPE fetch
		if cacheUpdater != nil && name == "cpe" {
			go func() {
				cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 2*time.Hour)
				defer cacheCancel()
				n, err := cacheUpdater.FetchAndStore(cacheCtx, fetcher.FetchOptions{})
				if err != nil {
					log.Error().Err(err).Msg("Redis CPE cache refresh failed")
				} else {
					log.Info().Int("count", n).Msg("Redis CPE cache refreshed")
				}
			}()
		}
	}
}

// tryAcquireLock atomically acquires a MongoDB-backed distributed lock.
func tryAcquireLock(ctx context.Context, col *mongo.Collection, source string, maxDuration time.Duration) bool {
	now := time.Now().UTC()
	// Delete stale lock
	col.DeleteOne(ctx, bson.M{ //nolint:errcheck
		"_id":        source,
		"started_at": bson.M{"$lt": now.Add(-maxDuration)},
	})
	result := col.FindOneAndUpdate(
		ctx,
		bson.M{"_id": source},
		bson.M{"$setOnInsert": bson.M{"_id": source, "started_at": now}},
		mongoopts.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(mongoopts.Before),
	)
	var existing bson.M
	err := result.Decode(&existing)
	return err == mongo.ErrNoDocuments
}

func releaseLock(ctx context.Context, col *mongo.Collection, source string) {
	col.DeleteOne(ctx, bson.M{"_id": source}) //nolint:errcheck
}

func setLastModified(ctx context.Context, col *mongo.Collection, source string, t time.Time) {
	col.UpdateOne(ctx,
		bson.M{"db": source},
		bson.M{"$set": bson.M{"lastModified": t}},
		mongoopts.Update().SetUpsert(true),
	) //nolint:errcheck
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
