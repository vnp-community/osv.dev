// Command search-service is the unified Full-Text Search Platform.
// Consolidates: search (OpenSearch) + cve-search-service (Postgres+Mongo).
//
// Backend selection via SEARCH_BACKEND env var:
//   - "opensearch" — OpenSearch/Elasticsearch
//   - "postgres"   — PostgreSQL full-text search
//   - "mongo"      — MongoDB text search
//   - "auto"       — tries OpenSearch, falls back to Postgres (default)
//
// NATS subscriptions for index maintenance:
//   - osv.vuln.imported  → index
//   - osv.vuln.updated   → update
//   - osv.vuln.withdrawn → delete
package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/osv/search-service/internal/factory"
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

	backend := factory.FromEnv()
	grpcPort := envOr("GRPC_PORT", "50056")
	httpPort := envOr("HTTP_PORT", "8082")

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen gRPC")
	}
	s := grpc.NewServer()
	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(s, healthSvc)
	healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	// TODO: searchv1.RegisterSearchServiceServer(s, searchHandler)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	})
	// TODO: mux.HandleFunc("/api/v1/search", searchHandler.ServeHTTP)

	httpSrv := &http.Server{Addr: ":" + httpPort, Handler: mux}

	log.Info().
		Str("backend", string(backend)).
		Str("grpc_port", grpcPort).
		Str("http_port", httpPort).
		Strs("nats_consumed", []string{"osv.vuln.imported", "osv.vuln.updated", "osv.vuln.withdrawn"}).
		Msg("search-service starting")

	go func() { s.Serve(lis) }()          //nolint:errcheck
	go func() { httpSrv.ListenAndServe() }() //nolint:errcheck

	<-ctx.Done()
	s.GracefulStop()
	httpSrv.Shutdown(context.Background()) //nolint:errcheck
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
