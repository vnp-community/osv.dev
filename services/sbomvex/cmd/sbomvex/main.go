// Package main is the entry point for the sbomvex gRPC service.
package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/osv/sbomvex/internal/adapter/grpc/handler"

	pb "github.com/osv/proto/gen/go/sbomvex/v1"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Str("service", "sbomvex").Logger()

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50055"
	}

	addr := net.JoinHostPort("0.0.0.0", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Str("addr", addr).Msg("failed to listen")
	}

	sbomvexHandler := handler.New(log.Logger)

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(100<<20), // 100MB for large SBOM files
		grpc.MaxSendMsgSize(100<<20),
	)
	pb.RegisterSBOMVEXServiceServer(grpcServer, sbomvexHandler)

	healthSvc := health.NewServer()
	healthSvc.SetServingStatus("sbomvex.v1.SBOMVEXService", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthSvc)
	reflection.Register(grpcServer)

	log.Info().Str("addr", addr).Msg("sbomvex gRPC server starting")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Error().Err(err).Msg("gRPC server error")
		}
	}()

	<-stop
	log.Info().Msg("shutting down sbomvex service")
	grpcServer.GracefulStop()
	log.Info().Msg("sbomvex service stopped")
}
