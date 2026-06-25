// Package ranking provides the embedded entry point for ranking-service when running
// as part of the osv-server monolith.
package ranking

import (
	"context"
	"net/http"
	"os"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/mongo"
	mongooptions "go.mongodb.org/mongo-driver/mongo/options"

	deliveryhttp "github.com/osv/ranking-service/internal/delivery/http"
	mongorepo "github.com/osv/ranking-service/internal/infra/mongo"
)

// WireEmbedded registers the ranking-service HTTP routes on the provided mux.
//
// Routes registered:
//
//	GET    /api/v1/ranking          → Handler.ListRankings
//	GET    /api/v1/ranking/lookup   → Handler.LookupRanking
//	POST   /api/v1/ranking          → Handler.CreateRanking
//	POST   /api/v1/ranking/bulk     → Handler.BulkCreateRankings
//	DELETE /api/v1/ranking/{id}     → Handler.DeleteRanking
func WireEmbedded(ctx context.Context, logger zerolog.Logger, mux *http.ServeMux) error {
	mongoURI := os.Getenv("MONGO_URI")
	mongoDB := os.Getenv("MONGO_DB")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}
	if mongoDB == "" {
		mongoDB = "ranking"
	}

	client, err := mongo.Connect(ctx, mongooptions.Client().ApplyURI(mongoURI))
	if err != nil {
		return err
	}
	if err := client.Ping(ctx, nil); err != nil {
		logger.Warn().Err(err).Msg("[ranking] MongoDB ping failed — ranking features unavailable")
		return nil // Soft fail: ranking is optional
	}

	db := client.Database(mongoDB)
	repo := mongorepo.NewMongoRankingRepo(db)
	handler := deliveryhttp.NewHandler(repo)
	router := deliveryhttp.NewRouter(handler)

	// Mount with /api/v1 prefix stripped — chi router uses /ranking/... paths
	stripped := http.StripPrefix("/api/v1", router)
	mux.Handle("/api/v1/ranking", stripped)
	mux.Handle("/api/v1/ranking/", stripped)

	logger.Info().Str("mongo_db", mongoDB).Msg("[ranking] WireEmbedded complete")
	return nil
}
