// Package main is the entry point for browse-service.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	deliveryhttp "github.com/osv/browse-service/internal/delivery/http"
	rediscache "github.com/osv/browse-service/internal/infra/redis"
	mongorepo "github.com/osv/browse-service/internal/infra/mongo"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	mongoURI := env("MONGO_URI", "mongodb://localhost:27017")
	mongoDB  := env("MONGO_DB", "cvedb")
	redisAddr := env("REDIS_ADDR", "localhost:6379")
	port     := env("PORT", "8080")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// MongoDB connection
	mongoClient, err := mongo.Connect(ctx, mongoopts.Client().ApplyURI(mongoURI).SetAppName("browse-service"))
	if err != nil {
		log.Fatal().Err(err).Msg("MongoDB connect failed")
	}
	pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
	if err := mongoClient.Ping(pingCtx, readpref.Primary()); err != nil {
		log.Fatal().Err(err).Msg("MongoDB ping failed")
	}
	pingCancel()
	defer mongoClient.Disconnect(ctx) //nolint:errcheck

	db := mongoClient.Database(mongoDB)

	// Redis connection
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close() //nolint:errcheck

	// Wire dependencies
	cache   := rediscache.NewBrowseCache(redisClient)
	cpeRepo := mongorepo.NewCPERepo(db)
	handler := deliveryhttp.NewHandler(cache, cpeRepo, log.Logger)
	router  := deliveryhttp.NewRouter(handler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Info().Str("port", port).Msg("browse-service started")
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
	log.Info().Msg("browse-service stopped")
}

func env(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
