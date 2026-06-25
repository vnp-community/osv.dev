// backend_selector.go selects between GCP Pub/Sub and NATS backends
// based on the CLI_BACKEND environment variable.
//
// RULE: existing GCP backend code is NOT modified.
// When CLI_BACKEND=microservices, the NATSPublisher is used alongside
// (not instead of) the existing importer.Config.Publisher — it publishes
// to NATS after the existing GCP publish completes.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/osv/apps/cli/internal/importer"
)

// selectNATSPublisherIfNeeded returns a NATSPublisher when CLI_BACKEND=microservices.
// Returns (nil, false, nil) when GCP backend is active (default).
//
// Usage in main():
//
//	if natsPub, ok, err := selectNATSPublisherIfNeeded(ctx); err != nil {
//	    logger.FatalContext(ctx, "NATS publisher init failed", slog.Any("error", err))
//	} else if ok {
//	    defer natsPub.Close()
//	    config.NATSPublisher = natsPub
//	}
func selectNATSPublisherIfNeeded(ctx context.Context) (*importer.NATSPublisher, bool, error) {
	if os.Getenv("CLI_BACKEND") != "microservices" {
		return nil, false, nil // Use existing GCP publisher (default)
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	slog.InfoContext(ctx, "CLI_BACKEND=microservices: initialising NATS publisher",
		slog.String("nats_url", natsURL))

	pub, err := importer.NewNATSPublisher(natsURL)
	if err != nil {
		return nil, false, fmt.Errorf("nats publisher init: %w", err)
	}

	return pub, true, nil
}
