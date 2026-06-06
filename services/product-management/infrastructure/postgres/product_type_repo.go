package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/defectdojo/product-management/internal/domain/product_type"
)

// ProductTypeRepo implements repository.ProductTypeRepository.
type ProductTypeRepo struct {
	pool *pgxpool.Pool
}

func NewProductTypeRepo(pool *pgxpool.Pool) *ProductTypeRepo {
	return &ProductTypeRepo{pool: pool}
}

func (r *ProductTypeRepo) Create(ctx context.Context, pt *product_type.ProductType) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO product_types (id, name, description, critical_product, key_product,
			enable_full_risk_acceptance, enable_simple_risk_acceptance, tags, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		pt.ID, pt.Name, pt.Description, pt.CriticalProduct, pt.KeyProduct,
		pt.EnableFullRiskAcceptance, pt.EnableSimpleRiskAcceptance,
		pt.Tags, pt.CreatedAt, pt.UpdatedAt,
	)
	return err
}

func (r *ProductTypeRepo) FindByID(ctx context.Context, id uuid.UUID) (*product_type.ProductType, error) {
	pt := &product_type.ProductType{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, description, critical_product, key_product,
			enable_full_risk_acceptance, enable_simple_risk_acceptance, tags, created_at, updated_at
		FROM product_types WHERE id = $1`, id).Scan(
		&pt.ID, &pt.Name, &pt.Description, &pt.CriticalProduct, &pt.KeyProduct,
		&pt.EnableFullRiskAcceptance, &pt.EnableSimpleRiskAcceptance,
		&pt.Tags, &pt.CreatedAt, &pt.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("product_type not found: %w", err)
	}
	return pt, nil
}

func (r *ProductTypeRepo) FindByName(ctx context.Context, name string) (*product_type.ProductType, error) {
	pt := &product_type.ProductType{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, description, critical_product, key_product,
			enable_full_risk_acceptance, enable_simple_risk_acceptance, tags, created_at, updated_at
		FROM product_types WHERE name = $1`, name).Scan(
		&pt.ID, &pt.Name, &pt.Description, &pt.CriticalProduct, &pt.KeyProduct,
		&pt.EnableFullRiskAcceptance, &pt.EnableSimpleRiskAcceptance,
		&pt.Tags, &pt.CreatedAt, &pt.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("product_type not found by name: %w", err)
	}
	return pt, nil
}

func (r *ProductTypeRepo) List(ctx context.Context, limit, offset int) ([]*product_type.ProductType, int, error) {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM product_types`).Scan(&total)

	rows, err := r.pool.Query(ctx, `
		SELECT id, name, description, tags, created_at, updated_at
		FROM product_types ORDER BY name LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var types []*product_type.ProductType
	for rows.Next() {
		pt := &product_type.ProductType{}
		if err := rows.Scan(&pt.ID, &pt.Name, &pt.Description, &pt.Tags, &pt.CreatedAt, &pt.UpdatedAt); err != nil {
			return nil, 0, err
		}
		types = append(types, pt)
	}
	return types, total, rows.Err()
}

func (r *ProductTypeRepo) Update(ctx context.Context, pt *product_type.ProductType) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE product_types SET name=$2, description=$3, tags=$4, updated_at=NOW()
		WHERE id=$1`, pt.ID, pt.Name, pt.Description, pt.Tags)
	return err
}

func (r *ProductTypeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM product_types WHERE id=$1`, id)
	return err
}
