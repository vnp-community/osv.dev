// Package scan provides the embedded HTTP entrypoint for scan-service.
// Wires scan HTTP handlers (scan CRUD, stats, agents) onto the provided ServeMux.
package scan

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	pgadapter "github.com/osv/scan-service/internal/adapters/repository/postgres"
	httpdelivery "github.com/osv/scan-service/internal/delivery/http"
	schedulehttp "github.com/osv/scan-service/internal/delivery/http/schedule"
	schedpostgres "github.com/osv/scan-service/internal/infra/persistence/postgres/schedule"
	agentuc "github.com/osv/scan-service/internal/usecase/agent"
)

// WireEmbedded initializes the Scan service routes on the provided ServeMux.
// When pool is non-nil, wires real PostgreSQL repositories.
// When pool is nil, handlers return graceful empty responses.
func WireEmbedded(ctx context.Context, logger zerolog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) error {
	var agentHandler *httpdelivery.AgentHandler
	var scanHandler *httpdelivery.ScanAPIHandler
	var statsHandler *httpdelivery.StatsHandler
	var scheduleHandler *schedulehttp.ScheduleHandler // [FIX TASK-HC-011]
	var resultsHandler *httpdelivery.ResultsHandler   // FIX: results/nmap + results/zap

	if pool != nil {
		// FIX MOCK-007 / MOCK-006: Wire real PostgreSQL repositories
		scanRepo := pgadapter.NewScanRepo(pool)

		// AgentUseCase: Register + List + SubmitReport backed by real DB
		agentUC := agentuc.NewUseCase(pool, nil)
		agentHandler = httpdelivery.NewAgentHandler(agentUC, logger)
		logger.Info().Msg("scan-service: AgentHandler wired (PostgreSQL)")

		// ScanAPIHandler via adapter (delivery layer uses interface{} slices)
		scanAdapter := newScanRepoAdapter(scanRepo)
		scanHandler = httpdelivery.NewScanAPIHandler(scanAdapter, logger)

		// Wire CreateScan use case so POST /api/v1/scans works
		createUC := newCreateScanUCAdapter(scanRepo)
		scanHandler = scanHandler.WithCreateScanUC(createUC)
		logger.Info().Msg("scan-service: ScanAPIHandler wired (PostgreSQL)")

		// StatsHandler via adapter
		statsAdapter := newStatsRepoAdapter(scanRepo)
		statsHandler = httpdelivery.NewStatsHandler(statsAdapter, logger)
		logger.Info().Msg("scan-service: StatsHandler wired (PostgreSQL)")

		// FIX: ResultsHandler for GET /scans/{id}/results/nmap and /results/zap
		findingRepo := pgadapter.NewFindingRepo(pool)
		alertRepo := pgadapter.NewWebAlertRepo(pool)
		resultsHandler = httpdelivery.NewResultsHandler(findingRepo, alertRepo, logger)
		logger.Info().Msg("scan-service: ResultsHandler wired (PostgreSQL)")

		// [FIX TASK-HC-011] Wire ScheduleHandler with real PostgreSQL repo
		schedRepo := schedpostgres.NewScheduleRepo(pool)
		scheduleHandler = schedulehttp.NewScheduleHandler(schedRepo, logger)
		logger.Info().Msg("scan-service: ScheduleHandler wired (PostgreSQL)")
	} else {
		// Graceful degradation: nil repos → empty/zero responses
		logger.Warn().Msg("scan-service: pool is nil — using stub handlers (no DB)")
		agentHandler = httpdelivery.NewAgentHandler(nil, logger)
		scanHandler = httpdelivery.NewScanAPIHandler(nil, logger)
		statsHandler = httpdelivery.NewStatsHandler(nil, logger)
	}

	// Build the router with all handlers including StatsHandler
	// NewRouterFull: scan CRUD + schedule + stats + results endpoints
	router := httpdelivery.NewRouterFull(
		nil,             // importHandler — not wired in embedded mode
		nil,             // parserHandler — not wired in embedded mode
		agentHandler,    // agentHandler
		scanHandler,     // scanHandler — provides GET/POST /api/v1/scans
		scheduleHandler, // [FIX TASK-HC-011]: wired with real DB repo when pool != nil
		statsHandler,    // statsHandler — CR-008: provides /api/v1/scans/stats and /stats/weekly
		resultsHandler,  // FIX: provides /api/v1/scans/{id}/results/nmap and /results/zap
		logger,
	)

	mux.Handle("/", router)
	return nil
}

