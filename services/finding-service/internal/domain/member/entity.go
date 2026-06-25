// Package member defines RBAC membership domain entities for Products and ProductTypes.
package member

import (
	"time"

	"github.com/google/uuid"
)

// Role represents a user's access role within a product or product type.
type Role string

const (
	RoleOwner       Role = "Owner"
	RoleMaintainer  Role = "Maintainer"
	RoleWriter      Role = "Writer"
	RoleAPIImporter Role = "API Importer"
	RoleReader      Role = "Reader"
)

// RoleOrder returns the numeric authority level — higher = more access.
func (r Role) RoleOrder() int {
	switch r {
	case RoleOwner:
		return 5
	case RoleMaintainer:
		return 4
	case RoleWriter:
		return 3
	case RoleAPIImporter:
		return 2
	case RoleReader:
		return 1
	default:
		return 0
	}
}

// CanManageFindings returns true if this role can create/update findings.
func (r Role) CanManageFindings() bool {
	return r.RoleOrder() >= RoleAPIImporter.RoleOrder()
}

// CanManageProduct returns true if this role can update product settings.
func (r Role) CanManageProduct() bool {
	return r.RoleOrder() >= RoleMaintainer.RoleOrder()
}

// IsValid returns true if the role is a recognized value.
func (r Role) IsValid() bool {
	switch r {
	case RoleOwner, RoleMaintainer, RoleWriter, RoleAPIImporter, RoleReader:
		return true
	}
	return false
}

// ProductMember represents a user's RBAC membership within a Product.
type ProductMember struct {
	ID        uuid.UUID
	ProductID uuid.UUID
	UserID    uuid.UUID
	Role      Role
	CreatedAt time.Time
}

// New creates a new ProductMember.
func New(productID, userID uuid.UUID, role Role) *ProductMember {
	return &ProductMember{
		ID:        uuid.New(),
		ProductID: productID,
		UserID:    userID,
		Role:      role,
		CreatedAt: time.Now().UTC(),
	}
}

// ProductTypeMember represents a user's RBAC membership within a ProductType.
type ProductTypeMember struct {
	ID            uuid.UUID
	ProductTypeID uuid.UUID
	UserID        uuid.UUID
	Role          Role
	CreatedAt     time.Time
}
