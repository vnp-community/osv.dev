package entity

import (
	"time"

	"github.com/google/uuid"
)

// RoleScope defines the scope level for a role assignment.
type RoleScope string

const (
	ScopeGlobal      RoleScope = "global"
	ScopeProductType RoleScope = "product_type"
	ScopeProduct     RoleScope = "product"
)

// RoleAssignment links a user to a role within a specific scope.
// A user may have multiple assignments: one global + per-product overrides.
type RoleAssignment struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	RoleID       RoleID
	Scope        RoleScope
	ResourceID   *uuid.UUID // nil when Scope == global
	ResourceType string     // "product" | "product_type" | "" for global
	CreatedAt    time.Time
}
