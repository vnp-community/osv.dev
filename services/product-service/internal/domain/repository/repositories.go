// Package repository defines persistence interfaces for the product-management service.
package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/defectdojo/product-service/internal/domain/engagement"
	"github.com/defectdojo/product-service/internal/domain/product"
	"github.com/defectdojo/product-service/internal/domain/product_type"
	testp "github.com/defectdojo/product-service/internal/domain/test"
)

// ProductTypeRepository defines persistence operations for ProductType.
type ProductTypeRepository interface {
	Create(ctx context.Context, pt *product_type.ProductType) error
	FindByID(ctx context.Context, id uuid.UUID) (*product_type.ProductType, error)
	FindByName(ctx context.Context, name string) (*product_type.ProductType, error)
	List(ctx context.Context, limit, offset int) ([]*product_type.ProductType, int, error)
	Update(ctx context.Context, pt *product_type.ProductType) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// ProductFilter defines query parameters for listing products.
type ProductFilter struct {
	ProductTypeID *uuid.UUID
	Search        string
	Limit         int
	Offset        int
}

// ProductRepository defines persistence operations for Product.
type ProductRepository interface {
	Create(ctx context.Context, p *product.Product) error
	FindByID(ctx context.Context, id uuid.UUID) (*product.Product, error)
	FindByName(ctx context.Context, name string, productTypeID *uuid.UUID) (*product.Product, error)
	List(ctx context.Context, filter ProductFilter) ([]*product.Product, int, error)
	Update(ctx context.Context, p *product.Product) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// EngagementRepository defines persistence operations for Engagement.
type EngagementRepository interface {
	Create(ctx context.Context, e *engagement.Engagement) error
	FindByID(ctx context.Context, id uuid.UUID) (*engagement.Engagement, error)
	FindByNameAndProduct(ctx context.Context, name string, productID uuid.UUID) (*engagement.Engagement, error)
	List(ctx context.Context, productID uuid.UUID, limit, offset int) ([]*engagement.Engagement, int, error)
	Update(ctx context.Context, e *engagement.Engagement) error
}

// TestRepository defines persistence operations for Test.
type TestRepository interface {
	Create(ctx context.Context, t *testp.Test) error
	FindByID(ctx context.Context, id uuid.UUID) (*testp.Test, error)
	FindByEngagementAndType(ctx context.Context, engagementID uuid.UUID, scanType string) (*testp.Test, error)
	List(ctx context.Context, engagementID uuid.UUID) ([]*testp.Test, error)
	Update(ctx context.Context, t *testp.Test) error
}
