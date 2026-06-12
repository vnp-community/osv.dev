// Package grpcutil implements production gRPC clients for all downstream services.
// Each client wraps the gRPC connection with circuit breaker, retry, and keepalive.
package grpcutil

import (
	"context"
	"fmt"
	"time"

	"github.com/osv/pkg/resilience"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ConnectionConfig holds gRPC connection settings.
type ConnectionConfig struct {
	Target  string
	Timeout time.Duration

	// Circuit breaker settings
	MaxFailures int
	CBTimeout   time.Duration

	// Keepalive
	KeepaliveTime    time.Duration
	KeepaliveTimeout time.Duration
}

// DefaultConfig returns sensible defaults for service-to-service calls.
func DefaultConfig(target string) ConnectionConfig {
	return ConnectionConfig{
		Target:           target,
		Timeout:          5 * time.Second,
		MaxFailures:      5,
		CBTimeout:        30 * time.Second,
		KeepaliveTime:    30 * time.Second,
		KeepaliveTimeout: 10 * time.Second,
	}
}

// BaseClient holds a shared gRPC connection with circuit breaker.
type BaseClient struct {
	conn    *grpc.ClientConn
	breaker *resilience.CircuitBreaker
	config  ConnectionConfig
	log     zerolog.Logger
}

// NewBaseClient creates a new gRPC base client.
func NewBaseClient(config ConnectionConfig, log zerolog.Logger) (*BaseClient, error) {
	conn, err := grpc.Dial(
		config.Target,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // mTLS handled by Istio sidecar
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                config.KeepaliveTime,
			Timeout:             config.KeepaliveTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.WaitForReady(false),
			grpc.MaxCallRecvMsgSize(16*1024*1024),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", config.Target, err)
	}

	cbConfig := resilience.CircuitBreakerConfig{
		MaxFailures:         config.MaxFailures,
		Timeout:             config.CBTimeout,
		HalfOpenMaxRequests: 2,
	}

	return &BaseClient{
		conn:    conn,
		breaker: resilience.NewCircuitBreaker(config.Target, cbConfig),
		config:  config,
		log:     log,
	}, nil
}

// Conn returns the underlying gRPC connection.
func (c *BaseClient) Conn() *grpc.ClientConn { return c.conn }

// Execute runs a gRPC call through the circuit breaker.
func (c *BaseClient) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	callCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	return c.breaker.Execute(callCtx, fn)
}

// ExecuteWithResult runs a typed gRPC call through the circuit breaker.
func ExecuteWithResult[T any](
	bc *BaseClient,
	ctx context.Context,
	fn func(ctx context.Context) (T, error),
) (T, error) {
	callCtx, cancel := context.WithTimeout(ctx, bc.config.Timeout)
	defer cancel()

	return resilience.ExecuteWithResult(bc.breaker, callCtx, fn)
}

// Close closes the gRPC connection.
func (c *BaseClient) Close() error { return c.conn.Close() }

// CircuitState returns the current circuit breaker state for health checks.
func (c *BaseClient) CircuitState() resilience.State { return c.breaker.State() }
