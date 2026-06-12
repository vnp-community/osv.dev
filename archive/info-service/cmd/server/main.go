// Package main — info-service entry point.
// Provides MongoDB database statistics.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// CollectionInfo holds stats for a single MongoDB collection.
type CollectionInfo struct {
	Name         string    `json:"name"`
	Count        int64     `json:"count"`
	LastModified time.Time `json:"last_modified,omitempty"`
}

// DBInfo is the response body for GET /dbinfo.
type DBInfo struct {
	Collections  []CollectionInfo `json:"collections"`
	TotalCVEs    int64            `json:"total_cves"`
	TotalCPEs    int64            `json:"total_cpes"`
	DatabaseSize string           `json:"database_size"`
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	mongoURI := envDefault("MONGO_URI", "mongodb://localhost:27017")
	mongoDB  := envDefault("MONGO_DB", "cvedb")
	port     := envDefault("PORT", "8080")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := mongo.Connect(ctx, mongoopts.Client().ApplyURI(mongoURI).SetAppName("info-service"))
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

	r := chi.NewRouter()
	r.Use(middleware.RealIP, middleware.Recoverer, middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "info-service"})
	})

	// GET /dbinfo — full database stats
	r.Get("/dbinfo", func(w http.ResponseWriter, r *http.Request) {
		info := getDBInfo(r.Context(), db)
		writeJSON(w, http.StatusOK, info)
	})

	// GET /dbinfo/collections — just collection list
	r.Get("/dbinfo/collections", func(w http.ResponseWriter, r *http.Request) {
		info := getDBInfo(r.Context(), db)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"collections": info.Collections,
		})
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Info().Str("port", port).Msg("info-service started")
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

func getDBInfo(ctx context.Context, db *mongo.Database) *DBInfo {
	tracked := []string{"cves", "cpe", "cwe", "capec", "via4", "ranking"}
	info := &DBInfo{}

	for _, name := range tracked {
		count, _ := db.Collection(name).CountDocuments(ctx, bson.M{})

		// Retrieve last modified from info collection
		var infoDoc struct {
			LastModified time.Time `bson:"lastModified"`
		}
		db.Collection("info").FindOne(ctx, bson.M{"db": name}).Decode(&infoDoc) //nolint:errcheck

		ci := CollectionInfo{
			Name:         name,
			Count:        count,
			LastModified: infoDoc.LastModified,
		}
		info.Collections = append(info.Collections, ci)
		if name == "cves" {
			info.TotalCVEs = count
		}
		if name == "cpe" {
			info.TotalCPEs = count
		}
	}

	// Database size via dbStats command
	var stats bson.M
	db.RunCommand(ctx, bson.D{{Key: "dbStats", Value: 1}}).Decode(&stats) //nolint:errcheck
	if dataSize, ok := stats["dataSize"].(float64); ok {
		info.DatabaseSize = fmt.Sprintf("%.2f MB", dataSize/1024/1024)
	}

	return info
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
