// Command ingestion-service is the unified Data Ingestion Platform.
// Consolidates: ingestion + source-sync + ingest-service + cve-sync-service + converter.
//
// Run modes (RUN_MODE env var):
//
//	pipeline — GCS blob pipeline processor
//	sync     — external source sync (circl, nvd, pypi, ids)
//	fetch    — scheduled fetchers (EPSS, CAPEC, CWE, CPE cache)
//	grpc     — DataSyncService gRPC server
//	all      — all workers (default, for development)
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	mode := envOr("RUN_MODE", "all")
	log.Info().Str("mode", mode).Msg("ingestion-service starting")

	switch mode {
	case "pipeline":
		runPipeline(ctx)
	case "sync":
		runSync(ctx)
	case "fetch":
		runFetchers(ctx)
	case "grpc":
		runGRPC(ctx)
	default: // "all"
		go runPipeline(ctx)
		go runSync(ctx)
		go runFetchers(ctx)
		runGRPC(ctx)
	}
}

func runGRPC(ctx context.Context) {
	port := envOr("GRPC_PORT", "50054")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen")
	}
	s := grpc.NewServer()
	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(s, healthSvc)
	healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	// TODO: Register DataSyncService handler
	// datasyncv1.RegisterDataSyncServiceServer(s, dataSyncHandler)
	log.Info().Str("port", port).Msg("gRPC DataSyncService listening")
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC serve failed")
		}
	}()
	<-ctx.Done()
	log.Info().Msg("shutting down ingestion-service gRPC")
	s.GracefulStop()
	fmt.Println("ingestion-service gRPC stopped")
}

func runPipeline(ctx context.Context) {
	log.Info().Msg("pipeline worker started — watching GCS blobs")
	// TODO: Wire GCS blob store + idempotency store + NATS publisher
	// pipeline.NewProcessor(blobStore, idempotencyStore, publisher).Run(ctx)
	<-ctx.Done()
}

func runSync(ctx context.Context) {
	log.Info().Msg("sync worker started — circl / nvd / pypi / ids")
	// TODO: Wire circl, nvd, pypi, ids sync connectors
	<-ctx.Done()
}

func runFetchers(ctx context.Context) {
	log.Info().Msg("fetch worker started — EPSS / CAPEC / CWE / CPE")
	// TODO: Wire EPSS, CAPEC, CWE, CPE fetchers with cron schedule
	<-ctx.Done()
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
