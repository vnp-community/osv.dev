// Package main is the entry point for the scanner service.
package main

import (
	"context"
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

	"github.com/osv/scanner/internal/checkers"
	_ "github.com/osv/scanner/internal/checkers"
	"github.com/osv/scanner/internal/domain/service"
	grpchandler "github.com/osv/scanner/internal/adapter/grpc/handler"
	"github.com/osv/scanner/internal/infrastructure/config"

	pb "github.com/osv/proto/gen/go/scanner/v1"
)

func main() {
	cfg := config.Load()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Str("service", "scanner").Logger()

	switch cfg.LogLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Build all registered checkers via init() auto-registration
	compiled, err := checkers.BuildAll()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to build checkers")
	}

	checkerSvc := service.NewCheckerService(compiled)
	log.Info().
		Int("checkers", checkerSvc.CheckerCount()).
		Msg("checkers loaded")

	// Create gRPC handler
	scannerHandler := grpchandler.New(checkerSvc, log.Logger)

	// gRPC server
	addr := net.JoinHostPort("0.0.0.0", "50054")
	if envPort := os.Getenv("SCANNER_GRPC_PORT"); envPort != "" {
		addr = net.JoinHostPort("0.0.0.0", envPort)
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Str("addr", addr).Msg("failed to listen")
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(100<<20), // 100MB
		grpc.MaxSendMsgSize(100<<20),
	)
	pb.RegisterScannerServiceServer(grpcServer, scannerHandler)

	// Health check
	healthSvc := health.NewServer()
	healthSvc.SetServingStatus("scanner.v1.ScannerService", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthSvc)

	// Reflection (dev tools)
	reflection.Register(grpcServer)

	log.Info().Str("addr", addr).Msg("scanner gRPC server starting")

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Error().Err(err).Msg("gRPC server error")
		}
	}()

	<-stop
	log.Info().Msg("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout())
	defer cancel()
	_ = ctx

	grpcServer.GracefulStop()
	log.Info().Msg("scanner service stopped")
}
