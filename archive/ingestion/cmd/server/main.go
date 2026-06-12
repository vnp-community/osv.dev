// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package main is the entry point for the Ingestion Service (Enterprise Edition).
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

	cloudFirestore "cloud.google.com/go/firestore"
	cloudStorage "cloud.google.com/go/storage"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	importcmd "github.com/osv/ingestion/internal/application/command/import_vulnerability"
	"github.com/osv/ingestion/internal/infra/idempotency/redis"
	firestorepkg "github.com/osv/ingestion/internal/infra/messaging/nats"
	"github.com/osv/ingestion/internal/infra/messaging/nats/consumer"
	natsinfra "github.com/osv/ingestion/internal/infra/messaging/nats"
	gcsinfra "github.com/osv/ingestion/internal/infra/storage/gcs"
	firestoreinfra "github.com/osv/ingestion/internal/infra/persistence/firestore"
	pkgconfig "github.com/osv/pkg/config"
	"github.com/osv/pkg/grpcutil"
	"github.com/osv/pkg/health"
	"github.com/osv/pkg/observability"
)

// Config holds the service configuration.
type Config struct {
	GRPC      struct { Port int }
	HTTP      struct { Port int }
	Firestore struct { ProjectID string; Collection string }
	GCS       struct { Bucket string; Prefix string }
	NATS      struct { URL string; StreamName string }
	Redis     struct { Addr string; Password string; DB int }
	Telemetry struct { OTLPEndpoint string; ServiceName string }
	Deletion  struct { SafetyThresholdPct float64 }
}

func main() {
	// Configure zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ── OTel ─────────────────────────────────────────────────────────────────
	zlog := zerolog.New(os.Stdout).With().Timestamp().Str("service", "ingestion").Logger()
	otelShutdown := observability.MustSetup("ingestion", "1.0.0", zlog)
	defer otelShutdown()

	cfg, err := pkgconfig.Load[Config]("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	if err := run(ctx, cfg, zlog); err != nil {
		log.Fatal().Err(err).Msg("service exited with error")
	}
}

func run(ctx context.Context, cfg *Config, zlog zerolog.Logger) error {
	log.Info().Str("service", "ingestion").Msg("starting")

	// === Infrastructure Clients ===

	// Firestore
	fsClient, err := cloudFirestore.NewClient(ctx, cfg.Firestore.ProjectID)
	if err != nil {
		return fmt.Errorf("firestore client: %w", err)
	}
	defer fsClient.Close()

	// GCS
	gcsClient, err := cloudStorage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("gcs client: %w", err)
	}
	defer gcsClient.Close()

	// Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	// NATS
	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	defer nc.Close()

	// === Build Infrastructure Adapters ===

	vulnWriter := firestoreinfra.NewVulnerabilityWriter(fsClient, cfg.Firestore.Collection)
	blobStore := gcsinfra.NewVulnerabilityBlobStore(gcsClient, cfg.GCS.Bucket, cfg.GCS.Prefix)
	idempotencyStore := redisinfra.NewIdempotencyStore(redisClient, 24*time.Hour)
	publisher, err := natsinfra.NewEventPublisher(nc)
	if err != nil {
		return fmt.Errorf("nats publisher: %w", err)
	}
	_ = firestorepkg.SourceChangeDetected{} // reference to avoid import error

	// === Build Application Handlers ===

	// findingRepo placeholder — would be a Firestore implementation
	importHandler := importcmd.NewHandler(vulnWriter, nil, blobStore, publisher, idempotencyStore)

	// === Start NATS Consumers ===

	sourceConsumer, err := consumer.NewSourceChangeConsumer(nc, importHandler)
	if err != nil {
		return fmt.Errorf("source change consumer: %w", err)
	}
	go func() {
		if err := sourceConsumer.Start(ctx, cfg.NATS.StreamName); err != nil {
			log.Error().Err(err).Msg("source change consumer error")
		}
	}()

	// === Start gRPC Server (Enterprise interceptor chain) ===

	grpcServer := grpc.NewServer(grpcutil.ServerOptions("ingestion", zlog, 30*time.Second)...)
	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
	if err != nil {
		return fmt.Errorf("grpc listen: %w", err)
	}
	go func() {
		log.Info().Int("port", cfg.GRPC.Port).Msg("gRPC server starting")
		if err := grpcServer.Serve(grpcLis); err != nil {
			log.Error().Err(err).Msg("gRPC server error")
		}
	}()

	// === Start HTTP Health Server (pkg/health MultiChecker) ===

	checker := health.NewMultiChecker(5*time.Second,
		health.RedisProber(redisClient),
		health.NATSProber(nc),
		health.NewFirestoreProber(func(ctx context.Context) error {
			_, err := fsClient.Collection("health").Doc("probe").Get(ctx)
			if err != nil && err.Error() != "rpc error: code = NotFound" {
				return err
			}
			return nil
		}),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", health.LiveHandler())
	mux.Handle("/health/ready", health.ReadyHandler(checker))

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler: mux,
	}
	go func() {
		log.Info().Int("port", cfg.HTTP.Port).Msg("HTTP health server starting")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP server error")
		}
	}()

	// === Wait for shutdown ===

	<-ctx.Done()
	log.Info().Msg("shutting down ingestion service")

	grpcServer.GracefulStop()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)

	return nil
}
