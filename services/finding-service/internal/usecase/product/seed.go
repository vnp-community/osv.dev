// Package product_usecase provides the product hierarchy seed use case for SEED-002.
// It supports bulk creation of ProductTypes, Products, Engagements, and Tests
// for client-side data seeding via the API.
package product_usecase

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/engagement"
	"github.com/osv/finding-service/internal/domain/product"
	"github.com/osv/finding-service/internal/domain/product_type"
	"github.com/osv/finding-service/internal/domain/repository"
	testp "github.com/osv/finding-service/internal/domain/test"
)

// BulkResult is the per-item outcome of a bulk operation.
type BulkResult struct {
	Name    string     `json:"name"`
	Status  string     `json:"status"`  // "created" | "exists" | "error"
	ID      *uuid.UUID `json:"id,omitempty"`
	Message string     `json:"message,omitempty"`
}

// BulkSummary wraps per-item results with aggregated counts.
type BulkSummary struct {
	CreatedCount int          `json:"created_count"`
	ExistsCount  int          `json:"exists_count"`
	FailedCount  int          `json:"failed_count"`
	Results      []BulkResult `json:"results"`
}

// SeedResult is returned from SeedProductHierarchy.
type SeedResult struct {
	EngagementID uuid.UUID  `json:"engagement_id"`
	TestID       *uuid.UUID `json:"test_id,omitempty"`
}

// ProductCreateInput is the input for bulk/import product creation.
type ProductCreateInput struct {
	Name                string     `json:"name"`
	ProductTypeID       *uuid.UUID `json:"product_type_id,omitempty"`
	ProductTypeName     string     `json:"product_type_name"` // auto-resolve/create if ProductTypeID is nil
	Description         string     `json:"description"`
	BusinessCriticality string     `json:"business_criticality"`
	Platform            string     `json:"platform"`
	Lifecycle           string     `json:"lifecycle"`
	Origin              string     `json:"origin"`
	ExternalAudience    bool       `json:"external_audience"`
	InternetAccessible  bool       `json:"internet_accessible"`
	Tags                []string   `json:"tags"`
}

// EngagementCreateInput is the input for creating an engagement.
type EngagementCreateInput struct {
	Name           string    `json:"name"`
	EngagementType string    `json:"engagement_type"` // "Interactive" | "CI/CD"
	StartDate      time.Time `json:"start_date"`
	Version        string    `json:"version"`
	Tags           []string  `json:"tags"`
}

// TestCreateInput is the input for creating a test.
type TestCreateInput struct {
	Title       string     `json:"title"`
	TestType    string     `json:"test_type"` // nmap|zap|agent|manual|dast|sast
	TargetStart *time.Time `json:"target_start,omitempty"`
	TargetEnd   *time.Time `json:"target_end,omitempty"`
}

// ProductSeedInput is the input for composite seed (Engagement + optional Test).
type ProductSeedInput struct {
	Engagement EngagementCreateInput `json:"engagement"`
	Test       *TestCreateInput      `json:"test,omitempty"`
}

// UseCase orchestrates product hierarchy seed operations.
type UseCase struct {
	productTypeRepo  repository.ProductTypeRepository
	productRepo      repository.ProductRepository
	engagementRepo   repository.EngagementRepository
	testRepo         repository.TestRepository
}

// New creates a new productseed UseCase.
func New(
	ptr repository.ProductTypeRepository,
	pr repository.ProductRepository,
	er repository.EngagementRepository,
	tr repository.TestRepository,
) *UseCase {
	return &UseCase{
		productTypeRepo: ptr,
		productRepo:     pr,
		engagementRepo:  er,
		testRepo:        tr,
	}
}

// BulkCreateProductTypes creates multiple product types. If a type with the same
// name already exists, it is recorded as status "exists" (not an error).
func (uc *UseCase) BulkCreateProductTypes(ctx context.Context, names []string) BulkSummary {
	summary := BulkSummary{Results: make([]BulkResult, 0, len(names))}

	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			summary.FailedCount++
			summary.Results = append(summary.Results, BulkResult{
				Name: name, Status: "error", Message: "name is required",
			})
			continue
		}

		existing, _ := uc.productTypeRepo.FindByName(ctx, name)
		if existing != nil {
			id := existing.ID
			summary.ExistsCount++
			summary.Results = append(summary.Results, BulkResult{
				Name: name, Status: "exists", ID: &id,
			})
			continue
		}

		pt, err := product_type.New(name, "")
		if err != nil {
			summary.FailedCount++
			summary.Results = append(summary.Results, BulkResult{
				Name: name, Status: "error", Message: err.Error(),
			})
			continue
		}

		if err := uc.productTypeRepo.Create(ctx, pt); err != nil {
			summary.FailedCount++
			summary.Results = append(summary.Results, BulkResult{
				Name: name, Status: "error", Message: err.Error(),
			})
			continue
		}

		id := pt.ID
		summary.CreatedCount++
		summary.Results = append(summary.Results, BulkResult{
			Name: name, Status: "created", ID: &id,
		})
	}

	return summary
}

// BulkCreateProducts creates multiple products. If ProductTypeName is provided and
// no ProductTypeID is given, it resolves or auto-creates the ProductType.
func (uc *UseCase) BulkCreateProducts(ctx context.Context, inputs []ProductCreateInput) BulkSummary {
	summary := BulkSummary{Results: make([]BulkResult, 0, len(inputs))}

	for _, in := range inputs {
		if strings.TrimSpace(in.Name) == "" {
			summary.FailedCount++
			summary.Results = append(summary.Results, BulkResult{
				Name: in.Name, Status: "error", Message: "name is required",
			})
			continue
		}

		// Resolve ProductTypeID
		productTypeID, err := uc.resolveProductType(ctx, in.ProductTypeID, in.ProductTypeName)
		if err != nil {
			summary.FailedCount++
			summary.Results = append(summary.Results, BulkResult{
				Name: in.Name, Status: "error", Message: "product type: " + err.Error(),
			})
			continue
		}

		// Check if product with same name already exists
		existing, _ := uc.productRepo.FindByName(ctx, in.Name, &productTypeID)
		if existing != nil {
			id := existing.ID
			summary.ExistsCount++
			summary.Results = append(summary.Results, BulkResult{
				Name: in.Name, Status: "exists", ID: &id,
			})
			continue
		}

		p, err := product.New(productTypeID, in.Name, in.Description)
		if err != nil {
			summary.FailedCount++
			summary.Results = append(summary.Results, BulkResult{
				Name: in.Name, Status: "error", Message: err.Error(),
			})
			continue
		}

		// Apply optional fields
		if in.BusinessCriticality != "" {
			p.BusinessCriticality = product.BusinessCriticality(in.BusinessCriticality)
		}
		if in.Platform != "" {
			p.Platform = product.Platform(in.Platform)
		}
		if in.Lifecycle != "" {
			p.Lifecycle = product.Lifecycle(in.Lifecycle)
		}
		if in.Origin != "" {
			p.Origin = product.Origin(in.Origin)
		}
		p.ExternalAudience = in.ExternalAudience
		p.InternetAccessible = in.InternetAccessible
		if len(in.Tags) > 0 {
			p.Tags = in.Tags
		}

		if err := uc.productRepo.Create(ctx, p); err != nil {
			summary.FailedCount++
			summary.Results = append(summary.Results, BulkResult{
				Name: in.Name, Status: "error", Message: err.Error(),
			})
			continue
		}

		id := p.ID
		summary.CreatedCount++
		summary.Results = append(summary.Results, BulkResult{
			Name: in.Name, Status: "created", ID: &id,
		})
	}

	return summary
}

// SeedProductHierarchy creates an Engagement (and optional Test) for a given product.
// Returns the IDs of created entities.
func (uc *UseCase) SeedProductHierarchy(ctx context.Context, productID uuid.UUID, in ProductSeedInput) (*SeedResult, error) {
	engName := in.Engagement.Name
	if engName == "" {
		engName = "Seed Engagement"
	}
	engType := engagement.Type(in.Engagement.EngagementType)
	if engType == "" {
		engType = engagement.TypeInteractive
	}
	startDate := in.Engagement.StartDate
	if startDate.IsZero() {
		startDate = time.Now().UTC()
	}

	// Check if engagement already exists
	existing, _ := uc.engagementRepo.FindByNameAndProduct(ctx, engName, productID)
	var eng *engagement.Engagement
	if existing != nil {
		eng = existing
	} else {
		eng = &engagement.Engagement{
			ID:             uuid.New(),
			ProductID:      productID,
			Name:           engName,
			EngagementType: engType,
			StartDate:      startDate,
			Status:         engagement.StatusInProgress,
			Version:        in.Engagement.Version,
			Tags:           in.Engagement.Tags,
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}
		if err := uc.engagementRepo.Create(ctx, eng); err != nil {
			return nil, fmt.Errorf("create engagement: %w", err)
		}
	}

	result := &SeedResult{EngagementID: eng.ID}

	// Optional Test creation
	if in.Test != nil {
		testTitle := in.Test.Title
		if testTitle == "" {
			testTitle = "Seed Test"
		}
		scanType := in.Test.TestType
		if scanType == "" {
			scanType = "manual"
		}

		// Check if test already exists for this engagement + scan type
		existingTest, _ := uc.testRepo.FindByEngagementAndType(ctx, eng.ID, scanType)
		if existingTest != nil {
			testID := existingTest.ID
			result.TestID = &testID
		} else {
			now := time.Now().UTC()
			t := &testp.Test{
				ID:           uuid.New(),
				EngagementID: eng.ID,
				ScanType:     scanType,
				Title:        testTitle,
				TargetStart:  now,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if in.Test.TargetStart != nil {
				t.TargetStart = *in.Test.TargetStart
			}
			if in.Test.TargetEnd != nil {
				t.TargetEnd = in.Test.TargetEnd
			}

			if err := uc.testRepo.Create(ctx, t); err != nil {
				return nil, fmt.Errorf("create test: %w", err)
			}
			testID := t.ID
			result.TestID = &testID
		}
	}

	return result, nil
}

// ImportProductsFromJSON parses a JSON array of ProductCreateInput and bulk-creates them.
func (uc *UseCase) ImportProductsFromJSON(ctx context.Context, r io.Reader) (BulkSummary, error) {
	var inputs []ProductCreateInput
	if err := json.NewDecoder(r).Decode(&inputs); err != nil {
		return BulkSummary{}, fmt.Errorf("parse JSON: %w", err)
	}
	return uc.BulkCreateProducts(ctx, inputs), nil
}

// ImportProductsFromCSV parses CSV rows into ProductCreateInput and bulk-creates them.
// Expected header: name,product_type_name,description,business_criticality,platform,lifecycle,tags
func (uc *UseCase) ImportProductsFromCSV(ctx context.Context, r io.Reader) (BulkSummary, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true

	headers, err := cr.Read()
	if err != nil {
		return BulkSummary{}, fmt.Errorf("read CSV header: %w", err)
	}
	// Build header index (case-insensitive)
	idx := make(map[string]int, len(headers))
	for i, h := range headers {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	col := func(row []string, name string) string {
		if i, ok := idx[name]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	var inputs []ProductCreateInput
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return BulkSummary{}, fmt.Errorf("parse CSV row: %w", err)
		}

		var tags []string
		if t := col(row, "tags"); t != "" {
			for _, tag := range strings.Split(t, ";") {
				if tag = strings.TrimSpace(tag); tag != "" {
					tags = append(tags, tag)
				}
			}
		}

		inputs = append(inputs, ProductCreateInput{
			Name:                col(row, "name"),
			ProductTypeName:     col(row, "product_type_name"),
			Description:         col(row, "description"),
			BusinessCriticality: col(row, "business_criticality"),
			Platform:            col(row, "platform"),
			Lifecycle:           col(row, "lifecycle"),
			Tags:                tags,
		})
	}

	return uc.BulkCreateProducts(ctx, inputs), nil
}

// resolveProductType returns a ProductType UUID from an explicit ID or by name
// (auto-creating if it doesn't exist).
func (uc *UseCase) resolveProductType(ctx context.Context, id *uuid.UUID, name string) (uuid.UUID, error) {
	if id != nil && *id != uuid.Nil {
		return *id, nil
	}
	if name == "" {
		return uuid.Nil, fmt.Errorf("either product_type_id or product_type_name is required")
	}

	existing, _ := uc.productTypeRepo.FindByName(ctx, name)
	if existing != nil {
		return existing.ID, nil
	}

	// Auto-create ProductType
	pt, err := product_type.New(name, "")
	if err != nil {
		return uuid.Nil, err
	}
	if err := uc.productTypeRepo.Create(ctx, pt); err != nil {
		return uuid.Nil, fmt.Errorf("create product type: %w", err)
	}
	return pt.ID, nil
}
