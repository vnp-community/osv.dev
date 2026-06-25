package asset

import (
	"context"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog"

	httpdelivery "github.com/google/osv.dev/services/asset-service/internal/delivery/http"
	mygrpc "github.com/google/osv.dev/services/asset-service/internal/infra/grpc"
	assetinfra "github.com/google/osv.dev/services/asset-service/internal/infra/nats"
	"github.com/google/osv.dev/services/asset-service/internal/infra/postgres"
	ucasset "github.com/google/osv.dev/services/asset-service/internal/usecase/asset"
	natsgo "github.com/nats-io/nats.go"
)

// noopEventPublisher satisfies the asset.EventPublisher interface by doing nothing.
type noopEventPublisher struct{}

func (n *noopEventPublisher) Publish(_ string, _ map[string]any) error {
	return nil
}

// WireEmbedded initializes the Asset service routes on the provided ServeMux.
func WireEmbedded(ctx context.Context, logger zerolog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) error {
	// Derive *sql.DB from pgxpool for the asset-service (which uses database/sql)
	sqlDB := stdlib.OpenDBFromPool(pool)

	// Initialize repository
	repo := postgres.NewAssetRepository(sqlDB)

	// Initialize Finding gRPC Client (lazy connection, falls back to noop)
	var fc ucasset.FindingClient = &mygrpc.NoopFindingClient{}
	grpcTarget := os.Getenv("FINDING_SERVICE_GRPC")
	if grpcTarget == "" {
		grpcTarget = "localhost:50060"
	}
	if realFC, err := mygrpc.NewFindingClient(grpcTarget); err == nil {
		fc = realFC
	} else {
		logger.Warn().Err(err).Msg("Finding gRPC client unavailable, risk scoring will return 0")
	}

	// FIX MOCK-015: Wire real NATS EventPublisher instead of noopEventPublisher
	var eventPub ucasset.EventPublisher = &noopEventPublisher{} // graceful fallback
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}
	nc, natsErr := natsgo.Connect(natsURL,
		natsgo.RetryOnFailedConnect(true),
		natsgo.MaxReconnects(5),
	)
	if natsErr != nil {
		logger.Warn().Err(natsErr).Msg("asset-service: NATS unavailable, asset events disabled")
	} else {
		eventPub = assetinfra.NewAssetEventPublisher(nc)
		logger.Info().Msg("asset-service: NATS event publisher connected")
	}

	// Initialize use cases
	crudUC := ucasset.NewAssetCRUDUseCase(repo, eventPub)
	taggingUC := ucasset.NewTaggingUseCase(repo)
	riskUC := ucasset.NewRiskScoringUseCase(repo, fc)
	listUC := ucasset.NewListAssetsUseCase(repo)
	updateUC := ucasset.NewUpdateAssetUseCase(repo) // TASK-008 FIX

	// Initialize handler and mount router
	// Mount at /api/v1 so gateway's /api/v1/assets forwards correctly to
	// the chi sub-routes defined as /assets/... within handler.Router()
	handler := httpdelivery.NewHandler(crudUC, taggingUC, riskUC, listUC, logger).
		WithUpdateUC(updateUC) // TASK-008 FIX: wire PATCH/PUT usecase
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", handler.Router()))

	return nil
}
