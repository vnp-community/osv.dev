// Command vulnerability-service is the unified Vulnerability Data Platform.
// Consolidates: cve-service + kev-service + taxonomy-service + alias-relations.
//
// Exposed gRPC services (backward-compatible names retained):
//   - CVEService       (from cve-service)
//   - VulnerabilityService (new unified name)
//
// NATS subscriptions:
//   - osv.vuln.imported           → alias group detection
//   - osv.ai.enrichment.completed → alias embedding update
//
// NATS publications:
//   - osv.vuln.imported, osv.vuln.updated, osv.vuln.withdrawn
package main

import (
	"context"
	"net"
	"net/http"
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

	grpcPort := envOr("GRPC_PORT", "50053")
	httpPort := envOr("HTTP_PORT", "8080")

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen gRPC")
	}

	s := grpc.NewServer()
	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(s, healthSvc)
	healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// TODO: Register handlers once proto stubs are generated:
	// cvev1.RegisterCVEServiceServer(s, cveHandler)           // backward-compat
	// vulnv1.RegisterVulnerabilityServiceServer(s, cveHandler) // new name
	// Register KEV, Alias, Taxonomy methods as part of VulnerabilityService

	log.Info().
		Str("grpc_port", grpcPort).
		Str("http_port", httpPort).
		Strs("nats_consumed", []string{"osv.vuln.imported", "osv.ai.enrichment.completed"}).
		Msg("vulnerability-service starting")

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC serve failed")
		}
	}()

	// HTTP server for KEV REST endpoints (backward-compat)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	})
	// TODO: Register KEV HTTP handlers
	// mux.HandleFunc("/api/v1/kev/", kevHandler.ServeHTTP)

	httpSrv := &http.Server{Addr: ":" + httpPort, Handler: mux}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP serve failed")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down vulnerability-service")
	s.GracefulStop()
	httpSrv.Shutdown(context.Background()) //nolint:errcheck
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
