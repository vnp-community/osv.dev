package sla

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	httpdelivery "github.com/osv/sla-service/internal/delivery/http"
	"github.com/osv/sla-service/internal/infra/postgres"
	ucconfig "github.com/osv/sla-service/internal/usecase/config"
)

// noopEventPublisher satisfies ucconfig.EventPublisher by doing nothing.
type noopEventPublisher struct{}

func (n *noopEventPublisher) Publish(_ context.Context, _ string, _ map[string]any) error {
	return nil
}

// WireEmbedded initializes the SLA service routes on the provided ServeMux.
func WireEmbedded(ctx context.Context, logger zerolog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) error {
	// Initialize repositories
	configRepo := postgres.NewSLAConfigRepo(pool)
	assignRepo := postgres.NewSLAAssignmentRepo(pool)

	// EventPublisher — no-op in embedded mode
	ep := &noopEventPublisher{}

	// Initialize use cases
	createUC := ucconfig.NewCreate(configRepo, ep)
	updateUC := ucconfig.NewUpdate(configRepo, ep)
	deleteUC := ucconfig.NewDelete(configRepo, assignRepo)
	assignUC := ucconfig.NewAssignProduct(configRepo, assignRepo, ep)

	// Initialize handler
	configHandler := httpdelivery.NewSLAConfigHandler(createUC, updateUC, deleteUC, assignUC, configRepo)

	// Register routes using flat chi router (no trailing-slash redirect needed).
	// Explicit registration for both /api/v2/sla-configurations and
	// /api/v2/sla-configurations/ ensures POST without trailing slash works.
	r := chi.NewRouter()

	const base = "/api/v2/sla-configurations"

	// Collection endpoints — match both with and without trailing slash
	r.Post(base, configHandler.Create)
	r.Post(base+"/", configHandler.Create)
	r.Get(base, configHandler.List)
	r.Get(base+"/", configHandler.List)

	// Bulk sub-routes
	r.Post(base+"/bulk", configHandler.BulkCreateConfigs)
	r.Post(base+"/assign-bulk", configHandler.BulkAssign)

	// Item endpoints
	r.Get(base+"/{id}", configHandler.Get)
	r.Put(base+"/{id}", configHandler.Update)
	r.Delete(base+"/{id}", configHandler.Delete)
	r.Post(base+"/{id}/assign/{product_id}", configHandler.Assign)

	// v1 SLA config legacy/alias routes
	r.Get("/api/v1/sla/config", configHandler.GetConfig)
	r.Put("/api/v1/sla/config", configHandler.UpdateConfig)

	mux.Handle("/", r)
	return nil
}
