package asset

import (
    "context"
    "fmt"
    "sort"
    "time"

    "github.com/google/uuid"

    "github.com/google/osv.dev/services/asset-service/internal/domain/entity"
)

// AssetRepository defines storage for asset tagging + risk.
type AssetRepository interface {
    FindByID(ctx context.Context, id uuid.UUID) (*entity.Asset, error)
    Update(ctx context.Context, asset *entity.Asset) error
    FindAll(ctx context.Context, filter entity.AssetFilter) ([]*entity.Asset, int, error)
}

// FindingClient retrieves finding severity counts for an asset.
type FindingClient interface {
    CountBySeverity(ctx context.Context, assetIP string) (map[string]int, error)
}

// TaggingUseCase handles asset tag management.
type TaggingUseCase struct {
    repo      AssetRepository
}

// NewTaggingUseCase creates a TaggingUseCase.
func NewTaggingUseCase(repo AssetRepository) *TaggingUseCase {
    return &TaggingUseCase{repo: repo}
}

// TagAssetInput is the input for asset tagging.
type TagAssetInput struct {
    AssetID uuid.UUID
    Tags    []string
    Mode    string // "set"|"add"|"remove"
}

// Tag applies tag changes to an asset.
func (uc *TaggingUseCase) Tag(ctx context.Context, in TagAssetInput) error {
    asset, err := uc.repo.FindByID(ctx, in.AssetID)
    if err != nil {
        return fmt.Errorf("find asset: %w", err)
    }

    switch in.Mode {
    case "set":
        asset.Tags = dedupStrings(in.Tags)
    case "add":
        asset.Tags = dedupStrings(append(asset.Tags, in.Tags...))
    case "remove":
        asset.Tags = removeStrings(asset.Tags, in.Tags)
    default:
        return fmt.Errorf("invalid mode: %s (expected set|add|remove)", in.Mode)
    }

    asset.UpdatedAt = nowUTC()
    return uc.repo.Update(ctx, asset)
}

// RiskScoringUseCase computes asset risk scores from finding data.
type RiskScoringUseCase struct {
    repo          AssetRepository
    findingClient FindingClient
}

// NewRiskScoringUseCase creates a RiskScoringUseCase.
func NewRiskScoringUseCase(repo AssetRepository, fc FindingClient) *RiskScoringUseCase {
    return &RiskScoringUseCase{repo: repo, findingClient: fc}
}

// ComputeRiskScore derives an asset risk score (0.0-10.0) from its active findings.
// Formula: Critical×9.5 + High×7.5 + Medium×4.5 + Low×1.5, capped at 10.0
func (uc *RiskScoringUseCase) ComputeRiskScore(ctx context.Context, assetID uuid.UUID) (float64, error) {
    asset, err := uc.repo.FindByID(ctx, assetID)
    if err != nil {
        return 0, fmt.Errorf("find asset: %w", err)
    }

    counts, err := uc.findingClient.CountBySeverity(ctx, asset.IPAddress)
    if err != nil {
        return 0, fmt.Errorf("fetch findings: %w", err)
    }

    score := float64(counts["Critical"])*9.5 +
        float64(counts["High"])*7.5 +
        float64(counts["Medium"])*4.5 +
        float64(counts["Low"])*1.5

    // Normalize to 0-10 range
    if score > 10.0 {
        score = 10.0
    }

    // Update asset risk score
    asset.RiskScore = score
    asset.FindingCount = counts["Critical"] + counts["High"] + counts["Medium"] + counts["Low"]
    asset.UpdatedAt = nowUTC()

    if err := uc.repo.Update(ctx, asset); err != nil {
        return score, fmt.Errorf("update asset: %w", err)
    }

    return score, nil
}

// ListAssetsUseCase handles listing and filtering assets.
type ListAssetsUseCase struct {
    repo AssetRepository
}

// NewListAssetsUseCase creates a ListAssetsUseCase.
func NewListAssetsUseCase(repo AssetRepository) *ListAssetsUseCase {
    return &ListAssetsUseCase{repo: repo}
}

// List returns a filtered, paginated list of assets.
func (uc *ListAssetsUseCase) List(ctx context.Context, filter entity.AssetFilter) ([]*entity.Asset, int, error) {
    if filter.Limit <= 0 {
        filter.Limit = 20
    }
    if filter.Page <= 0 {
        filter.Page = 1
    }
    return uc.repo.FindAll(ctx, filter)
}

// dedupStrings returns a deduplicated, sorted slice of strings.
func dedupStrings(ss []string) []string {
    seen := make(map[string]struct{})
    result := make([]string, 0, len(ss))
    for _, s := range ss {
        if _, ok := seen[s]; !ok {
            seen[s] = struct{}{}
            result = append(result, s)
        }
    }
    sort.Strings(result)
    return result
}

// removeStrings removes all occurrences of removals from ss.
func removeStrings(ss, removals []string) []string {
    remove := make(map[string]struct{})
    for _, r := range removals {
        remove[r] = struct{}{}
    }
    result := make([]string, 0)
    for _, s := range ss {
        if _, ok := remove[s]; !ok {
            result = append(result, s)
        }
    }
    return result
}

func nowUTC() time.Time { return time.Now().UTC() }
