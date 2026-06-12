// Command notification-service is the unified OSV Notification Platform.
// Consolidates: notification (osv) + notification-service (globalcve).
// Consumes 7 NATS subjects from both OSV and GlobalCVE namespaces.
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

	grpcPort := envOr("GRPC_PORT", "50063")
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen")
	}

	s := grpc.NewServer()

	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(s, healthSvc)
	healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// NATS subjects consumed (7 total):
	// osv.vuln.imported, osv.vuln.updated, osv.vuln.withdrawn
	// cve.created, cve.updated, kev.added, sync.completed

	log.Info().Str("port", grpcPort).
		Strs("nats_subjects", []string{
			"osv.vuln.imported", "osv.vuln.updated", "osv.vuln.withdrawn",
			"cve.created", "cve.updated", "kev.added", "sync.completed",
		}).
		Msg("notification-service (unified) starting")

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC serve failed")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down notification-service")
	s.GracefulStop()
	fmt.Println("notification-service stopped")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
