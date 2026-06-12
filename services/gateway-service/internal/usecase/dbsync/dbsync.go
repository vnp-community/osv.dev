// Package dbsync implements the database sync use case for the gateway.
package dbsync

import (
	"context"
	"fmt"

	gc "github.com/osv/gateway-service/internal/adapter/grpcclient"
)

// DataSyncPort defines what the use case needs from the datasync service.
type DataSyncPort interface {
	SyncAll(ctx context.Context, req gc.SyncAllRequest) (*gc.SyncAllResult, error)
	SyncSource(ctx context.Context, source, nvdMode, nvdAPIKey string) (*gc.SyncSourceResult, error)
	GetSyncStatus(ctx context.Context) (*gc.SyncStatus, error)
}

// Input configures the database sync.
type Input struct {
	Source          string   // "" = all sources
	DisabledSources []string
	ForceUpdate     bool
	NVDMode         string
	NVDAPIKey       string
	Mirror          string
}

// Output holds the sync result.
type Output struct {
	SourcesUpdated []string
	SourcesFailed  []string
	TotalCVEs      int32
	Duration       string
}

// UseCase implements DB sync orchestration.
type UseCase struct {
	datasyncClient DataSyncPort
}

// NewUseCase creates a DBSync use case.
func NewUseCase(client DataSyncPort) *UseCase {
	return &UseCase{datasyncClient: client}
}

// Execute dispatches to SyncAll or SyncSource.
func (uc *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
	if in.Source != "" {
		res, err := uc.datasyncClient.SyncSource(ctx, in.Source, in.NVDMode, in.NVDAPIKey)
		if err != nil {
			return nil, fmt.Errorf("SyncSource: %w", err)
		}
		updated := []string{}
		if res.SourceName != "" {
			updated = []string{res.SourceName}
		}
		return &Output{
			SourcesUpdated: updated,
			TotalCVEs:      res.TotalCVEs,
		}, nil
	}

	res, err := uc.datasyncClient.SyncAll(ctx, gc.SyncAllRequest{
		DisabledSources: in.DisabledSources,
		ForceUpdate:     in.ForceUpdate,
		NVDMode:         in.NVDMode,
		NVDAPIKey:       in.NVDAPIKey,
		Mirror:          in.Mirror,
	})
	if err != nil {
		return nil, fmt.Errorf("SyncAll: %w", err)
	}

	return &Output{
		SourcesUpdated: res.SourcesUpdated,
		SourcesFailed:  res.SourcesFailed,
		TotalCVEs:      res.TotalCVEs,
		Duration:       res.Duration,
	}, nil
}

// GetStatus returns current sync status.
func (uc *UseCase) GetStatus(ctx context.Context) (*gc.SyncStatus, error) {
	return uc.datasyncClient.GetSyncStatus(ctx)
}
