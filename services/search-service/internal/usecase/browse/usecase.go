// Package browse — Browse vendor/product use case for search-service.
// Business logic layer between HTTP handler and Redis BrowseRepository.
package browse

import (
	"context"
	"fmt"
	"time"

	"github.com/osv/search-service/internal/domain/entity"
	"github.com/osv/search-service/internal/domain/repository"
)

// UseCase handles vendor/product browsing operations.
type UseCase struct {
	repo repository.BrowseRepository
}

// New creates a BrowseUseCase.
func New(repo repository.BrowseRepository) *UseCase {
	return &UseCase{repo: repo}
}

// ListVendors returns all vendors in the CPE catalog.
// Returns VendorCatalog with sorted vendor list and total count.
func (uc *UseCase) ListVendors(ctx context.Context) (*entity.VendorCatalog, error) {
	vendors, err := uc.repo.GetAllVendors(ctx)
	if err != nil {
		return nil, fmt.Errorf("list vendors: %w", err)
	}

	return &entity.VendorCatalog{
		Vendors:  vendors,
		Total:    len(vendors),
		CachedAt: time.Now().UTC(),
	}, nil
}

// ListProductsByVendor returns all products for a given vendor.
// vendor: case-insensitive vendor name (will be lowercased internally)
// Returns ProductCatalog with sorted product list and total count.
// Returns error if vendor not found in CPE database.
func (uc *UseCase) ListProductsByVendor(ctx context.Context, vendor string) (*entity.ProductCatalog, error) {
	products, err := uc.repo.GetProductsByVendor(ctx, vendor)
	if err != nil {
		return nil, err // propagate "not found" error with original message
	}

	return &entity.ProductCatalog{
		Vendor:   vendor,
		Products: products,
		Total:    len(products),
		CachedAt: time.Now().UTC(),
	}, nil
}
