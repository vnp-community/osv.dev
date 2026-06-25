package member

import (
	"context"

	"github.com/google/uuid"
)

// ProductMemberRepository defines persistence operations for product members.
type ProductMemberRepository interface {
	Save(ctx context.Context, m *ProductMember) error
	FindByProductAndUser(ctx context.Context, productID, userID uuid.UUID) (*ProductMember, error)
	ListByProduct(ctx context.Context, productID uuid.UUID) ([]*ProductMember, error)
	Delete(ctx context.Context, productID, userID uuid.UUID) error
	GetRole(ctx context.Context, productID, userID uuid.UUID) (*Role, error)
}

// ProductTypeMemberRepository defines persistence operations for product-type members.
type ProductTypeMemberRepository interface {
	Save(ctx context.Context, m *ProductTypeMember) error
	FindByProductTypeAndUser(ctx context.Context, productTypeID, userID uuid.UUID) (*ProductTypeMember, error)
	ListByProductType(ctx context.Context, productTypeID uuid.UUID) ([]*ProductTypeMember, error)
	Delete(ctx context.Context, productTypeID, userID uuid.UUID) error
}
