package product

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Filter defines query parameters for listing products.
type Filter struct {
	ProductTypeID *uuid.UUID
	Search        string
	Limit         int
	Offset        int
}

// Repository defines all persistence operations for products.
type Repository interface {
	Create(ctx context.Context, p *Product) error
	FindByID(ctx context.Context, id uuid.UUID) (*Product, error)
	FindByName(ctx context.Context, name string, productTypeID *uuid.UUID) (*Product, error)
	List(ctx context.Context, filter Filter) ([]*Product, int, error)
	ListWithStats(ctx context.Context, filter Filter) ([]*ProductWithStats, int, error)
	ListWithGrades(ctx context.Context) ([]*ProductWithStats, error)
	GetCriticalCount(ctx context.Context, productID string) (int, error)
	GetCriticalCountAt(ctx context.Context, productID string, atTime time.Time) (int, error)
	Save(ctx context.Context, p *Product) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// ProductWithStats includes a product and its finding aggregations.
type ProductWithStats struct {
	*Product
	CriticalCount int
	HighCount     int
	MediumCount   int
	LowCount      int
	TotalActive   int
}
