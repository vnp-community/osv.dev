// Package product_usecase implements CRUD use cases for Product entities.
package product_usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	natsutil "github.com/defectdojo/pkg/nats"
	"github.com/defectdojo/product-management/internal/domain/product"
	"github.com/defectdojo/product-management/internal/domain/product_type"
	"github.com/defectdojo/product-management/internal/domain/repository"
)

// ErrNotFound is returned when an entity does not exist.
var ErrNotFound = errors.New("not found")

// ─── CreateProduct ────────────────────────────────────────────────────────────

type CreateProductInput struct {
	Name          string
	Description   string
	ProductTypeID uuid.UUID
	RequestorID   uuid.UUID
}

type CreateProductUseCase struct {
	productRepo     repository.ProductRepository
	productTypeRepo repository.ProductTypeRepository
	eventPub        *natsutil.Publisher
}

func NewCreateProduct(
	pr repository.ProductRepository,
	ptr repository.ProductTypeRepository,
	pub *natsutil.Publisher,
) *CreateProductUseCase {
	return &CreateProductUseCase{productRepo: pr, productTypeRepo: ptr, eventPub: pub}
}

func (uc *CreateProductUseCase) Execute(ctx context.Context, in CreateProductInput) (*product.Product, error) {
	// Validate ProductType exists
	_, err := uc.productTypeRepo.FindByID(ctx, in.ProductTypeID)
	if err != nil {
		return nil, fmt.Errorf("product type not found: %w", err)
	}

	p, err := product.New(in.ProductTypeID, in.Name, in.Description)
	if err != nil {
		return nil, err
	}

	if err := uc.productRepo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("create product: %w", err)
	}

	_ = uc.eventPub.Publish(ctx, "defectdojo.product.created", map[string]interface{}{
		"product_id":      p.ID,
		"name":            p.Name,
		"product_type_id": p.ProductTypeID,
		"requestor_id":    in.RequestorID,
	})

	return p, nil
}

// ─── GetOrCreateProduct ───────────────────────────────────────────────────────

type GetOrCreateProductInput struct {
	Name            string
	ProductTypeName string
	RequestorID     uuid.UUID
}

type GetOrCreateProductUseCase struct {
	productRepo     repository.ProductRepository
	productTypeRepo repository.ProductTypeRepository
	eventPub        *natsutil.Publisher
}

func NewGetOrCreateProduct(
	pr repository.ProductRepository,
	ptr repository.ProductTypeRepository,
	pub *natsutil.Publisher,
) *GetOrCreateProductUseCase {
	return &GetOrCreateProductUseCase{productRepo: pr, productTypeRepo: ptr, eventPub: pub}
}

// Execute finds or creates a product by name, creating the ProductType if needed.
// Returns (product, productType, created, error).
func (uc *GetOrCreateProductUseCase) Execute(ctx context.Context, in GetOrCreateProductInput) (*product.Product, *product_type.ProductType, bool, error) {
	// 1. Get or create ProductType
	pt, err := uc.productTypeRepo.FindByName(ctx, in.ProductTypeName)
	if err != nil {
		// Create it
		pt, err = product_type.New(in.ProductTypeName, "")
		if err != nil {
			return nil, nil, false, err
		}
		if err := uc.productTypeRepo.Create(ctx, pt); err != nil {
			return nil, nil, false, err
		}
	}

	// 2. Find or create Product
	existing, err := uc.productRepo.FindByName(ctx, in.Name, &pt.ID)
	if err == nil && existing != nil {
		return existing, pt, false, nil
	}

	p, err := product.New(pt.ID, in.Name, "")
	if err != nil {
		return nil, nil, false, err
	}
	if err := uc.productRepo.Create(ctx, p); err != nil {
		return nil, nil, false, err
	}

	_ = uc.eventPub.Publish(ctx, "defectdojo.product.created", map[string]interface{}{
		"product_id":      p.ID,
		"name":            p.Name,
		"product_type_id": p.ProductTypeID,
	})

	return p, pt, true, nil
}
