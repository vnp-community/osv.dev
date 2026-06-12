// Package syncsource implements the SyncSource use case.
// Triggers an on-demand sync for a single named data source.
package syncsource

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/data-service/internal/adapter/external/sources"
	"github.com/osv/data-service/internal/domain/service"
)

// CVEDBPusher is the interface for pushing CVE data to the CVEDB service.
type CVEDBPusher interface {
	PopulateDB(ctx context.Context, data sources.CVEData) error
}

// Input specifies which source to sync.
type Input struct {
	SourceName string // e.g. "NVD", "OSV", "GAD", "EPSS"
}

// Output holds the result of the single-source sync.
type Output struct {
	SourceName string
	TotalCVEs  int
	Duration   time.Duration
}

// UseCase runs a single data source sync on-demand.
type UseCase struct {
	sources     map[string]service.DataSource
	cvedbClient CVEDBPusher
	log         zerolog.Logger
}

// New creates a new SyncSource use case.
func New(srcs map[string]service.DataSource, cvedbClient CVEDBPusher, log zerolog.Logger) *UseCase {
	return &UseCase{
		sources:     srcs,
		cvedbClient: cvedbClient,
		log:         log.With().Str("usecase", "SyncSource").Logger(),
	}
}

// Execute fetches and pushes data for the specified source.
func (uc *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
	if in.SourceName == "" {
		return nil, fmt.Errorf("source name is required")
	}

	src, ok := uc.sources[in.SourceName]
	if !ok {
		return nil, fmt.Errorf("unknown source: %q", in.SourceName)
	}

	start := time.Now()
	uc.log.Info().Str("source", in.SourceName).Msg("sync source start")

	data, err := src.FetchCVEData(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", in.SourceName, err)
	}

	if err := uc.cvedbClient.PopulateDB(ctx, data); err != nil {
		return nil, fmt.Errorf("populate %s: %w", in.SourceName, err)
	}

	elapsed := time.Since(start)
	uc.log.Info().
		Str("source", in.SourceName).
		Int("severities", len(data.Severities)).
		Dur("elapsed", elapsed).
		Msg("sync source done")

	return &Output{
		SourceName: in.SourceName,
		TotalCVEs:  len(data.Severities),
		Duration:   elapsed,
	}, nil
}
