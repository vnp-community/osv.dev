// Package adminuser provides the admin user management use case for SEED-001.
// It supports direct user creation (bypassing the invite flow), bulk creation,
// and product-scoped role assignment.
package adminuser

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/osv/identity-service/internal/domain/entity"
	domainerr "github.com/osv/identity-service/internal/domain/error"
	"github.com/osv/identity-service/internal/domain/repository"
)

// UseCase orchestrates admin user seed operations.
type UseCase struct {
	userRepo repository.UserRepository
}

// New creates a new admin user UseCase.
func New(userRepo repository.UserRepository) *UseCase {
	return &UseCase{userRepo: userRepo}
}

// CreateUser creates a single user directly (admin only, bypasses invite flow).
// The plaintext password is validated and hashed before storage.
func (uc *UseCase) CreateUser(ctx context.Context, in entity.UserCreateInput) (*entity.User, error) {
	if err := validateRole(in.Role); err != nil {
		return nil, err
	}
	if len(in.Password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &entity.User{
		ID:           uuid.New(),
		Email:        in.Email,
		Username:     in.Username,
		Role:         in.Role,
		AuthProvider: entity.AuthProviderLocal,
		IsActive:     in.IsActive,
		IsVerified:   in.IsVerified,
	}

	if err := uc.userRepo.CreateDirect(ctx, user, string(hashed)); err != nil {
		return nil, err
	}
	return user, nil
}

// CreateBulkUsers creates up to 100 users in a single transaction.
// Partial failures are captured per-item without aborting the whole batch.
func (uc *UseCase) CreateBulkUsers(ctx context.Context, inputs []entity.UserCreateInput) ([]entity.UserCreateResult, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("users list is empty")
	}
	if len(inputs) > 100 {
		return nil, fmt.Errorf("bulk limit exceeded: max 100 users per request, got %d", len(inputs))
	}

	// Pre-validate roles before hitting the DB
	results := make([]entity.UserCreateResult, 0, len(inputs))
	validInputs := make([]entity.UserCreateInput, 0, len(inputs))

	for _, in := range inputs {
		if err := validateRole(in.Role); err != nil {
			results = append(results, entity.UserCreateResult{
				Email:   in.Email,
				Status:  "error",
				Message: err.Error(),
			})
			continue
		}
		validInputs = append(validInputs, in)
	}

	if len(validInputs) > 0 {
		dbResults, err := uc.userRepo.CreateBulk(ctx, validInputs)
		if err != nil {
			return nil, fmt.Errorf("bulk create: %w", err)
		}
		results = append(results, dbResults...)
	}

	return results, nil
}

// AssignRole grants a global or product-scoped role to a user.
func (uc *UseCase) AssignRole(ctx context.Context, in entity.RoleAssignment) error {
	// Verify user exists
	if _, err := uc.userRepo.FindByID(ctx, in.UserID); err != nil {
		if err == domainerr.ErrUserNotFound {
			return fmt.Errorf("user %s not found", in.UserID)
		}
		return fmt.Errorf("find user: %w", err)
	}

	if in.Scope != "global" && in.Scope != "product" {
		return fmt.Errorf("invalid scope %q: must be 'global' or 'product'", in.Scope)
	}
	if in.Scope == "product" && in.ResourceID == nil {
		return fmt.Errorf("resource_id is required when scope is 'product'")
	}

	return uc.userRepo.AssignRole(ctx, in)
}

// validateRole returns an error if the role is not allowed.
func validateRole(role string) error {
	switch role {
	case "admin", "user", "readonly", "agent":
		return nil
	default:
		return fmt.Errorf("invalid role %q: must be admin, user, readonly, or agent", role)
	}
}
