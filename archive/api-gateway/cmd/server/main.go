// cmd/server/main.go — API Gateway entry point (OpenVulnScan edition)
// Wires: GRPCAuthValidator → OVSAuthMiddleware → HTTPProxy → upstream services
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/osv/api-gateway/internal/domain/policy"
	gatewayauth "github.com/osv/api-gateway/internal/infra/auth"
	"github.com/osv/api-gateway/internal/infra/handlers"
	"github.com/osv/api-gateway/internal/infra/proxy"
	"github.com/osv/api-gateway/internal/infra/ratelimit"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Str("service", "api-gateway").Logger()

	// Redis client (shared: rate limiting + auth token cache)
	redisAddr := envStr("REDIS_ADDR", "localhost:6379")
	rc := redis.NewClient(&redis.Options{Addr: redisAddr, DB: 0})
	defer rc.Close()

	// Rate limiter
	limiter := ratelimit.NewRedisRateLimiter(rc)

	// ── OVS: gRPC Auth Validator ──────────────────────────────────────────
	authServiceAddr := envStr("AUTH_SERVICE_ADDR", "localhost:9001")
	authValidator, err := gatewayauth.NewGRPCAuthValidator(authServiceAddr, rc, log)
	if err != nil {
		log.Fatal().Err(err).Str("addr", authServiceAddr).Msg("failed to connect to auth-service gRPC")
	}
	log.Info().Str("addr", authServiceAddr).Msg("auth-service gRPC connected")

	// ── OVS: HTTP Proxy with circuit breakers ─────────────────────────────
	upstreamURLs := map[string]string{
		"auth-service":         envStr("AUTH_SERVICE_URL",         "http://auth-service:9101"),
		"scan-service":         envStr("SCAN_SERVICE_URL",         "http://scan-service:9102"),
		"asset-service":        envStr("ASSET_SERVICE_URL",        "http://asset-service:9103"),
		"cve-service":          envStr("CVE_SERVICE_URL",          "http://cve-service:9104"),
		"agent-service":        envStr("AGENT_SERVICE_URL",        "http://agent-service:9105"),
		"schedule-service":     envStr("SCHEDULE_SERVICE_URL",     "http://schedule-service:9106"),
		"report-service":       envStr("REPORT_SERVICE_URL",       "http://report-service:9107"),
		"notification-service": envStr("NOTIFICATION_SERVICE_URL", "http://notification:8080"),
	}
	httpProxy, err := proxy.NewHTTPProxy(proxy.OVSRoutes, upstreamURLs, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create HTTP proxy")
	}
	log.Info().Int("upstreams", len(upstreamURLs)).Msg("HTTP proxy initialized")

	// ── OVS: Auth Middleware ───────────────────────────────────────────────
	skipPaths := []string{
		"/health/live", "/health/ready", "/metrics",
		"/.well-known/jwks.json", "/api/v1/auth",
	}
	authMW := handlers.NewOVSAuthMiddleware(authValidator, httpProxy, skipPaths, log)

	// Routing policy (existing OSV routes — kept for backward compat)
	routes := policy.DefaultRoutes()

	// gRPC reverse proxy (existing OSV gRPC services)
	grpcProxy := proxy.NewGRPCProxy(routes, log)

	// ── gRPC Server ───────────────────────────────────────────────────────
	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			recoveryInterceptor(log),
			tracingInterceptor(),
			loggingInterceptor(log),
			rateLimitInterceptor(limiter, log),
		),
	)
	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(grpcSrv, healthSvc)
	_ = grpcProxy // registered when proto-gen stubs exist

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", envStr("GRPC_PORT", "50051")))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen gRPC")
	}

	go func() {
		log.Info().Str("port", envStr("GRPC_PORT", "50051")).Msg("gRPC server starting")
		if err := grpcSrv.Serve(lis); err != nil {
			log.Error().Err(err).Msg("gRPC server error")
		}
	}()

	// ── HTTP Server (OVS auth middleware + health endpoints) ──────────────
	mux := http.NewServeMux()

	// Health & metrics (bypass auth middleware)
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","time":"%s"}`, time.Now().UTC().Format(time.RFC3339))
	})
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := rc.Ping(r.Context()).Err(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"not_ready","reason":"redis unreachable"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ready"}`)
	})

	// All /api/v1/* routes go through OVS auth middleware → proxy
	mux.Handle("/api/v1/", authMW)

	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%s", envStr("HTTP_PORT", "8080")),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("port", envStr("HTTP_PORT", "8080")).Msg("HTTP server starting")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP server error")
		}
	}()

	// ── Graceful Shutdown ─────────────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	log.Info().Msg("shutting down api-gateway...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	grpcSrv.GracefulStop()
	httpSrv.Shutdown(ctx) //nolint:errcheck
	log.Info().Msg("shutdown complete")
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Stub interceptors — expanded in pkg/middleware
func recoveryInterceptor(log zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(ctx, req)
	}
}

func tracingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(ctx, req)
	}
}

func loggingInterceptor(log zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		log.Info().
			Str("method", info.FullMethod).
			Dur("latency_ms", time.Since(start)).
			Err(err).
			Msg("grpc call")
		return resp, err
	}
}

func rateLimitInterceptor(limiter *ratelimit.RedisRateLimiter, log zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(ctx, req)
	}
}
