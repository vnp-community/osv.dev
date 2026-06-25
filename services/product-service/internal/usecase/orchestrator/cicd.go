package orchestrator

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"

    "github.com/google/osv.dev/services/product-service/internal/domain/entity"
)

// Repositories required by the CI/CD orchestrator.
type ProductRepository interface {
    FindByName(ctx context.Context, name string) (*entity.Product, error)
    Save(ctx context.Context, p *entity.Product) error
    FindByID(ctx context.Context, id uuid.UUID) (*entity.Product, error)
}

type EngagementRepository interface {
    Save(ctx context.Context, e *entity.Engagement) error
    FindOpenCICD(ctx context.Context, productID uuid.UUID) (*entity.Engagement, error)
    Update(ctx context.Context, e *entity.Engagement) error
}

type TestRepository interface {
    Save(ctx context.Context, t *entity.Test) error
    Update(ctx context.Context, t *entity.Test) error
}

// ScanClient triggers scans on the scan-service.
type ScanClient interface {
    TriggerScan(ctx context.Context, targets []string, scanType string) (uuid.UUID, error)
}

// CICDInput is sent by CI/CD pipelines to trigger a scan within product context.
type CICDInput struct {
    ProductName string
    ProductType string // "web" | "api" | "mobile"
    Targets     []string
    ScanType    string // "full" | "web" | "discovery"
    BuildID     string
    CommitHash  string
    Branch      string
    RepoURL     string
    Version     string
}

// CICDOutput is returned after orchestration setup.
type CICDOutput struct {
    ScanID       uuid.UUID
    ProductID    uuid.UUID
    EngagementID uuid.UUID
    TestID       uuid.UUID
}

// Orchestrator automates product + engagement + test creation for CI/CD pipelines.
type Orchestrator struct {
    productRepo    ProductRepository
    engagementRepo EngagementRepository
    testRepo       TestRepository
    scanClient     ScanClient
    logger         zerolog.Logger
}

// New creates the CI/CD Orchestrator.
func New(
    productRepo ProductRepository,
    engagementRepo EngagementRepository,
    testRepo TestRepository,
    scanClient ScanClient,
    logger zerolog.Logger,
) *Orchestrator {
    return &Orchestrator{
        productRepo:    productRepo,
        engagementRepo: engagementRepo,
        testRepo:       testRepo,
        scanClient:     scanClient,
        logger:         logger,
    }
}

// Orchestrate handles the full CI/CD scan flow:
// 1. Find or create Product
// 2. Create or reuse a CI/CD Engagement for this build
// 3. Create a Test entry
// 4. Trigger the actual scan
func (o *Orchestrator) Orchestrate(ctx context.Context, in CICDInput) (*CICDOutput, error) {
    // 1. Find or create product
    product, err := o.findOrCreateProduct(ctx, in)
    if err != nil {
        return nil, fmt.Errorf("find/create product: %w", err)
    }

    // 2. Create CI/CD engagement for this build
    engagement := entity.NewEngagement(product.ID, fmt.Sprintf("CI/CD — %s", in.BuildID), entity.TypeCICD)
    engagement.BuildID = in.BuildID
    engagement.CommitHash = in.CommitHash
    engagement.BranchTag = in.Branch
    engagement.SourceCodeManagementURI = in.RepoURL
    engagement.Version = in.Version
    engagement.Status = entity.StatusInProgress

    if err := o.engagementRepo.Save(ctx, engagement); err != nil {
        return nil, fmt.Errorf("save engagement: %w", err)
    }

    // 3. Trigger scan
    scanID, err := o.scanClient.TriggerScan(ctx, in.Targets, in.ScanType)
    if err != nil {
        return nil, fmt.Errorf("trigger scan: %w", err)
    }

    // 4. Create Test entry
    testTitle := fmt.Sprintf("%s Scan - %s", in.ScanType, time.Now().UTC().Format("2006-01-02"))
    test := entity.NewTest(engagement.ID, testTitle, in.ScanType, &scanID)
    if err := o.testRepo.Save(ctx, test); err != nil {
        return nil, fmt.Errorf("save test: %w", err)
    }

    o.logger.Info().
        Str("product", product.Name).
        Str("engagement_id", engagement.ID.String()).
        Str("scan_id", scanID.String()).
        Msg("CI/CD orchestration complete")

    return &CICDOutput{
        ScanID:       scanID,
        ProductID:    product.ID,
        EngagementID: engagement.ID,
        TestID:       test.ID,
    }, nil
}

func (o *Orchestrator) findOrCreateProduct(ctx context.Context, in CICDInput) (*entity.Product, error) {
    if product, err := o.productRepo.FindByName(ctx, in.ProductName); err == nil && product != nil {
        return product, nil
    }

    // Create new product
    product, err := entity.NewProduct(uuid.Nil, in.ProductName, "Auto-created by CI/CD orchestrator")
    if err != nil {
        return nil, err
    }
    product.Platform = entity.Platform(in.ProductType)

    if err := o.productRepo.Save(ctx, product); err != nil {
        return nil, err
    }
    return product, nil
}
