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

	<-ctx.Done()
	log.Info().Msg("shutting down scan-service")
	s.GracefulStop()
	fmt.Println("scan-service stopped")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
