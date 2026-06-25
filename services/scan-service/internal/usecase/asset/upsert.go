package asset

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"

    assetdomain "github.com/osv/scan-service/internal/domain/asset"
)

// AssetRepository defines the storage interface for assets
type AssetRepository interface {
    FindByIP(ctx context.Context, ipAddress string) (*assetdomain.Asset, error)
    Save(ctx context.Context, asset *assetdomain.Asset) error
    Update(ctx context.Context, asset *assetdomain.Asset) error
}

// UpsertInput contains the data from a scan to upsert into the asset registry
type UpsertInput struct {
    ScanID    uuid.UUID
    IPAddress string
    Hostname  string
    OS        string
    OSVersion string
    Services  []assetdomain.NetworkService
    WebTech   []assetdomain.WebTechnology
}

// UpsertResult describes what happened during the upsert
type UpsertResult struct {
    Asset   *assetdomain.Asset
    Created bool   // true if new asset was created; false if existing was updated
    Reason  string // "created" | "updated"
}

// UpsertUseCase handles creating or updating assets from scan results
type UpsertUseCase struct {
    repo   AssetRepository
    logger zerolog.Logger
}

// New creates the UpsertUseCase
func New(repo AssetRepository, logger zerolog.Logger) *UpsertUseCase {
    return &UpsertUseCase{repo: repo, logger: logger}
}

// Execute upserts an asset: creates new if IP is unknown, updates if IP already tracked.
// This is IDEMPOTENT: multiple calls with the same IP update the existing asset.
func (uc *UpsertUseCase) Execute(ctx context.Context, in UpsertInput) (*UpsertResult, error) {
    if in.IPAddress == "" {
        return nil, fmt.Errorf("IP address is required")
    }

    // Try to find existing asset by IP
    existing, err := uc.repo.FindByIP(ctx, in.IPAddress)
    if err != nil {
        return nil, fmt.Errorf("find asset by IP: %w", err)
    }

    if existing != nil {
        // UPDATE existing asset
        existing.UpdateFromScan(in.ScanID, in.Hostname, in.OS, in.OSVersion, in.Services)
        if len(in.WebTech) > 0 {
            existing.WebTech = in.WebTech
        }

        if err := uc.repo.Update(ctx, existing); err != nil {
            return nil, fmt.Errorf("update asset: %w", err)
        }

        uc.logger.Debug().
            Str("ip", in.IPAddress).
            Str("asset_id", existing.ID.String()).
            Str("scan_id", in.ScanID.String()).
            Msg("asset updated from scan")

        return &UpsertResult{Asset: existing, Created: false, Reason: "updated"}, nil
    }

    // CREATE new asset
    a, err := assetdomain.NewAsset(in.IPAddress, in.Hostname, in.OS, in.OSVersion)
    if err != nil {
        return nil, fmt.Errorf("create asset: %w", err)
    }

    a.Services = in.Services
    a.WebTech = in.WebTech
    scanID := in.ScanID
    now := time.Now().UTC()
    a.LastScanID = &scanID
    a.LastScannedAt = &now

    if err := uc.repo.Save(ctx, a); err != nil {
        return nil, fmt.Errorf("save new asset: %w", err)
    }

    uc.logger.Info().
        Str("ip", in.IPAddress).
        Str("asset_id", a.ID.String()).
        Str("scan_id", in.ScanID.String()).
        Msg("new asset registered")

    return &UpsertResult{Asset: a, Created: true, Reason: "created"}, nil
}

// ExecuteBatch upserts multiple assets from a scan (e.g., /24 network scan)
func (uc *UpsertUseCase) ExecuteBatch(ctx context.Context, inputs []UpsertInput) ([]*UpsertResult, error) {
    results := make([]*UpsertResult, 0, len(inputs))

    for _, in := range inputs {
        result, err := uc.Execute(ctx, in)
        if err != nil {
            uc.logger.Error().Err(err).Str("ip", in.IPAddress).Msg("failed to upsert asset")
            continue // Don't fail entire batch for single asset
        }
        results = append(results, result)
    }

    uc.logger.Info().
        Int("total", len(inputs)).
        Int("success", len(results)).
        Msg("batch asset upsert completed")

    return results, nil
}
