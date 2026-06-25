package bff

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	grpcclient "github.com/osv/gateway-service/internal/adapter/grpcclient"
)

// DashboardData aggregates data from multiple services for the main dashboard.
type DashboardData struct {
	Findings    FindingsSummary `json:"findings"`
	Scans       ScansSummary    `json:"scans"`
	KEV         KEVSummary      `json:"kev"`
	AI          AISummary       `json:"ai"`
	GeneratedAt time.Time       `json:"generated_at"`
}

// FindingsSummary contains finding counts by severity.
type FindingsSummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Open     int `json:"open"`
	Resolved int `json:"resolved"`
}

// ScansSummary contains scan job statistics.
type ScansSummary struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// KEVSummary contains KEV (Known Exploited Vulnerabilities) stats.
type KEVSummary struct {
	Total          int `json:"total"`
	AffectedAssets int `json:"affected_assets"`
}

// AISummary contains AI enrichment coverage stats.
type AISummary struct {
	EnrichedCVEs int `json:"enriched_cves"`
	TotalCVEs    int `json:"total_cves"`
}

// DashboardAggregator aggregates data from multiple downstream services.
// All client fields are optional — if nil the aggregator returns graceful zero-values.
type DashboardAggregator struct {
	aiClient *grpcclient.AIClient // optional
	log      zerolog.Logger
}

// NewDashboardAggregator creates a DashboardAggregator.
// All client parameters are optional (pass nil to skip that data source).
func NewDashboardAggregator(
	aiClient *grpcclient.AIClient,
	log zerolog.Logger,
) *DashboardAggregator {
	return &DashboardAggregator{
		aiClient: aiClient,
		log:      log,
	}
}

// GetDashboard fetches and aggregates data for the main dashboard view.
// Uses errgroup for parallel calls. Each call degrades gracefully on error.
// The overall call never fails due to a downstream service being unavailable.
func (a *DashboardAggregator) GetDashboard(ctx context.Context) (*DashboardData, error) {
	// Total timeout for all parallel calls
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	g, gctx := errgroup.WithContext(ctx)

	var (
		findings FindingsSummary
		scans    ScansSummary
		kev      KEVSummary
		ai       AISummary
	)

	// ── Parallel call: AI summary (ai-service) ────────────────────────────────
	// Future: replace with real gRPC call when ai-service exposes GetStats RPC.
	if a.aiClient != nil {
		g.Go(func() error {
			// Placeholder: ai-service does not expose a stats RPC yet.
			// When available, call a.aiClient.GetStats(gctx) and populate ai.
			a.log.Debug().Msg("dashboard: ai stats placeholder")
			_ = gctx // suppress unused warning
			return nil
		})
	}

	// ── Additional sources (finding, scan, kev) ───────────────────────────────
	// These are populated by downstream services wired via proxy / future clients.
	// Pattern: add client field to DashboardAggregator and wire a g.Go() here.
	// Graceful degradation: log.Warn on error, continue with zero-value.
	_ = findings
	_ = scans
	_ = kev
	_ = ai

	// Wait for all goroutines — errors are non-fatal for dashboard
	if err := g.Wait(); err != nil {
		a.log.Warn().Err(err).Msg("dashboard: partial data due to downstream error")
	}

	return &DashboardData{
		Findings:    findings,
		Scans:       scans,
		KEV:         kev,
		AI:          ai,
		GeneratedAt: time.Now().UTC(),
	}, nil
}
