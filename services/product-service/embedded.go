// Package product provides the embedded product-service HTTP handler.
// It wires products, engagements, and tests CRUD to a PostgreSQL pool.
package product

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	handler "github.com/google/osv.dev/services/product-service/internal/delivery/http"
)

// WireEmbedded mounts the product-service routes on the provided ServeMux.
// Routes are exposed under /api/v1/:
//
//	POST   /api/v1/products
//	GET    /api/v1/products
//	GET    /api/v1/products/{id}
//	POST   /api/v1/products/{id}/engagements
//	GET    /api/v1/products/{id}/engagements
//	POST   /api/v1/engagements/{id}/tests
func WireEmbedded(_ context.Context, logger zerolog.Logger, db *pgxpool.Pool, mux *http.ServeMux) error {
	h := handler.NewHandler(db, logger)
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Mount("/api/v1", h.Router())
	mux.Handle("/", r)
	return nil
}
