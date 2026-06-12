// Package testutil — MongoDB Testcontainer helpers for integration tests.
package testutil

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/mongo"

	pkgmongo "github.com/osv/pkg/database/mongodb"
)

// StartMongoDB starts a MongoDB testcontainer and returns a connected *mongo.Database.
// The container is automatically terminated when the test ends via t.Cleanup.
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    db := testutil.StartMongoDB(t)
//	    repo := myrepo.New(db)
//	    ...
//	}
//
// Requires Docker. Skip gracefully with:
//
//	if os.Getenv("DOCKER_HOST") == "" { t.Skip("Docker not available") }
func StartMongoDB(t *testing.T) *mongo.Database {
	t.Helper()
	ctx := context.Background()

	container, err := tcmongo.RunContainer(ctx,
		testcontainers.WithImage("mongo:7.0"),
	)
	if err != nil {
		t.Fatalf("testutil: start MongoDB container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("testutil: terminate MongoDB container: %v", err)
		}
	})

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("testutil: get MongoDB connection string: %v", err)
	}

	db, cleanup, err := pkgmongo.Connect(ctx, pkgmongo.Config{
		URI:      uri,
		Database: "cvedb_test",
		AppName:  "cve-search-test",
	})
	if err != nil {
		t.Fatalf("testutil: connect to MongoDB: %v", err)
	}
	t.Cleanup(cleanup)

	return db
}

// SeedMongoDB inserts documents into a named collection for testing.
// Fails the test immediately if insertion fails.
func SeedMongoDB(t *testing.T, db *mongo.Database, collection string, docs []interface{}) {
	t.Helper()
	if len(docs) == 0 {
		return
	}
	_, err := db.Collection(collection).InsertMany(context.Background(), docs)
	if err != nil {
		t.Fatalf("testutil: seed MongoDB collection %q: %v", collection, err)
	}
}
