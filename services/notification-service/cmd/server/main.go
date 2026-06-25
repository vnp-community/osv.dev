// Command notification-service is the unified OSV Notification Platform.
// Consolidates: notification (osv) + notification-service (globalcve).
// Consumes NATS subjects from both OSV and GlobalCVE namespaces.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/osv/notification-service/internal/alertrepo"
	deliverhttp "github.com/osv/notification-service/internal/delivery/http"
	"github.com/osv/notification-service/internal/broker"
	"github.com/osv/notification-service/internal/infra/persistence/postgres"
	infrapostgres "github.com/osv/notification-service/internal/infra/postgres"
	mynats "github.com/osv/notification-service/internal/nats"
	"github.com/osv/notification-service/internal/scheduler"
	"github.com/osv/notification-service/internal/usecase"
	"github.com/osv/shared/pkg/observability"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	// [FIX BUG-006] SERVICE_VERSION env var set by CI/CD
	version := envOr("SERVICE_VERSION", "dev")

	log := observability.InitLogger("notification-service", version)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	metrics := observability.NewCommonMetrics("notification-service")
	_ = metrics
	// [FIX BUG-005] METRICS_PORT env var — default 9094 for notification-service per port map
	metricsPort := parseInt(envOr("METRICS_PORT", "9094"), 9094)
	observability.StartMetricsServer(metricsPort)

	shutdown, err := observability.InitTracer(ctx, "notification-service", version) // [FIX BUG-006]
	if err != nil {
		log.Warn().Err(err).Msg("tracing init failed, continuing without tracing")
	}
	defer shutdown()

	// Connect to Database
	// Priority: NOTIFICATION_DATABASE_URL > DATABASE_URL > POSTGRES_DSN
	// Credentials are NOT embedded in source — must be provided via env vars.
	dbURL := envOr("NOTIFICATION_DATABASE_URL", envOr("DATABASE_URL", os.Getenv("POSTGRES_DSN")))
	if dbURL == "" {
		// Build DSN from individual parts — credentials must come from env, never hardcoded
		pgHost := envOr("POSTGRES_HOST", "localhost")
		pgPort := envOr("POSTGRES_PORT", "5432")
		pgDB := envOr("POSTGRES_DB", "osvdb")
		pgUser := os.Getenv("POSTGRES_USER")
		pgPass := os.Getenv("POSTGRES_PASSWORD")
		if pgUser == "" || pgPass == "" {
			log.Fatal().Msg("database credentials not configured: set POSTGRES_DSN or POSTGRES_USER + POSTGRES_PASSWORD env vars")
		}
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			pgUser, pgPass, pgHost, pgPort, pgDB)
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to postgres")
	}
	defer pool.Close()

	// Connect to Redis
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
		log.Warn().Str("fallback", redisURL).Msg("REDIS_URL not set, using localhost fallback — configure in production")
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse redis URL")
	}
	redisClient := redis.NewClient(opt)
	defer redisClient.Close()

	// Connect to NATS
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
		log.Warn().Str("fallback", natsURL).Msg("NATS_URL not set, using localhost fallback — configure in production")
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		if os.Getenv("NATS_ENABLED") == "true" {
			log.Fatal().Err(err).Msg("failed to connect to nats")
		} else {
			log.Warn().Err(err).Msg("NATS disabled or unreachable, continuing without it")
			nc = nil
		}
	} else {
		defer nc.Close()
	}

	// Initialize Repositories
	webhookRepo := infrapostgres.NewWebhookRepository(pool)
	subRepo := infrapostgres.NewSubscriptionRepository(pool)

	// Initialize UseCases
	registerUC := usecase.NewRegisterWebhookUseCase(webhookRepo)
	deliverer := usecase.NewWebhookDeliverer(webhookRepo, redisClient)
	dispatcher := usecase.NewAlertDispatcher(webhookRepo, subRepo, deliverer)

	// Background Workers
	if nc != nil {
		subscriber := mynats.NewSubscriber(nc, dispatcher, log, nil)
		if err := subscriber.Start(ctx); err != nil {
			log.Fatal().Err(err).Msg("failed to start nats subscriber")
		}
	}

	retryWorker := scheduler.NewRetryWorker(webhookRepo, deliverer, log)
	go retryWorker.Run(ctx)

	// Initialize HTTP Handlers & Router
	whHandler := deliverhttp.NewWebhookHandler(registerUC, deliverer, webhookRepo)
	shHandler := deliverhttp.NewSubscriptionHandler(subRepo)
	ihHandler := deliverhttp.NewInternalHandler(dispatcher)
	ruleRepo := postgres.NewRuleRepo(pool, log)
	rhHandler := deliverhttp.NewRuleHandler(ruleRepo, log)
	// CR-009: DeliveryHandler — queries webhook_deliveries for flat list, retry, hourly stats
	dhHandler := deliverhttp.NewDeliveryHandler(pool)

	// TASK-003 FIX: Wire AlertsHandler so /api/v1/notifications/* routes are mounted.
	// Previously ah=nil caused the if-nil guard to skip all notification routes.
	alertAdapter := alertrepo.New(pool)
	ahHandler := deliverhttp.NewAlertsHandler(alertAdapter)

	// Wire SSEHandler for real-time notification streaming.
	evtBroker := broker.New()
	sseHandler := deliverhttp.NewSSEHandler(evtBroker, nil) // tokenSvc=nil: auth via X-User-ID from gateway

	router := deliverhttp.SetupRouter(whHandler, shHandler, ihHandler, ahHandler, sseHandler, rhHandler, dhHandler)

	httpPort := envOr("NOTIFICATION_HTTP_PORT", envOr("HTTP_PORT", "8086"))
	httpSrv := &http.Server{
		Addr:    ":" + httpPort,
		Handler: router,
	}

	go func() {
		log.Info().Str("port", httpPort).Msg("starting HTTP API")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http serve failed")
		}
	}()

	// gRPC Server setup
	grpcPort := envOr("NOTIFICATION_GRPC_PORT", envOr("GRPC_PORT", "50063"))
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen")
	}

	s := grpc.NewServer()
	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(s, healthSvc)
	healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	go func() {
		log.Info().Str("port", grpcPort).Msg("starting gRPC API")
		if err := s.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC serve failed")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down notification-service")
	httpSrv.Shutdown(context.Background())
	s.GracefulStop()
	fmt.Println("notification-service stopped")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// parseInt parses a string as int, returning def on error.
func parseInt(s string, def int) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n > 0 {
		return n
	}
	return def
}
