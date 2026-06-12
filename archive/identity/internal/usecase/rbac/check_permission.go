// Package rbac provides DefectDojo RBAC permission checking.
// CheckPermissionUseCase is called by the Identity gRPC server to evaluate
// whether a user may perform an action on an optional resource.
package rbac

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/defectdojo/identity/internal/domain/entity"
	"github.com/defectdojo/identity/internal/domain/repository"
)

// CheckPermissionInput specifies the permission check request.
type CheckPermissionInput struct {
	UserID        uuid.UUID
	Permission    entity.Permission
	ProductID     *uuid.UUID // optional: also check product-scoped role
	ProductTypeID *uuid.UUID // optional: also check product-type-scoped role
}

// CheckPermissionOutput holds the authoritative result.
type CheckPermissionOutput struct {
	Allowed bool
	// Reason documents why access was granted/denied for audit purposes.
	// Values: "superuser" | "global_role" | "product_type_role" | "product_role" | "denied"
	Reason string
}

// CheckPermissionUseCase evaluates whether a user holds a given permission.
// Priority: superuser bypass → global role → product_type role → product role → denied.
type CheckPermissionUseCase struct {
	userRepo  repository.UserRepository
	roleRepo  repository.RoleAssignmentRepository
	permCache repository.PermissionCache
}

// NewCheckPermissionUseCase constructs the use case with all required dependencies.
func NewCheckPermissionUseCase(
	userRepo repository.UserRepository,
	roleRepo repository.RoleAssignmentRepository,
	permCache repository.PermissionCache,
) *CheckPermissionUseCase {
	return &CheckPermissionUseCase{
		userRepo:  userRepo,
		roleRepo:  roleRepo,
		permCache: permCache,
	}
}

// Execute performs the RBAC check. Results are cached in Redis for 60 seconds.
func (uc *CheckPermissionUseCase) Execute(ctx context.Context, in CheckPermissionInput) (*CheckPermissionOutput, error) {
	cacheKey := buildCacheKey(in)

	// 1. Cache hit — avoid DB round-trips for repeated checks.
	if allowed, reason, found := uc.permCache.Get(ctx, cacheKey); found {
		return &CheckPermissionOutput{Allowed: allowed, Reason: reason}, nil
	}

	// 2. Load user.
	user, err := uc.userRepo.FindByID(ctx, in.UserID)
	if err != nil {
		return &CheckPermissionOutput{Allowed: false, Reason: "denied"}, fmt.Errorf("check permission: find user: %w", err)
	}

	// 3. Superuser bypass — no further checks needed.
	if user.IsSuperuser {
		out := &CheckPermissionOutput{Allowed: true, Reason: "superuser"}
		uc.permCache.Set(ctx, cacheKey, true, "superuser")
		return out, nil
	}

	// 4. Global role.
	if user.GlobalRoleID != nil && entity.RoleHasPermission(*user.GlobalRoleID, in.Permission) {
		out := &CheckPermissionOutput{Allowed: true, Reason: "global_role"}
		uc.permCache.Set(ctx, cacheKey, true, "global_role")
		return out, nil
	}

	// 5. ProductType-scoped role.
	if in.ProductTypeID != nil {
		ra, err := uc.roleRepo.FindByUserAndResource(ctx, in.UserID, entity.ScopeProductType, in.ProductTypeID)
		if err == nil && ra != nil && entity.RoleHasPermission(ra.RoleID, in.Permission) {
			out := &CheckPermissionOutput{Allowed: true, Reason: "product_type_role"}
			uc.permCache.Set(ctx, cacheKey, true, "product_type_role")
			return out, nil
		}
	}

	// 6. Product-scoped role.
	if in.ProductID != nil {
		ra, err := uc.roleRepo.FindByUserAndResource(ctx, in.UserID, entity.ScopeProduct, in.ProductID)
		if err == nil && ra != nil && entity.RoleHasPermission(ra.RoleID, in.Permission) {
			out := &CheckPermissionOutput{Allowed: true, Reason: "product_role"}
			uc.permCache.Set(ctx, cacheKey, true, "product_role")
			return out, nil
		}
	}

	// 7. Denied.
	out := &CheckPermissionOutput{Allowed: false, Reason: "denied"}
	uc.permCache.Set(ctx, cacheKey, false, "denied")
	return out, nil
}

// buildCacheKey generates a deterministic Redis key for a permission check.
func buildCacheKey(in CheckPermissionInput) string {
	pid := "nil"
	if in.ProductID != nil {
		pid = in.ProductID.String()
	}
	ptid := "nil"
	if in.ProductTypeID != nil {
		ptid = in.ProductTypeID.String()
	}
	return fmt.Sprintf("perm:%s:%s:%s:%s", in.UserID, in.Permission, pid, ptid)
}

// cacheTTL is how long a permission result is cached.
// Kept short (60s) to reflect role changes promptly.
const cacheTTL = 60 * time.Second
