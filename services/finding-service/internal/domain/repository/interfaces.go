package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/product"
	"github.com/osv/finding-service/internal/domain/product_type"
	testp "github.com/osv/finding-service/internal/domain/test"
)

type ProductRepository interface {
	Create(ctx context.Context, p *product.Product) error
	FindByID(ctx context.Context, id uuid.UUID) (*product.Product, error)
	FindByName(ctx context.Context, name string, productTypeID *uuid.UUID) (*product.Product, error)
	Save(ctx context.Context, p *product.Product) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type ProductTypeRepository interface {
	Create(ctx context.Context, pt *product_type.ProductType) error
	FindByID(ctx context.Context, id uuid.UUID) (*product_type.ProductType, error)
	FindByName(ctx context.Context, name string) (*product_type.ProductType, error)
}

type TestRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*testp.Test, error)
	ListByEngagement(ctx context.Context, engagementID uuid.UUID) ([]*testp.Test, error)
	Create(ctx context.Context, t *testp.Test) error
	FindByEngagementAndType(ctx context.Context, engagementID uuid.UUID, scanType string) (*testp.Test, error)
	Delete(ctx context.Context, id uuid.UUID) error
}


