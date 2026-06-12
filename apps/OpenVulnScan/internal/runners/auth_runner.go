// Package runners — auth_runner.go
// AuthRunner chạy auth-service như một goroutine với gRPC server trên bufconn.
//
// Giải pháp cho Go "internal" restriction:
// auth-service chứa tất cả logic trong internal/ package.
// Chúng ta không thể import internal/ từ module khác.
// Cách giải quyết: spawn auth-service cmd/server như subprocess độc lập
// hoặc sử dụng cmd/server logic được exposed qua public interface.
//
// Trong phase này, AuthRunner implement gRPC bridge dùng shared/proto
// và JWT/crypto operations không phụ thuộc internal auth-service packages.
package runners

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	// shared/proto — giao diện gRPC công khai
	sharedauthv1 "github.com/osv/shared/proto/gen/go/auth/v1"

	"github.com/osv/apps/openvulnscan/internal/transport"
)

// AuthRunnerConfig chứa tất cả config cần thiết cho auth goroutine.
type AuthRunnerConfig struct {
	DBURL             string
	RedisURL          string
	JWTPrivateKeyPath string
	JWTIssuer         string
	JWTAudience       []string
	JWTAccessTTL      time.Duration
	JWTRefreshTTL     time.Duration
	GoogleClientID    string
	GoogleSecret      string
	GoogleRedirectURL string
	GitHubClientID    string
	GitHubSecret      string
	GitHubRedirectURL string
}

// AuthRunner implements app.ServiceRunner cho auth-service.
// Sử dụng gRPC server với shared/proto interface.
type AuthRunner struct {
	cfg    AuthRunnerConfig
	lis    *bufconn.Listener
	server *grpc.Server
	log    zerolog.Logger

	// Shared state cho health check
	dbPool *pgxpool.Pool
	rdb    *redis.Client
}

// NewAuthRunner tạo AuthRunner mới.
func NewAuthRunner(cfg AuthRunnerConfig, lis *bufconn.Listener) *AuthRunner {
	return &AuthRunner{
		cfg: cfg,
		lis: lis,
		log: log.With().Str("runner", "auth-service").Logger(),
	}
}

func (r *AuthRunner) Name() string { return "auth-service" }

// Run khởi động auth-service goroutine.
func (r *AuthRunner) Run(ctx context.Context) error {
	r.log.Info().Msg("initializing...")

	// PostgreSQL
	db, err := pgxpool.New(ctx, r.cfg.DBURL)
	if err != nil {
		return fmt.Errorf("auth: db: %w", err)
	}
	defer db.Close()
	r.dbPool = db

	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("auth: db ping: %w", err)
	}
	r.log.Info().Msg("postgres connected")

	// Redis
	redisOpt, err := redis.ParseURL(r.cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("auth: redis url: %w", err)
	}
	rdb := redis.NewClient(redisOpt)
	defer rdb.Close()
	r.rdb = rdb

	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("auth: redis ping: %w", err)
	}
	r.log.Info().Msg("redis connected")

	// auth-service bridge — delegate auth logic
	// (auth-service internal packages exposed via bridge pattern)
	bridge := newAuthBridge(r.cfg, db, rdb, r.log)
	if err := bridge.init(); err != nil {
		return fmt.Errorf("auth: bridge init: %w", err)
	}

	// gRPC server
	r.server = grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcRecoveryInterceptor, grpcLoggingInterceptor),
	)
	sharedauthv1.RegisterAuthServiceServer(r.server, bridge)

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(r.server, healthSrv)

	errCh := make(chan error, 1)
	go func() {
		r.log.Info().Msg("gRPC server ready on bufconn")
		errCh <- r.server.Serve(r.lis)
	}()

	select {
	case <-ctx.Done():
		r.server.GracefulStop()
		return nil
	case err := <-errCh:
		return wrapRunnerError("auth-service", err)
	}
}

// Health kiểm tra service.
func (r *AuthRunner) Health(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	conn, err := transport.DialBufConn(hctx, r.lis)
	if err != nil {
		return fmt.Errorf("auth health: %w", err)
	}
	defer conn.Close()

	hc := grpc_health_v1.NewHealthClient(conn)
	resp, err := hc.Check(hctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("auth health: %w", err)
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("auth not serving: %s", resp.Status)
	}
	return nil
}

// Listener returns the bufconn listener.
func (r *AuthRunner) Listener() *bufconn.Listener { return r.lis }
