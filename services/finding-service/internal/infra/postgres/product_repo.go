// Package postgres provides PostgreSQL implementation for the product repository.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/osv/finding-service/internal/domain/product"
)

// ProductRepo implements repository.ProductRepository using pgxpool.
type ProductRepo struct {
	pool *pgxpool.Pool
}

// NewProductRepo creates a new ProductRepo.
func NewProductRepo(pool *pgxpool.Pool) *ProductRepo {
	return &ProductRepo{pool: pool}
}

const productColumns = `
	id, product_type_id, name, description,
	business_criticality, platform, lifecycle, origin,
	external_audience, internet_accessible,
	enable_full_risk_acceptance, enable_simple_risk_acceptance,
	enable_product_tag_inheritance, sla_configuration_id,
	tags, created_at, updated_at`

func scanProduct(row pgx.Row) (*product.Product, error) {
	p := &product.Product{}
	var bc, pl, lc, origin string
	err := row.Scan(
		&p.ID, &p.ProductTypeID, &p.Name, &p.Description,
		&bc, &pl, &lc, &origin,
		&p.ExternalAudience, &p.InternetAccessible,
		&p.EnableFullRiskAcceptance, &p.EnableSimpleRiskAcceptance,
		&p.EnableProductTagInheritance, &p.SLAConfigurationID,
		&p.Tags, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.BusinessCriticality = product.BusinessCriticality(bc)
	p.Platform = product.Platform(pl)
	p.Lifecycle = product.Lifecycle(lc)
	p.Origin = product.Origin(origin)
	return p, nil
}

// Create inserts a new product into the database.
func (r *ProductRepo) Create(ctx context.Context, p *product.Product) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO products (
			id, product_type_id, name, description,
			business_criticality, platform, lifecycle, origin,
			external_audience, internet_accessible,
			enable_full_risk_acceptance, enable_simple_risk_acceptance,
			enable_product_tag_inheritance, sla_configuration_id,
			tags, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (id) DO NOTHING`,
		p.ID, p.ProductTypeID, p.Name, p.Description,
		string(p.BusinessCriticality), string(p.Platform),
		string(p.Lifecycle), string(p.Origin),
		p.ExternalAudience, p.InternetAccessible,
		p.EnableFullRiskAcceptance, p.EnableSimpleRiskAcceptance,
		p.EnableProductTagInheritance, p.SLAConfigurationID,
		p.Tags, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

// FindByID retrieves a product by its ID.
func (r *ProductRepo) FindByID(ctx context.Context, id uuid.UUID) (*product.Product, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+productColumns+` FROM products WHERE id = $1`, id)
	p, err := scanProduct(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// FindByName retrieves a product by name, optionally filtered by product type.
func (r *ProductRepo) FindByName(ctx context.Context, name string, productTypeID *uuid.UUID) (*product.Product, error) {
	var row pgx.Row
	if productTypeID != nil {
		row = r.pool.QueryRow(ctx,
			`SELECT `+productColumns+` FROM products WHERE name = $1 AND product_type_id = $2`,
			name, *productTypeID)
	} else {
		row = r.pool.QueryRow(ctx,
			`SELECT `+productColumns+` FROM products WHERE name = $1`,
			name)
	}
	p, err := scanProduct(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// Save updates an existing product.
func (r *ProductRepo) Save(ctx context.Context, p *product.Product) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE products SET
			product_type_id=$1, name=$2, description=$3,
			business_criticality=$4, platform=$5, lifecycle=$6, origin=$7,
			external_audience=$8, internet_accessible=$9,
			enable_full_risk_acceptance=$10, enable_simple_risk_acceptance=$11,
			enable_product_tag_inheritance=$12, sla_configuration_id=$13,
			tags=$14, updated_at=$15
		WHERE id=$16`,
		p.ProductTypeID, p.Name, p.Description,
		string(p.BusinessCriticality), string(p.Platform),
		string(p.Lifecycle), string(p.Origin),
		p.ExternalAudience, p.InternetAccessible,
		p.EnableFullRiskAcceptance, p.EnableSimpleRiskAcceptance,
		p.EnableProductTagInheritance, p.SLAConfigurationID,
		p.Tags, time.Now().UTC(), p.ID,
	)
	return err
}

// Delete removes a product by ID.
func (r *ProductRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, id)
	return err
}

// List returns a paginated list of products.
func (r *ProductRepo) List(ctx context.Context, filter product.Filter) ([]*product.Product, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM products`).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	rows, err := r.pool.Query(ctx, `SELECT `+productColumns+` FROM products ORDER BY name ASC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var products []*product.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}
	return products, total, rows.Err()
}

// ListWithStats returns products enriched with finding severity counts.
func (r *ProductRepo) ListWithStats(ctx context.Context, filter product.Filter) ([]*product.ProductWithStats, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM products`).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}

	q := `SELECT
		p.id, p.name, p.description, p.product_type_id, p.business_criticality,
		p.platform, p.lifecycle, p.origin, p.tags, p.created_at,
		COUNT(f.id) FILTER (WHERE f.severity = 'Critical' AND f.active = true AND f.duplicate = false) AS critical_count,
		COUNT(f.id) FILTER (WHERE f.severity = 'High' AND f.active = true AND f.duplicate = false) AS high_count,
		COUNT(f.id) FILTER (WHERE f.severity = 'Medium' AND f.active = true AND f.duplicate = false) AS medium_count,
		COUNT(f.id) FILTER (WHERE f.severity = 'Low' AND f.active = true AND f.duplicate = false) AS low_count,
		COUNT(f.id) FILTER (WHERE f.active = true AND f.duplicate = false) AS total_active
	FROM products p
	LEFT JOIN findings f ON f.product_id = p.id
	GROUP BY p.id
	ORDER BY p.name ASC
	LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, q, limit, filter.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var products []*product.ProductWithStats
	for rows.Next() {
		pw := &product.ProductWithStats{Product: &product.Product{}}
		p := pw.Product
		var ptID uuid.UUID
		var bc, pl, lc, origin string
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Description, &ptID, &bc,
			&pl, &lc, &origin, &p.Tags, &p.CreatedAt,
			&pw.CriticalCount, &pw.HighCount, &pw.MediumCount, &pw.LowCount, &pw.TotalActive,
		); err != nil {
			return nil, 0, err
		}
		p.ProductTypeID = ptID
		p.BusinessCriticality = product.BusinessCriticality(bc)
		p.Platform = product.Platform(pl)
		p.Lifecycle = product.Lifecycle(lc)
		p.Origin = product.Origin(origin)
		products = append(products, pw)
	}
	return products, total, rows.Err()
}

// ListWithGrades returns all products with severity counts ordered by risk.
func (r *ProductRepo) ListWithGrades(ctx context.Context) ([]*product.ProductWithStats, error) {
	q := `SELECT
		p.id, p.name,
		COUNT(f.id) FILTER (WHERE f.severity = 'Critical' AND f.active = true AND f.duplicate = false) AS critical_count,
		COUNT(f.id) FILTER (WHERE f.severity = 'High' AND f.active = true AND f.duplicate = false) AS high_count,
		COUNT(f.id) FILTER (WHERE f.severity = 'Medium' AND f.active = true AND f.duplicate = false) AS medium_count,
		COUNT(f.id) FILTER (WHERE f.severity = 'Low' AND f.active = true AND f.duplicate = false) AS low_count,
		COUNT(f.id) FILTER (WHERE f.active = true AND f.duplicate = false) AS total_active
	FROM products p
	LEFT JOIN findings f ON f.product_id = p.id
	GROUP BY p.id, p.name
	ORDER BY critical_count DESC, high_count DESC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*product.ProductWithStats
	for rows.Next() {
		pw := &product.ProductWithStats{Product: &product.Product{}}
		if err := rows.Scan(
			&pw.Product.ID, &pw.Product.Name,
			&pw.CriticalCount, &pw.HighCount, &pw.MediumCount, &pw.LowCount, &pw.TotalActive,
		); err != nil {
			return nil, err
		}
		products = append(products, pw)
	}
	return products, rows.Err()
}

// GetCriticalCount returns the number of active critical findings for a product.
func (r *ProductRepo) GetCriticalCount(ctx context.Context, productID string) (int, error) {
	pid, err := uuid.Parse(productID)
	if err != nil {
		return 0, err
	}
	var count int
	err = r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM findings WHERE product_id=$1 AND severity='Critical' AND active=true AND duplicate=false`,
		pid).Scan(&count)
	return count, err
}

// GetCriticalCountAt returns the number of critical findings that were open at a specific time.
func (r *ProductRepo) GetCriticalCountAt(ctx context.Context, productID string, atTime time.Time) (int, error) {
	pid, err := uuid.Parse(productID)
	if err != nil {
		return 0, err
	}
	var count int
	err = r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM findings
		 WHERE product_id=$1 AND severity='Critical' AND created_at <= $2
		 AND (mitigated_at IS NULL OR mitigated_at > $2)`,
		pid, atTime).Scan(&count)
	return count, err
}
