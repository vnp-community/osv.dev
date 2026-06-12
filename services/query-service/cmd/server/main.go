// Command query-service is the unified Vulnerability Query Platform.
// Consolidates: vulnerability-query + query-service + ranking-service + browse-service.
//
// Exposed gRPC: QueryService (port 50055)
// HTTP: Browse endpoints /api/v1/cpe/{type}/vendors, /api/v1/cpe/{type}/products/{vendor}
//
// Ranking logic (from ranking-service) is inlined into query response pipeline.
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

	grpcPort := envOr("GRPC_PORT", "50055")
	httpPort := envOr("HTTP_PORT", "8081")

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen gRPC")
	}
	s := grpc.NewServer()
	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(s, healthSvc)
	healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	// TODO: queryv1.RegisterQueryServiceServer(s, queryHandler)

	// HTTP for browse endpoints
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	})
	// TODO: mux.Handle("/api/v1/cpe/", browseHandler)

	httpSrv := &http.Server{Addr: ":" + httpPort, Handler: mux}

	log.Info().Str("grpc_port", grpcPort).Str("http_port", httpPort).
		Msg("query-service starting — vulnerability-query + browse + ranking inlined")

	go func() { s.Serve(lis) }()                                            //nolint:errcheck
	go func() { httpSrv.ListenAndServe() }()                                 //nolint:errcheck

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
