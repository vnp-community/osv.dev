// Package audit provides the embedded entry point for audit-service when running
// as part of the osv-server monolith.
package audit

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	auditpg "github.com/osv/audit-service/internal/infra/postgres"
	deliveryhttp "github.com/osv/audit-service/internal/delivery/http"
)

// WireEmbedded registers the audit-service HTTP routes on the provided mux.
//
// Routes registered:
//
//	GET /api/v1/audit-log  → AuditHandler.ListAuditLog
//	GET /api/v2/audit-log  → AuditHandler.ListAuditLog
func WireEmbedded(ctx context.Context, logger zerolog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) error {
	// HTTP read side — query audit_events via postgres
	auditRepo := auditpg.NewAuditRepo(pool)
	auditHandler := deliveryhttp.NewAuditHandler(auditRepo)
	router := deliveryhttp.NewAuditRouter(auditHandler)

	// Register routes on shared mux
	mux.Handle("/api/v1/audit-log", router)
	mux.Handle("/api/v1/audit-log/", router)
	mux.Handle("/api/v2/audit-log", router)
	mux.Handle("/api/v2/audit-log/", router)

	logger.Info().Msg("[audit] WireEmbedded complete")
	return nil
}
