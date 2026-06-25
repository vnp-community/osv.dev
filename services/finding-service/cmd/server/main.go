// Command finding-service is the unified DefectDojo Finding Platform.
// Consolidates: finding-management + sla + audit.
// Exposes HTTP REST API on port 8085 and gRPC on port 50055.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	deliveryhttp "github.com/osv/finding-service/internal/delivery/http"
	"github.com/osv/finding-service/internal/infra/postgres"
	ucfinding "github.com/osv/finding-service/internal/usecase/finding"
	enguc "github.com/osv/finding-service/internal/usecase/engagement"
	reportuc "github.com/osv/finding-service/internal/usecase/report"
	rauc "github.com/osv/finding-service/internal/usecase/riskacceptance"
	stats_uc "github.com/osv/finding-service/internal/usecase"
	findingpkg "github.com/osv/finding-service"

	_ "github.com/rs/zerolog" // keep zerolog import used
)

func main() {
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx); err != nil {
		log.Fatal().Err(err).Msg("finding-service exited with error")
	}
}

func run(ctx context.Context) error {
	httpPort := envOr("HTTP_PORT", "8085")
	grpcPort := envOr("GRPC_PORT", "50055")
	dsn := envOr("DATABASE_URL", envOr("POSTGRES_DSN", ""))

	log.Info().
		Str("http_port", httpPort).
		Str("grpc_port", grpcPort).
		Msg("finding-service starting")

	// ── Database ──────────────────────────────────────────────
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL or POSTGRES_DSN env var is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("pgxpool connect: %w", err)
	}
	defer pool.Close()

	// Ping DB to verify connectivity
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("pgxpool ping: %w", err)
	}
	log.Info().Msg("database connected")

	// ── Repositories ──────────────────────────────────────────
	findingRepo := postgres.NewFindingRepo(pool)
	raRepo := postgres.NewRiskAcceptanceRepo(pool)
	memberRepo := postgres.NewMemberRepo(pool)
	reportRepo := postgres.NewReportRepo(pool)
	engagementRepo := postgres.NewEngagementRepo(pool)
	noteRepo := postgres.NewNoteRepo(pool)

	// ── Use cases ─────────────────────────────────────────────
	// NewStatusTransition accepts an optional NATS publisher (nil = disabled)
	transition := ucfinding.NewStatusTransition(findingRepo, nil)
	// statsFindingAdapter bridges FindingRepo to StatsUseCase interface
	statsAdapter := findingpkg.NewStatsFindingAdapter(findingRepo)
	statsUC := stats_uc.NewStatsUseCase(statsAdapter)

	// Engagement use cases
	getOrCreateUC := enguc.NewGetOrCreate(engagementRepo, nil)
	closeEngUC := enguc.NewClose(engagementRepo, nil)

	// ── HTTP handlers ─────────────────────────────────────────
	findingHandler := deliveryhttp.NewFindingHandler(findingRepo, transition, log.Logger)
	internalHandler := deliveryhttp.NewInternalHandler(statsUC)
	engagementHandler := deliveryhttp.NewEngagementHandler(engagementRepo, getOrCreateUC, closeEngUC, log.Logger)

	createRAUC := rauc.NewCreate(raRepo, memberRepo, findingRepo, nil)
	removeRAUC := rauc.NewRemoveFinding(raRepo, memberRepo, findingRepo)
	riskAcceptanceHandler := deliveryhttp.NewRiskAcceptanceHandler(createRAUC, removeRAUC).WithRepo(raRepo)

	// Wire BulkHandler — enables POST /findings/bulk/close, /bulk/reopen, /bulk/assign
	bulkUC := ucfinding.NewBulkUpdate(findingRepo, nil)
	bulkHandler := deliveryhttp.NewBulkHandler(bulkUC, findingRepo, nil, log.Logger)

	// Wire NoteHandler — enables GET/POST /findings/{id}/notes
	noteHandler := deliveryhttp.NewNoteHandler(findingRepo, noteRepo)

	// report is mostly disabled in standalone mode without full wiring
	var generateReportUC *reportuc.GenerateUseCase
	reportHandler := deliveryhttp.NewReportHandler(generateReportUC, reportRepo, nil)

	router := deliveryhttp.NewRouter(
		findingHandler,
		bulkHandler,   // bulk ✓
		noteHandler,   // note ✓
		engagementHandler, // engagement ✓
		nil,              // test
		nil,              // member
		nil,              // tool
		reportHandler,    // report
		riskAcceptanceHandler, // riskAcceptance
		internalHandler,  // internal — stats, risk-trend, product-grades, sla-breaches
		nil,              // sla
		nil,              // product
		nil,              // productSeed
		nil,              // findingSeed
		nil,              // findingGroup
		log.Logger,
	)

	// ── HTTP server ───────────────────────────────────────────
	httpSrv := &http.Server{
		Addr:         ":" + httpPort,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// ── gRPC server ───────────────────────────────────────────
	grpcLis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		return fmt.Errorf("gRPC listen: %w", err)
	}
	grpcSrv := grpc.NewServer()
	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(grpcSrv, healthSvc)
	healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// ── Start both servers ────────────────────────────────────
	errCh := make(chan error, 2)

	go func() {
		log.Info().Str("addr", httpSrv.Addr).Msg("HTTP server starting")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTP server: %w", err)
		}
	}()

	go func() {
		log.Info().Str("addr", ":"+grpcPort).Msg("gRPC server starting")
		if err := grpcSrv.Serve(grpcLis); err != nil {
			errCh <- fmt.Errorf("gRPC server: %w", err)
		}
	}()

	log.Info().
		Str("http", ":"+httpPort).
		Str("grpc", ":"+grpcPort).
		Msg("finding-service ready")

	// ── Wait for shutdown ─────────────────────────────────────
	select {
	case <-ctx.Done():
		log.Info().Msg("shutdown signal received")
	case err := <-errCh:
		return err
	}

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()

	log.Info().Msg("shutting down finding-service...")
	grpcSrv.GracefulStop()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("HTTP shutdown error")
	}

	log.Info().Msg("finding-service stopped")
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

