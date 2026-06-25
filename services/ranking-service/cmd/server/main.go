// Package main — ranking-service entry point.
// Provides CPE ranking CRUD and loosy lookup via HTTP REST.
// Port: 8088 (default)
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	mongoinfra "github.com/osv/ranking-service/internal/infra/mongo"
	deliveryhttp "github.com/osv/ranking-service/internal/delivery/http"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	mongoURI := envDefault("MONGO_URI", "mongodb://localhost:27017")
	mongoDB  := envDefault("MONGO_DB", "cvedb")
	port     := envDefault("RANKING_PORT", envDefault("PORT", "8088"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// MongoDB connect
	client, err := mongo.Connect(ctx,
		mongoopts.Client().ApplyURI(mongoURI).SetAppName("ranking-service"),
	)
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

	// Create MongoDB indexes
	ensureIndexes(ctx, db)

	// Wire dependencies
	repo    := mongoinfra.NewMongoRankingRepo(db)
	handler := deliveryhttp.NewHandler(repo)
	router  := deliveryhttp.NewRouter(handler)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("port", port).Msg("ranking-service started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down ranking-service...")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	srv.Shutdown(shutCtx) //nolint:errcheck
}

func ensureIndexes(ctx context.Context, db *mongo.Database) {
	col := db.Collection("ranking")
	_, err := col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "cpe", Value: 1}},
			Options: mongoopts.Index().SetUnique(true).SetName("ranking_cpe_unique"),
		},
		{
			Keys:    bson.D{{Key: "rank.group", Value: 1}},
			Options: mongoopts.Index().SetName("ranking_group"),
		},
	})
	if err != nil {
		log.Warn().Err(err).Msg("Create indexes warning (may already exist)")
	}
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
