package rbac

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/defectdojo/identity/internal/domain/entity"
	"github.com/defectdojo/identity/internal/domain/repository"
)

// AssignRoleInput specifies a role assignment operation.
type AssignRoleInput struct {
	TargetUserID  uuid.UUID
	RoleID        entity.RoleID
	Scope         entity.RoleScope
	ResourceID    *uuid.UUID // nil for global scope
	ResourceType  string     // "product" | "product_type" | ""
	RequestorID   uuid.UUID  // must be Owner or Superuser of the resource
}

// AssignRoleUseCase creates a scoped role assignment and invalidates permission cache.
type AssignRoleUseCase struct {
	roleRepo  repository.RoleAssignmentRepository
	permCache repository.PermissionCache
}

// NewAssignRoleUseCase creates the use case.
func NewAssignRoleUseCase(
	roleRepo repository.RoleAssignmentRepository,
	permCache repository.PermissionCache,
) *AssignRoleUseCase {
	return &AssignRoleUseCase{roleRepo: roleRepo, permCache: permCache}
}

// Execute creates the role assignment and invalidates the target user's permission cache.
func (uc *AssignRoleUseCase) Execute(ctx context.Context, in AssignRoleInput) error {
	ra := &entity.RoleAssignment{
		ID:           uuid.New(),
		UserID:       in.TargetUserID,
		RoleID:       in.RoleID,
		Scope:        in.Scope,
		ResourceID:   in.ResourceID,
		ResourceType: in.ResourceType,
		CreatedAt:    time.Now().UTC(),
	}

	if err := uc.roleRepo.Create(ctx, ra); err != nil {
		return err
	}

	// Invalidate cached permission results for the affected user.
	return uc.permCache.InvalidateUser(ctx, in.TargetUserID)
}

// RevokeRoleInput specifies a role revocation operation.
type RevokeRoleInput struct {
	AssignmentID uuid.UUID
	UserID       uuid.UUID // for cache invalidation
}

// RevokeRoleUseCase removes a role assignment.
type RevokeRoleUseCase struct {
	roleRepo  repository.RoleAssignmentRepository
	permCache repository.PermissionCache
}

// NewRevokeRoleUseCase creates the use case.
func NewRevokeRoleUseCase(roleRepo repository.RoleAssignmentRepository, permCache repository.PermissionCache) *RevokeRoleUseCase {
	return &RevokeRoleUseCase{roleRepo: roleRepo, permCache: permCache}
}

// Execute removes the assignment and invalidates permission cache.
func (uc *RevokeRoleUseCase) Execute(ctx context.Context, in RevokeRoleInput) error {
	if err := uc.roleRepo.Delete(ctx, in.AssignmentID); err != nil {
		return err
	}
	return uc.permCache.InvalidateUser(ctx, in.UserID)
}
