package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/osv/search-service/internal/domain/repository"
)

type pgVendorRepository struct{ db *sqlx.DB }

func NewVendorRepository(db *sqlx.DB) repository.VendorRepository {
	return &pgVendorRepository{db: db}
}

func (r *pgVendorRepository) ListVendors(ctx context.Context, q string, limit int) ([]*repository.VendorEntry, int64, error) {
	args := []interface{}{limit}
	where := ""
	if q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		where = fmt.Sprintf("WHERE lower(vendor) LIKE $%d", len(args))
	}

	var entries []*repository.VendorEntry
	query := fmt.Sprintf(`
        SELECT vendor, COUNT(DISTINCT product) as product_count
        FROM cpe_dict
        %s
        GROUP BY vendor
        ORDER BY vendor
        LIMIT $1
    `, where)
	err := r.db.SelectContext(ctx, &entries, query, args...)

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(DISTINCT vendor) FROM cpe_dict %s", where)
	if len(args) > 1 {
		r.db.QueryRowContext(ctx, countQuery, args[1:]...).Scan(&total)
	} else {
		r.db.QueryRowContext(ctx, countQuery).Scan(&total)
	}

	return entries, total, err
}

func (r *pgVendorRepository) GetProductsByVendor(ctx context.Context, vendor string) ([]string, error) {
	var products []string
	err := r.db.SelectContext(ctx, &products, `
        SELECT DISTINCT product
        FROM cpe_dict
        WHERE lower(vendor) = lower($1)
        ORDER BY product
    `, vendor)
	return products, err
}

func (r *pgVendorRepository) ListProducts(ctx context.Context, vendor, q string, limit int) ([]string, error) {
	args := []interface{}{strings.ToLower(vendor), limit}
	where := "WHERE lower(vendor) = $1"
	if q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		where += fmt.Sprintf(" AND lower(product) LIKE $%d", len(args))
	}

	var products []string
	query := fmt.Sprintf("SELECT DISTINCT product FROM cpe_dict %s ORDER BY product LIMIT $2", where)
	err := r.db.SelectContext(ctx, &products, query, args...)
	return products, err
}
