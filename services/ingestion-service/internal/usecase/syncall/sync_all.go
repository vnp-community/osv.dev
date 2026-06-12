// Package syncall implements the SyncAll use case.
// Orchestrates fetching from all enabled data sources and pushing results
// to the CVEDB service via gRPC.
package syncall

import (
	"context"
	"sort"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/ingestion-service/internal/adapter/external/nvd"
	"github.com/osv/ingestion-service/internal/adapter/external/sources"
	"github.com/osv/ingestion-service/internal/adapter/grpcclient"
	"github.com/osv/ingestion-service/internal/domain/service"
)

const defaultSyncAge = 24 * time.Hour

// CVEDBPusher is the interface for pushing CVE data to the CVEDB service.
// Allows mocking in tests.
type CVEDBPusher interface {
	PopulateDB(ctx context.Context, data sources.CVEData) error
}

// Input configures the SyncAll execution.
type Input struct {
	DisabledSources []string // source names to skip
	ForceUpdate     bool     // ignore 24h schedule, always fetch
	Offline         bool     // skip all network fetches
	Mirror          string   // custom NVD mirror URL (json-mirror mode)
	NVDAPIKey       string   // NVD API key for api2 mode
	NVDMode         string   // "json-mirror"|"json-nvd"|"api2"
	CacheDir        string   // cache directory for state files
}

// Output summarizes what was synced.
type Output struct {
	SourcesUpdated []string
	SourcesFailed  []string
	TotalCVEs      int       // total severity rows pushed
	Duration       time.Duration
}

// UseCase coordinates fetching from all sources and pushing to CVEDB.
type UseCase struct {
	orchestrator *service.SyncOrchestrator
	cvedbClient  CVEDBPusher
	stateManager *service.SyncStateManager
	log          zerolog.Logger
	// baseSources are all sources except NVD (NVD is dynamically configured)
	baseSources map[string]service.DataSource
}

// New creates a new SyncAll use case.
// baseSources should contain all non-NVD sources (OSV, GAD, EPSS, PURL2CPE, REDHAT, CURL).
// NVD source is built dynamically per execution based on Input options.
func New(
	baseSources map[string]service.DataSource,
	cvedbClient CVEDBPusher,
	cacheDir string,
	log zerolog.Logger,
) *UseCase {
	return &UseCase{
		baseSources:  baseSources,
		cvedbClient:  cvedbClient,
		stateManager: service.NewSyncStateManager(cacheDir),
		log:          log.With().Str("usecase", "SyncAll").Logger(),
	}
}

// NewWithCVEDBClient creates a SyncAll use case with a real CVEDBClient.
// This is the production constructor; for tests use New with a mock pusher.
func NewWithCVEDBClient(
	baseSources map[string]service.DataSource,
	cvedbAddr string,
	cacheDir string,
	log zerolog.Logger,
) (*UseCase, error) {
	client, err := grpcclient.NewCVEDBClient(cvedbAddr, log)
	if err != nil {
		return nil, err
	}
	return New(baseSources, client, cacheDir, log), nil
}

// Execute runs the full sync pipeline.
func (uc *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
	start := time.Now()
	out := &Output{}

	if in.Offline {
		uc.log.Info().Msg("offline mode: skipping all syncs")
		return out, nil
	}

	// Build source map: add NVD with configured mode
	allSources := make(map[string]service.DataSource, len(uc.baseSources)+1)
	for k, v := range uc.baseSources {
		allSources[k] = v
	}

	// Configure NVD source dynamically
	nvdMode := in.NVDMode
	if nvdMode == "" {
		nvdMode = nvd.ModeJSONMirror
	}
	nvdSrc := nvd.New(nvdMode, in.Mirror, in.NVDAPIKey, in.CacheDir, nil)
	allSources["NVD"] = nvdSrc

	// Build orchestrator with current source set
	orch := service.NewSyncOrchestrator(allSources, uc.log)

	// Apply 24h schedule: filter out sources that don't need sync
	var effectiveDisabled []string
	effectiveDisabled = append(effectiveDisabled, in.DisabledSources...)
	if !in.ForceUpdate {
		for _, name := range orch.SourceNames() {
			if !uc.stateManager.NeedsSync(name, defaultSyncAge) {
				uc.log.Info().Str("source", name).Msg("skip: synced within 24h")
				effectiveDisabled = append(effectiveDisabled, name)
			}
		}
	}

	// Fetch all sources in parallel
	results := orch.FetchAll(ctx, effectiveDisabled)

	// Sort: NVD first (mirrors Python cve-bin-tool behavior)
	sort.Slice(results, func(i, j int) bool {
		if results[i].SourceName == "NVD" {
			return true
		}
		if results[j].SourceName == "NVD" {
			return false
		}
		return results[i].SourceName < results[j].SourceName
	})

	// Push each result to CVEDB
	for _, r := range results {
		if r.Error != nil {
			uc.log.Error().Err(r.Error).Str("source", r.SourceName).Msg("fetch failed, skipping push")
			out.SourcesFailed = append(out.SourcesFailed, r.SourceName)
			continue
		}

		if err := uc.cvedbClient.PopulateDB(ctx, r.Data); err != nil {
			uc.log.Error().Err(err).Str("source", r.SourceName).Msg("populate failed")
			out.SourcesFailed = append(out.SourcesFailed, r.SourceName)
			continue
		}

		// Record success
		uc.stateManager.MarkSynced(r.SourceName) //nolint:errcheck
		out.SourcesUpdated = append(out.SourcesUpdated, r.SourceName)
		out.TotalCVEs += len(r.Data.Severities)
	}

	out.Duration = time.Since(start)
	uc.log.Info().
		Strs("updated", out.SourcesUpdated).
		Strs("failed", out.SourcesFailed).
		Int("total_cves", out.TotalCVEs).
		Dur("elapsed", out.Duration).
		Msg("SyncAll complete")

	return out, nil
}
