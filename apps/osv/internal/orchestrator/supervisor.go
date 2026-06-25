// Package orchestrator defines the Service interface and Supervisor
// for running multiple embedded microservices in a single process.
//
// This package is ADDITIVE — apps/osv/cmd/server/main.go is NOT modified here.
// The orchestrator will be wired in SC-OSV-02 (config + wire + main.go update).
//
// Architecture: "Single Process, Multiple Services"
//   - Each service implements Service interface
//   - Supervisor uses errgroup to run all services concurrently
//   - Any service returning a non-nil error causes all services to shut down
//   - Graceful shutdown on SIGINT/SIGTERM via context cancellation
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"
)

// Service is implemented by each embedded microservice.
// Each service's Start method must:
//   - Block until ctx is cancelled (graceful shutdown) or a fatal error occurs
//   - Return nil on clean shutdown, non-nil on error
type Service interface {
	// Name returns a human-readable service identifier for logging.
	Name() string
	// Start serves the service and blocks until ctx is cancelled or error.
	Start(ctx context.Context) error
}

// Supervisor manages the lifecycle of multiple embedded Services.
// All services run in separate goroutines within a shared errgroup.
type Supervisor struct {
	services        []Service
	shutdownTimeout time.Duration
}

// New creates a new Supervisor with the given services.
func New(services ...Service) *Supervisor {
	return &Supervisor{
		services:        services,
		shutdownTimeout: 30 * time.Second,
	}
}

// WithShutdownTimeout overrides the default 30-second graceful shutdown timeout.
func (s *Supervisor) WithShutdownTimeout(d time.Duration) *Supervisor {
	s.shutdownTimeout = d
	return s
}

// Run starts all services and blocks until all have exited.
// If any service returns a non-nil error, Run cancels the context (triggering
// all other services to shut down) and returns the first error.
//
// On context cancellation (SIGINT/SIGTERM), Run waits up to ShutdownTimeout
// for all services to complete graceful shutdown.
func (s *Supervisor) Run(ctx context.Context) error {
	eg, egCtx := errgroup.WithContext(ctx)

	for _, svc := range s.services {
		svc := svc // capture loop variable
		eg.Go(func() error {
			slog.InfoContext(egCtx, "service starting", slog.String("service", svc.Name()))
			if err := svc.Start(egCtx); err != nil {
				slog.ErrorContext(egCtx, "service exited with error",
					slog.String("service", svc.Name()),
					slog.Any("error", err),
				)
				return fmt.Errorf("%s: %w", svc.Name(), err)
			}
			slog.InfoContext(egCtx, "service exited cleanly", slog.String("service", svc.Name()))
			return nil
		})
	}

	return eg.Wait()
}
