// Command scan-service is the unified entrypoint for the Scan Platform.
// It consolidates scan-service, agent-service, asset-service, scanner, and sbomvex.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"net/http"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	httpdelivery "github.com/osv/scan-service/internal/delivery/http"
)

func main() {
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	grpcPort := envOr("GRPC_PORT", "50058")
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen")
	}

	s := grpc.NewServer()

	// Register health service
	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(s, healthSvc)
	healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// TODO: Register domain services once proto stubs are generated:
	// pb.RegisterScanServiceServer(s, scanHandler)
	// pb.RegisterAgentServiceServer(s, agentHandler)
	// pb.RegisterAssetServiceServer(s, assetHandler)

	log.Info().Str("port", grpcPort).Msg("scan-service (unified) starting")

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC serve failed")
		}
	}()

	// Start HTTP Server
	httpPort := envOr("HTTP_PORT", "8082")
	
	// Create handlers (dependencies are nil for now until fully wired)
	var importHandler *httpdelivery.ImportHandler
	var parserHandler *httpdelivery.ParserHandler
	var agentHandler  *httpdelivery.AgentHandler = httpdelivery.NewAgentHandler(nil, log.Logger)

	router := httpdelivery.NewRouter(importHandler, parserHandler, agentHandler, log.Logger)
	httpSrv := &http.Server{
		Addr:    ":" + httpPort,
		Handler: router,
	}

	log.Info().Str("port", httpPort).Msg("scan-service HTTP starting")
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP serve failed")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down scan-service")
	
	// Shutdown HTTP Server gracefully
	httpSrv.Shutdown(context.Background())

	s.GracefulStop()
	fmt.Println("scan-service stopped")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
