// Command product-management starts the DefectDojo Product Management microservice.
package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	pgdb "github.com/defectdojo/pkg/database/postgres"
	natsutil "github.com/defectdojo/pkg/nats"
	grpcserver "github.com/defectdojo/product-management/adapter/grpc/server"
	"github.com/defectdojo/product-management/adapter/http/handler"
	"github.com/defectdojo/product-management/infrastructure/postgres"
	engagement_uc "github.com/defectdojo/product-management/internal/usecase/engagement"
	product_uc "github.com/defectdojo/product-management/internal/usecase/product"
	test_uc "github.com/defectdojo/product-management/internal/usecase/test"
	productv1 "github.com/defectdojo/proto/product/v1"
)

func main() {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// ── Database ─────────────────────────────────────────────────────────────
	pool := pgdb.MustNewPool(ctx, &pgdb.Config{URL: mustEnv("DATABASE_URL")})
	defer pool.Close()

	// ── NATS JetStream ────────────────────────────────────────────────────────
	nc, err := natsutil.Connect(mustEnv("NATS_URL"))
	if err != nil {
		log.Fatal().Err(err).Msg("nats connect")
	}
	defer nc.Drain()

	js, err := natsutil.SetupStream(ctx, nc)
	if err != nil {
		log.Fatal().Err(err).Msg("nats setup stream")
	}
	pub := natsutil.NewPublisher(js, "product-management/v1")

	// ── Repositories ──────────────────────────────────────────────────────────
	productRepo := postgres.NewProductRepo(pool)
	productTypeRepo := postgres.NewProductTypeRepo(pool)
	engagementRepo := postgres.NewEngagementRepo(pool)
	testRepo := postgres.NewTestRepo(pool)

	// ── Use Cases ─────────────────────────────────────────────────────────────
	createProduct := product_uc.NewCreateProduct(productRepo, productTypeRepo, pub)
	getOrCreateProduct := product_uc.NewGetOrCreateProduct(productRepo, productTypeRepo, pub)
	getOrCreateEngagement := engagement_uc.NewGetOrCreate(engagementRepo, pub)
	closeEngagement := engagement_uc.NewClose(engagementRepo, pub)
	getOrCreateTest := test_uc.NewGetOrCreate(testRepo)

	// ── HTTP Server ───────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/health/ready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	productHandler := handler.NewProductHandler(createProduct, productRepo, log.Logger)
	productHandler.RegisterRoutes(r)

	httpAddr := envOr("HTTP_ADDR", ":8083")
	httpServer := &http.Server{Addr: httpAddr, Handler: r}

	go func() {
		log.Info().Str("addr", httpAddr).Msg("HTTP listening")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server")
		}
	}()

	// ── gRPC Server ───────────────────────────────────────────────────────────
	grpcAddr := envOr("GRPC_ADDR", ":9003")
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("gRPC listen")
	}

	grpcSrv := grpc.NewServer()
	productv1.RegisterProductServiceServer(grpcSrv, grpcserver.New(
		getOrCreateProduct,
		getOrCreateEngagement,
		getOrCreateTest,
		closeEngagement,
	))

	go func() {
		log.Info().Str("addr", grpcAddr).Msg("gRPC listening")
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC server")
		}
	}()

	// ── Graceful Shutdown ─────────────────────────────────────────────────────
	<-ctx.Done()
	log.Info().Msg("shutting down")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()

	_ = httpServer.Shutdown(shutCtx)
	grpcSrv.GracefulStop()

	log.Info().Msg("shutdown complete")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatal().Str("key", key).Msg("required env var not set")
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
