// Package main is the entry point for the reporter gRPC service.
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

	"github.com/osv/report-service/internal/adapter/grpc/handler"
	"github.com/osv/report-service/internal/domain/entity"
	consolefmt "github.com/osv/report-service/internal/formatters/console"
	csvfmt "github.com/osv/report-service/internal/formatters/csv"
	jsonfmt "github.com/osv/report-service/internal/formatters/json"
	htmlfmt "github.com/osv/report-service/internal/formatters/html"
	pdffmt "github.com/osv/report-service/internal/formatters/pdf"
	"github.com/osv/report-service/internal/formatters"
	"github.com/osv/report-service/internal/usecase/generatereport"

	pb "github.com/osv/proto/gen/go/reporter/v1"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Str("service", "reporter").Logger()

	// Build formatter registry
	htmlFormatter, err := htmlfmt.New()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init HTML formatter")
	}

	registry := formatters.Registry{
		entity.FormatConsole: consolefmt.New(),
		entity.FormatCSV:     csvfmt.New(),
		entity.FormatJSON:    jsonfmt.New(),
		entity.FormatJSON2:   jsonfmt.NewJSON2(),
		entity.FormatHTML:    htmlFormatter,
		entity.FormatPDF:     pdffmt.New(),
	}

	// Wire use cases
	generateReportUC := generatereport.NewUseCase(registry)

	// gRPC handler
	reporterHandler := handler.New(generateReportUC, log.Logger)

	// gRPC server
	port := os.Getenv("REPORTER_GRPC_PORT")
	if port == "" {
		port = "50054"
	}
	addr := net.JoinHostPort("0.0.0.0", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Str("addr", addr).Msg("failed to listen")
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(50<<20), // 50MB
		grpc.MaxSendMsgSize(50<<20),
	)
	pb.RegisterReporterServiceServer(grpcServer, reporterHandler)

	healthSvc := health.NewServer()
	healthSvc.SetServingStatus("reporter.v1.ReporterService", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthSvc)
	reflection.Register(grpcServer)

	log.Info().Str("addr", addr).Msg("reporter gRPC server starting")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Error().Err(err).Msg("gRPC server error")
		}
	}()

	<-stop
	log.Info().Msg("shutting down reporter service")
	grpcServer.GracefulStop()
	log.Info().Msg("reporter service stopped")
}
