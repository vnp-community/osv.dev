// Package service contains the SyncOrchestrator domain service.
// It manages parallel fetching from all data sources and provides
// last-sync state tracking.
package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/ingestion-service/internal/adapter/external/sources"
)

// DataSource is the interface all external CVE sources must satisfy.
// Defined here so domain service doesn't import adapter packages.
type DataSource interface {
	Name() string
	FetchCVEData(ctx context.Context) (sources.CVEData, error)
}

// FetchResult holds the outcome of fetching one data source.
type FetchResult struct {
	SourceName string
	Data       sources.CVEData
	Error      error
	Duration   time.Duration
}

// SyncOrchestrator coordinates parallel fetches from multiple DataSources.
type SyncOrchestrator struct {
	sources map[string]DataSource
	log     zerolog.Logger
}

// NewSyncOrchestrator creates a SyncOrchestrator from a name→source map.
func NewSyncOrchestrator(srcs map[string]DataSource, log zerolog.Logger) *SyncOrchestrator {
	return &SyncOrchestrator{
		sources: srcs,
		log:     log.With().Str("service", "SyncOrchestrator").Logger(),
	}
}

// FetchAll runs all enabled sources concurrently and collects results.
// Sources in the disabled list are skipped.
// Results order is non-deterministic; sort by SourceName afterward if needed.
func (o *SyncOrchestrator) FetchAll(ctx context.Context, disabled []string) []FetchResult {
	disabledSet := make(map[string]struct{}, len(disabled))
	for _, d := range disabled {
		disabledSet[strings.ToUpper(d)] = struct{}{}
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []FetchResult
	)

	for name, src := range o.sources {
		if _, skip := disabledSet[strings.ToUpper(name)]; skip {
			continue
		}
		wg.Add(1)
		go func(n string, s DataSource) {
			defer wg.Done()
			start := time.Now()
			o.log.Info().Str("source", n).Msg("fetch start")

			data, err := s.FetchCVEData(ctx)
			elapsed := time.Since(start)

			if err != nil {
				o.log.Error().Err(err).Str("source", n).Dur("elapsed", elapsed).Msg("fetch failed")
			} else {
				o.log.Info().
					Str("source", n).
					Int("severities", len(data.Severities)).
					Int("ranges", len(data.Ranges)).
					Int("metrics", len(data.Metrics)).
					Dur("elapsed", elapsed).
					Msg("fetch done")
			}

			mu.Lock()
			results = append(results, FetchResult{
				SourceName: n,
				Data:       data,
				Error:      err,
				Duration:   elapsed,
			})
			mu.Unlock()
		}(name, src)
	}

	wg.Wait()
	return results
}

// SourceNames returns the names of all registered sources.
func (o *SyncOrchestrator) SourceNames() []string {
	names := make([]string, 0, len(o.sources))
	for n := range o.sources {
		names = append(names, n)
	}
	return names
}

// HasSource checks if a source by name is registered.
func (o *SyncOrchestrator) HasSource(name string) bool {
	_, ok := o.sources[strings.ToUpper(name)]
	return ok
}
