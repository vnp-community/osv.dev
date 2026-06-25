// Command osv is the main entrypoint for the OSV.dev modular monolith.
// It initializes and runs all embedded microservices as goroutines.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/osv/apps/osv/internal/config"
	"github.com/osv/apps/osv/internal/orchestrator"
	"github.com/osv/shared/pkg/logger"
	"github.com/osv/shared/pkg/observability"
	"github.com/rs/zerolog"
)

func main() {
	logger.InitGlobalLogger()
	defer logger.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")

	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	shutdown := observability.MustSetup("osv-server", "dev", log)
	defer shutdown()

	if err := run(ctx, projectID); err != nil {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, projectID string) error {
	cfg := config.FromEnv()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config invalid: %w", err)
	}

	slog.InfoContext(ctx, "OSV modular monolith starting",
		slog.String("project", projectID),
		slog.String("mode", cfg.Mode),
	)

	// Wire and initialize all embedded services
	services := config.WireServices(cfg)

	slog.InfoContext(ctx, "registered embedded services",
		slog.Int("count", len(services)),
	)
	for _, s := range services {
		slog.InfoContext(ctx, "  service registered", slog.String("name", s.Name()))
	}

	// Start all services as goroutines
	supervisor := orchestrator.New(services...)
	return supervisor.Run(ctx)
}
