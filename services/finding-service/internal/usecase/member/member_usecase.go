// Package member_usecase implements RBAC membership use cases for product access control.
package member_usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/member"
)

var (
	// ErrNotOwner is returned when the requester lacks Owner/Maintainer role.
	ErrNotOwner = errors.New("only product owner or maintainer can manage members")
	// ErrMemberExists is returned when the user is already a member.
	ErrMemberExists = errors.New("user is already a member of this product")
	// ErrInvalidRole is returned when the role is not a recognized value.
	ErrInvalidRole = errors.New("invalid role")
	// ErrNotFound is returned when a member is not found.
	ErrNotFound = errors.New("member not found")
)

// Permission names for product-level RBAC.
const (
	PermProductView   = "product:view"
	PermProductAdd    = "product:add"
	PermProductChange = "product:change"
	PermProductDelete = "product:delete"
	PermFindingView   = "finding:view"
	PermFindingAdd    = "finding:add"
	PermFindingChange = "finding:change"
	PermFindingDelete = "finding:delete"
	PermScanImport    = "scan:import"
	PermMemberAdd     = "member:add"
	PermMemberRemove  = "member:remove"
	PermAuditView     = "audit:view"
)

// rolePermissions maps Role → allowed permissions.
var rolePermissions = map[member.Role][]string{
	member.RoleOwner: {
		PermProductView, PermProductAdd, PermProductChange, PermProductDelete,
		PermFindingView, PermFindingAdd, PermFindingChange, PermFindingDelete,
		PermScanImport, PermMemberAdd, PermMemberRemove, PermAuditView,
	},
	member.RoleMaintainer: {
		PermProductView, PermProductChange,
		PermFindingView, PermFindingAdd, PermFindingChange, PermFindingDelete,
		PermScanImport, PermMemberAdd, PermAuditView,
	},
	member.RoleWriter: {
		PermProductView,
		PermFindingView, PermFindingAdd, PermFindingChange,
		PermScanImport,
	},
	member.RoleAPIImporter: {
		PermProductView, PermFindingAdd, PermScanImport,
	},
	member.RoleReader: {
		PermProductView, PermFindingView,
	},
}

// ─── AddProductMember ─────────────────────────────────────────────────────────

// AddProductMemberInput is the request for adding a member to a product.
type AddProductMemberInput struct {
	RequesterUserID uuid.UUID
	ProductID       uuid.UUID
	UserID          uuid.UUID
	Role            member.Role
}

// AddProductMemberUseCase adds a user as a member of a product.
type AddProductMemberUseCase struct {
	memberRepo member.ProductMemberRepository
}

// NewAddProductMember creates a new AddProductMemberUseCase.
func NewAddProductMember(repo member.ProductMemberRepository) *AddProductMemberUseCase {
	return &AddProductMemberUseCase{memberRepo: repo}
}

// Execute adds a member if the requester has Owner or Maintainer role.
func (uc *AddProductMemberUseCase) Execute(ctx context.Context, in AddProductMemberInput) (*member.ProductMember, error) {
	// 1. Validate role
	if !in.Role.IsValid() {
		return nil, ErrInvalidRole
	}

	// 2. Check requester is Owner or Maintainer
	requesterRole, err := uc.memberRepo.GetRole(ctx, in.ProductID, in.RequesterUserID)
	if err != nil || requesterRole == nil {
		return nil, ErrNotOwner
	}
	if *requesterRole != member.RoleOwner && *requesterRole != member.RoleMaintainer {
		return nil, ErrNotOwner
	}

	// 3. Check user not already member
	existing, _ := uc.memberRepo.FindByProductAndUser(ctx, in.ProductID, in.UserID)
	if existing != nil {
		return nil, ErrMemberExists
	}

	// 4. Create and save
	m := member.New(in.ProductID, in.UserID, in.Role)
	if err := uc.memberRepo.Save(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// ─── RemoveProductMember ──────────────────────────────────────────────────────

// RemoveProductMemberInput is the request for removing a member from a product.
type RemoveProductMemberInput struct {
	RequesterUserID uuid.UUID
	ProductID       uuid.UUID
	UserID          uuid.UUID
}

// RemoveProductMemberUseCase removes a user's membership from a product.
type RemoveProductMemberUseCase struct {
	memberRepo member.ProductMemberRepository
}

// NewRemoveProductMember creates a new RemoveProductMemberUseCase.
func NewRemoveProductMember(repo member.ProductMemberRepository) *RemoveProductMemberUseCase {
	return &RemoveProductMemberUseCase{memberRepo: repo}
}

// Execute removes a member — only Owner can remove members.
func (uc *RemoveProductMemberUseCase) Execute(ctx context.Context, in RemoveProductMemberInput) error {
	role, err := uc.memberRepo.GetRole(ctx, in.ProductID, in.RequesterUserID)
	if err != nil || role == nil {
		return ErrNotOwner
	}
	if *role != member.RoleOwner {
		return ErrNotOwner
	}
	return uc.memberRepo.Delete(ctx, in.ProductID, in.UserID)
}

// ─── CheckProductPermission ───────────────────────────────────────────────────

// CheckProductPermissionInput is the request for checking a user's permission.
type CheckProductPermissionInput struct {
	UserID     uuid.UUID
	ProductID  uuid.UUID
	Permission string
}

// CheckProductPermissionUseCase checks if a user has a specific permission on a product.
type CheckProductPermissionUseCase struct {
	memberRepo member.ProductMemberRepository
}

// NewCheckProductPermission creates a new CheckProductPermissionUseCase.
func NewCheckProductPermission(repo member.ProductMemberRepository) *CheckProductPermissionUseCase {
	return &CheckProductPermissionUseCase{memberRepo: repo}
}

// Execute returns true if the user has the requested permission on the product.
func (uc *CheckProductPermissionUseCase) Execute(ctx context.Context, in CheckProductPermissionInput) (bool, error) {
	role, err := uc.memberRepo.GetRole(ctx, in.ProductID, in.UserID)
	if err != nil || role == nil {
		return false, nil // not a member
	}
	perms := rolePermissions[*role]
	for _, p := range perms {
		if p == in.Permission {
			return true, nil
		}
	}
	return false, nil
}

// ─── ListProductMembers ───────────────────────────────────────────────────────

// ListProductMembersUseCase lists all members of a product.
type ListProductMembersUseCase struct {
	memberRepo member.ProductMemberRepository
}

// NewListProductMembers creates a new ListProductMembersUseCase.
func NewListProductMembers(repo member.ProductMemberRepository) *ListProductMembersUseCase {
	return &ListProductMembersUseCase{memberRepo: repo}
}

// Execute returns all members of the given product.
func (uc *ListProductMembersUseCase) Execute(ctx context.Context, productID uuid.UUID) ([]*member.ProductMember, error) {
	return uc.memberRepo.ListByProduct(ctx, productID)
}
