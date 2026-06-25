// Package postgres provides the PostgreSQL repositories for sla-service.
package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	ucconfig "github.com/osv/sla-service/internal/usecase/config"
)

// ─── SLA Config Repository ────────────────────────────────────────────────────

// SLAConfigRepo is the PostgreSQL implementation of ucconfig.Repository.
type SLAConfigRepo struct {
	db *pgxpool.Pool
}

// NewSLAConfigRepo creates a new SLAConfigRepo.
func NewSLAConfigRepo(db *pgxpool.Pool) ucconfig.Repository {
	return &SLAConfigRepo{db: db}
}

// Save inserts or updates an SLA configuration.
func (r *SLAConfigRepo) Save(ctx context.Context, cfg *ucconfig.SLAConfiguration) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO sla_configurations
			(id, name, description, critical_days, high_days, medium_days, low_days, is_default, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET
			name=$2, description=$3,
			critical_days=$4, high_days=$5, medium_days=$6, low_days=$7,
			is_default=$8, updated_at=$10
	`, cfg.ID, cfg.Name, cfg.Description,
		cfg.Critical, cfg.High, cfg.Medium, cfg.Low,
		cfg.IsDefault, cfg.CreatedAt, cfg.UpdatedAt)
	return err
}

// FindByID returns the SLA configuration with the given ID.
func (r *SLAConfigRepo) FindByID(ctx context.Context, id uuid.UUID) (*ucconfig.SLAConfiguration, error) {
	cfg := &ucconfig.SLAConfiguration{}
	err := r.db.QueryRow(ctx, `
		SELECT id, name, description, critical_days, high_days, medium_days, low_days, is_default, created_at, updated_at
		FROM sla_configurations WHERE id=$1
	`, id).Scan(
		&cfg.ID, &cfg.Name, &cfg.Description,
		&cfg.Critical, &cfg.High, &cfg.Medium, &cfg.Low,
		&cfg.IsDefault, &cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return cfg, err
}

// FindDefault returns the default SLA configuration.
func (r *SLAConfigRepo) FindDefault(ctx context.Context) (*ucconfig.SLAConfiguration, error) {
	cfg := &ucconfig.SLAConfiguration{}
	err := r.db.QueryRow(ctx, `
		SELECT id, name, description, critical_days, high_days, medium_days, low_days, is_default, created_at, updated_at
		FROM sla_configurations WHERE is_default=TRUE LIMIT 1
	`).Scan(
		&cfg.ID, &cfg.Name, &cfg.Description,
		&cfg.Critical, &cfg.High, &cfg.Medium, &cfg.Low,
		&cfg.IsDefault, &cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return cfg, err
}

// List returns all SLA configurations.
func (r *SLAConfigRepo) List(ctx context.Context) ([]*ucconfig.SLAConfiguration, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, description, critical_days, high_days, medium_days, low_days, is_default, created_at, updated_at
		FROM sla_configurations ORDER BY is_default DESC, name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*ucconfig.SLAConfiguration
	for rows.Next() {
		cfg := &ucconfig.SLAConfiguration{}
		if err := rows.Scan(
			&cfg.ID, &cfg.Name, &cfg.Description,
			&cfg.Critical, &cfg.High, &cfg.Medium, &cfg.Low,
			&cfg.IsDefault, &cfg.CreatedAt, &cfg.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, cfg)
	}
	return result, rows.Err()
}

// Delete removes an SLA configuration by ID.
func (r *SLAConfigRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sla_configurations WHERE id=$1`, id)
	return err
}

// ─── SLA Assignment Repository ────────────────────────────────────────────────

// SLAAssignmentRepo is the PostgreSQL implementation of ucconfig.AssignmentRepository.
type SLAAssignmentRepo struct {
	db *pgxpool.Pool
}

// NewSLAAssignmentRepo creates a new SLAAssignmentRepo.
func NewSLAAssignmentRepo(db *pgxpool.Pool) ucconfig.AssignmentRepository {
	return &SLAAssignmentRepo{db: db}
}

// Save upserts a product ↔ SLA config assignment.
func (r *SLAAssignmentRepo) Save(ctx context.Context, a *ucconfig.SLAProductAssignment) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO sla_product_assignments (product_id, sla_configuration_id, assigned_at, assigned_by)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (product_id) DO UPDATE SET
			sla_configuration_id=$2, assigned_at=$3, assigned_by=$4
	`, a.ProductID, a.SLAConfigurationID, a.AssignedAt, a.AssignedBy)
	return err
}

// FindByProductID returns the SLA assignment for a product.
func (r *SLAAssignmentRepo) FindByProductID(ctx context.Context, productID uuid.UUID) (*ucconfig.SLAProductAssignment, error) {
	a := &ucconfig.SLAProductAssignment{}
	err := r.db.QueryRow(ctx, `
		SELECT product_id, sla_configuration_id, assigned_at, assigned_by
		FROM sla_product_assignments WHERE product_id=$1
	`, productID).Scan(&a.ProductID, &a.SLAConfigurationID, &a.AssignedAt, &a.AssignedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

// CountAssignments counts products assigned to a specific SLA config.
func (r *SLAAssignmentRepo) CountAssignments(ctx context.Context, slaConfigID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM sla_product_assignments WHERE sla_configuration_id=$1
	`, slaConfigID).Scan(&count)
	return count, err
}
