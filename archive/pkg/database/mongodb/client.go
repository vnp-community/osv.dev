// Package mongodb provides a MongoDB client factory for all cve-search services.
// Usage:
//
//	db, cleanup, err := mongodb.Connect(ctx, mongodb.Config{
//	    URI:      os.Getenv("MONGO_URI"),
//	    Database: os.Getenv("MONGO_DB"),
//	})
//	if err != nil { log.Fatal(err) }
//	defer cleanup()
package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Config holds MongoDB connection settings.
type Config struct {
	// URI is the MongoDB connection string, e.g. "mongodb://localhost:27017"
	URI string

	// Database is the name of the database to use, e.g. "cvedb"
	Database string

	// Timeout is the connection and ping timeout (default: 10s).
	Timeout time.Duration

	// AppName is the application name sent to MongoDB for monitoring (default: "cve-search").
	AppName string
}

// Connect creates a MongoDB client, pings the primary, and returns the database handle.
// The cleanup function disconnects the client; call it with defer in main().
func Connect(ctx context.Context, cfg Config) (*mongo.Database, func(), error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.AppName == "" {
		cfg.AppName = "cve-search"
	}
	if cfg.Database == "" {
		cfg.Database = "cvedb"
	}

	clientOpts := options.Client().
		ApplyURI(cfg.URI).
		SetConnectTimeout(cfg.Timeout).
		SetServerSelectionTimeout(cfg.Timeout).
		SetAppName(cfg.AppName)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("mongodb connect: %w", err)
	}

	// Verify connectivity with a ping
	pingCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		client.Disconnect(ctx) //nolint:errcheck
		return nil, nil, fmt.Errorf("mongodb ping %s: %w", cfg.URI, err)
	}

	cleanup := func() {
		disconnectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client.Disconnect(disconnectCtx) //nolint:errcheck
	}

	return client.Database(cfg.Database), cleanup, nil
}

// MustConnect calls Connect and panics on failure.
// Use in main() only where failure is unrecoverable.
func MustConnect(ctx context.Context, cfg Config) (*mongo.Database, func()) {
	db, cleanup, err := Connect(ctx, cfg)
	if err != nil {
		panic(fmt.Sprintf("mongodb MustConnect: %v", err))
	}
	return db, cleanup
}
