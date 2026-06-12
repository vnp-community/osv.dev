// Package postgres provides PostgreSQL repository implementations for product-management.
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/defectdojo/product-management/internal/domain/product"
	"github.com/defectdojo/product-management/internal/domain/repository"
)

// ProductRepo implements repository.ProductRepository using pgxpool.
type ProductRepo struct {
	pool *pgxpool.Pool
}

func NewProductRepo(pool *pgxpool.Pool) *ProductRepo {
	return &ProductRepo{pool: pool}
}

func (r *ProductRepo) Create(ctx context.Context, p *product.Product) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO products (id, product_type_id, name, description, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		p.ID, p.ProductTypeID, p.Name, p.Description, p.Tags, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *ProductRepo) FindByID(ctx context.Context, id uuid.UUID) (*product.Product, error) {
	p := &product.Product{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, product_type_id, name, description,
		       prod_numeric_grade, business_criticality, platform, lifecycle,
		       origin, external_audience, internet_accessible,
		       enable_full_risk_acceptance, enable_simple_risk_acceptance,
		       tags, created_at, updated_at
		FROM products WHERE id = $1`, id).Scan(
		&p.ID, &p.ProductTypeID, &p.Name, &p.Description,
		&p.ProdNumericGrade, &p.BusinessCriticality, &p.Platform, &p.Lifecycle,
		&p.Origin, &p.ExternalAudience, &p.InternetAccessible,
		&p.EnableFullRiskAcceptance, &p.EnableSimpleRiskAcceptance,
		&p.Tags, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}
	return p, nil
}

func (r *ProductRepo) FindByName(ctx context.Context, name string, productTypeID *uuid.UUID) (*product.Product, error) {
	p := &product.Product{}
	var err error
	if productTypeID != nil {
		err = r.pool.QueryRow(ctx,
			`SELECT id, product_type_id, name, description, tags, created_at, updated_at
			 FROM products WHERE name = $1 AND product_type_id = $2`,
			name, *productTypeID).Scan(&p.ID, &p.ProductTypeID, &p.Name, &p.Description, &p.Tags, &p.CreatedAt, &p.UpdatedAt)
	} else {
		err = r.pool.QueryRow(ctx,
			`SELECT id, product_type_id, name, description, tags, created_at, updated_at
			 FROM products WHERE name = $1 LIMIT 1`,
			name).Scan(&p.ID, &p.ProductTypeID, &p.Name, &p.Description, &p.Tags, &p.CreatedAt, &p.UpdatedAt)
	}
	if err != nil {
		return nil, fmt.Errorf("product not found by name: %w", err)
	}
	return p, nil
}

func (r *ProductRepo) List(ctx context.Context, filter repository.ProductFilter) ([]*product.Product, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM products`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, product_type_id, name, description, tags, created_at, updated_at
		FROM products
		ORDER BY name ASC
		LIMIT $1 OFFSET $2`,
		filter.Limit, filter.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var products []*product.Product
	for rows.Next() {
		p := &product.Product{}
		if err := rows.Scan(&p.ID, &p.ProductTypeID, &p.Name, &p.Description, &p.Tags, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}
	return products, total, rows.Err()
}

func (r *ProductRepo) Update(ctx context.Context, p *product.Product) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE products SET
			name = $2, description = $3, product_type_id = $4,
			business_criticality = $5, lifecycle = $6, tags = $7, updated_at = NOW()
		WHERE id = $1`,
		p.ID, p.Name, p.Description, p.ProductTypeID, p.BusinessCriticality, p.Lifecycle, p.Tags,
	)
	return err
}

func (r *ProductRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, id)
	return err
}
