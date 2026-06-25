// Package postgres — vendor_repo.go
// PostgreSQL implementation of VendorRepository.
// Queries vendor/product data from the `cves` table.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	httpdelivery "github.com/osv/data-service/internal/delivery/http"
)

// VendorRepo implements httpdelivery.VendorRepository via PostgreSQL.
type VendorRepo struct {
	db *pgxpool.Pool
}

// NewVendorRepo creates a new VendorRepo.
func NewVendorRepo(db *pgxpool.Pool) *VendorRepo {
	return &VendorRepo{db: db}
}

// AutocompleteVendors implements the existing VendorRepository interface used by VendorHandler.
func (r *VendorRepo) AutocompleteVendors(ctx context.Context, prefix string, limit int) ([]string, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT vendor
		FROM cves
		WHERE vendor IS NOT NULL AND vendor != ''
		  AND ($1 = '' OR vendor ILIKE $1 || '%')
		ORDER BY vendor
		LIMIT $2
	`, prefix, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vendors []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			continue
		}
		vendors = append(vendors, v)
	}
	return vendors, rows.Err()
}

// GetVendors returns vendors with CVE counts, filtered by optional query string.
func (r *VendorRepo) GetVendors(ctx context.Context, q string, limit int) ([]httpdelivery.VendorEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT vendor, COUNT(*) AS cve_count
		FROM cves
		WHERE vendor IS NOT NULL AND vendor != ''
		  AND ($1 = '' OR vendor ILIKE '%' || $1 || '%')
		GROUP BY vendor
		ORDER BY cve_count DESC
		LIMIT $2
	`, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vendors []httpdelivery.VendorEntry
	for rows.Next() {
		var v httpdelivery.VendorEntry
		if err := rows.Scan(&v.Vendor, &v.CVECount); err != nil {
			continue
		}
		vendors = append(vendors, v)
	}
	return vendors, rows.Err()
}

// GetProductsByVendor returns products for a vendor with CVE counts, paginated.
func (r *VendorRepo) GetProductsByVendor(ctx context.Context, vendor string, page, pageSize int) ([]httpdelivery.ProductEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int
	r.db.QueryRow(ctx, `  //nolint:errcheck
		SELECT COUNT(DISTINCT product)
		FROM cves
		WHERE vendor = $1 AND product IS NOT NULL AND product != ''
	`, vendor).Scan(&total) //nolint:errcheck

	rows, err := r.db.Query(ctx, `
		SELECT product, vendor, COUNT(*) AS cve_count
		FROM cves
		WHERE vendor = $1 AND product IS NOT NULL AND product != ''
		GROUP BY product, vendor
		ORDER BY cve_count DESC
		LIMIT $2 OFFSET $3
	`, vendor, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var products []httpdelivery.ProductEntry
	for rows.Next() {
		var p httpdelivery.ProductEntry
		if err := rows.Scan(&p.Product, &p.Vendor, &p.CVECount); err != nil {
			continue
		}
		products = append(products, p)
	}
	return products, total, rows.Err()
}

// GetCVEsByVendorProduct returns CVEs for a given vendor+product pair, paginated.
func (r *VendorRepo) GetCVEsByVendorProduct(ctx context.Context, vendor, product string, page, pageSize int) ([]httpdelivery.CVEBrowseEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int
	r.db.QueryRow(ctx, `  //nolint:errcheck
		SELECT COUNT(*) FROM cves WHERE vendor = $1 AND product = $2
	`, vendor, product).Scan(&total) //nolint:errcheck

	rows, err := r.db.Query(ctx, `
		SELECT
			cve_id,
			COALESCE(severity_v3, '')          AS severity_v3,
			COALESCE(cvss_v3_score, 0)         AS cvss_v3_score,
			COALESCE(is_kev, false)             AS is_kev,
			COALESCE(TO_CHAR(published_at, 'YYYY-MM-DD'), '') AS published_at
		FROM cves
		WHERE vendor = $1 AND product = $2
		ORDER BY published_at DESC NULLS LAST
		LIMIT $3 OFFSET $4
	`, vendor, product, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var cves []httpdelivery.CVEBrowseEntry
	for rows.Next() {
		var c httpdelivery.CVEBrowseEntry
		if err := rows.Scan(&c.CveID, &c.SeverityV3, &c.CVSSScore, &c.IsKEV, &c.PublishedAt); err != nil {
			continue
		}
		cves = append(cves, c)
	}
	return cves, total, rows.Err()
}
