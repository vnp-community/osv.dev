// Package http — rbac_repo.go
// TASK-HC-010: PostgreSQL repository for RBAC roles metadata and permission categories.
// Replaces static maps in GetRBACMatrix.
package http

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RoleMetadata holds display info for a single role.
type RoleMetadata struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

// PermissionCategory groups permissions for the UI matrix.
type PermissionCategory struct {
	Category string   `json:"category"`
	Items    []string `json:"items"`
}

// RBACRepo reads RBAC metadata from PostgreSQL.
type RBACRepo struct {
	pool *pgxpool.Pool
}

// NewRBACRepo creates a RBACRepo.
func NewRBACRepo(pool *pgxpool.Pool) *RBACRepo {
	return &RBACRepo{pool: pool}
}

// ListRoles returns all roles with their metadata.
func (r *RBACRepo) ListRoles(ctx context.Context) ([]RoleMetadata, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT name, display_name, description, color
		FROM rbac_roles
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("rbac_roles.List: %w", err)
	}
	defer rows.Close()

	var roles []RoleMetadata
	for rows.Next() {
		var rm RoleMetadata
		if err := rows.Scan(&rm.Name, &rm.DisplayName, &rm.Description, &rm.Color); err != nil {
			continue
		}
		roles = append(roles, rm)
	}
	return roles, rows.Err()
}

// ListPermissionCategories returns all permission categories with their items.
func (r *RBACRepo) ListPermissionCategories(ctx context.Context) ([]PermissionCategory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT pc.category, array_agg(cp.permission ORDER BY cp.permission) AS items
		FROM rbac_permission_categories pc
		LEFT JOIN rbac_category_permissions cp ON cp.category_id = pc.id
		GROUP BY pc.category, pc.sort_order
		ORDER BY pc.sort_order
	`)
	if err != nil {
		return nil, fmt.Errorf("rbac_categories.List: %w", err)
	}
	defer rows.Close()

	var cats []PermissionCategory
	for rows.Next() {
		var pc PermissionCategory
		var items []string
		if err := rows.Scan(&pc.Category, &items); err != nil {
			continue
		}
		pc.Items = items
		cats = append(cats, pc)
	}
	return cats, rows.Err()
}
