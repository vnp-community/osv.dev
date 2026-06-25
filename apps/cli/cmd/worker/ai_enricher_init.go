// ai_enricher_init.go activates AIEnricher when AI_ENRICHER_ADDR is set.
// This file is intentionally separate from main.go to keep the addition minimal.
//
// Usage: call maybeRegisterAIEnricher() in cmd/worker/main.go after existing pipeline setup.
package main

import (
	"log/slog"
	"os"

	"github.com/osv/apps/cli/internal/worker/pipeline/registry"
	"github.com/osv/apps/cli/internal/worker/pipeline"
)

// maybeRegisterAIEnricher appends AIEnricher to registry.List when AI_ENRICHER_ADDR is set.
// Call this function in main() AFTER existing registry initialization.
// Does nothing if AI_ENRICHER_ADDR is not set (backward compatible).
func maybeRegisterAIEnricher() {
	addr := os.Getenv("AI_ENRICHER_ADDR")
	if addr == "" {
		return // AI enricher not configured — keep existing pipeline unchanged
	}

	enricher, err := pipeline.NewAIEnricher(addr)
	if err != nil {
		slog.Warn("AI enricher init failed (skipping)",
			slog.String("addr", addr),
			slog.Any("error", err))
		return
	}

	// Append to existing registry list — non-destructive
	registry.List = append(registry.List, enricher)
	slog.Info("AI enricher registered", slog.String("addr", addr))
}
