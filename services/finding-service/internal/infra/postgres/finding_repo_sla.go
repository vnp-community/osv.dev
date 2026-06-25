package postgres

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	httpdelivery "github.com/osv/finding-service/internal/delivery/http"
)

// GetSLASummary returns SLA summary stats.
func (r *FindingRepo) GetSLASummary(ctx context.Context, productID string) (*httpdelivery.SLASummaryData, error) {
	q := `
		SELECT
		    COALESCE(ROUND(100.0 * COUNT(*) FILTER (WHERE sla_expiration_date > NOW() OR sla_expiration_date IS NULL)
		          / NULLIF(COUNT(*), 0), 2), 100.0) AS compliance_pct,
		    COUNT(*) FILTER (WHERE sla_expiration_date < NOW() AND NOT is_mitigated) AS breached,
		    COUNT(*) FILTER (WHERE sla_expiration_date BETWEEN NOW() AND NOW() + INTERVAL '7 days' AND NOT is_mitigated) AS at_risk,
		    COUNT(*) FILTER (WHERE (sla_expiration_date > NOW() + INTERVAL '7 days' OR sla_expiration_date IS NULL) AND NOT is_mitigated) AS on_time,
		    COUNT(*) AS total_active_findings
		FROM findings
		WHERE active = true AND duplicate = false
		  AND ($1::uuid IS NULL OR product_id = $1::uuid)
	`
	var pID *uuid.UUID
	if productID != "" {
		parsed, err := uuid.Parse(productID)
		if err == nil {
			pID = &parsed
		}
	}

	var d httpdelivery.SLASummaryData
	err := r.pool.QueryRow(ctx, q, pID).Scan(&d.CompliancePct, &d.Breached, &d.AtRisk, &d.OnTime, &d.TotalActiveFindings)
	if err != nil {
		return nil, fmt.Errorf("finding_repo sla_summary: %w", err)
	}
	d.Ok = d.Breached == 0
	return &d, nil
}

// GetSLAComplianceTrend returns the compliance trend over the last N months.
func (r *FindingRepo) GetSLAComplianceTrend(ctx context.Context, productID string, months int) ([]httpdelivery.SLATrendPoint, error) {
	q := `
		SELECT
		    TO_CHAR(DATE_TRUNC('month', checked_at), 'YYYY-MM') AS month,
		    COALESCE(AVG(compliance_pct), 100.0)::numeric(5,2) AS compliance_pct
		FROM sla_snapshots
		WHERE checked_at >= NOW() - ($1 || ' months')::interval
		  AND ($2::uuid IS NULL OR product_id = $2::uuid)
		GROUP BY month ORDER BY month ASC
	`
	var pID *uuid.UUID
	if productID != "" {
		parsed, err := uuid.Parse(productID)
		if err == nil {
			pID = &parsed
		}
	}

	rows, err := r.pool.Query(ctx, q, months, pID)
	if err != nil {
		// sla_snapshots may not exist (partial migration) — fallback to findings-based trend
		return r.getSLAComplianceTrendFromFindings(ctx, productID, months)
	}
	defer rows.Close()

	var res []httpdelivery.SLATrendPoint
	for rows.Next() {
		var pt httpdelivery.SLATrendPoint
		if err := rows.Scan(&pt.Month, &pt.CompliancePct); err == nil {
			res = append(res, pt)
		}
	}
	if err := rows.Err(); err != nil {
		return r.getSLAComplianceTrendFromFindings(ctx, productID, months)
	}
	return res, nil
}

// getSLAComplianceTrendFromFindings computes compliance trend from findings table
// when sla_snapshots is unavailable. Calculates monthly on-time rate.
func (r *FindingRepo) getSLAComplianceTrendFromFindings(ctx context.Context, productID string, months int) ([]httpdelivery.SLATrendPoint, error) {
	q := `
		SELECT
		    TO_CHAR(DATE_TRUNC('month', f.created_at), 'YYYY-MM') AS month,
		    COALESCE(
		        ROUND(
		            100.0 * COUNT(*) FILTER (WHERE f.sla_expiration_date > f.created_at OR f.sla_expiration_date IS NULL)
		            / NULLIF(COUNT(*), 0)
		        , 2)
		    , 100.0) AS compliance_pct
		FROM findings f
		WHERE f.active = true
		  AND f.duplicate = false
		  AND f.created_at >= NOW() - ($1 || ' months')::interval
		  AND ($2::uuid IS NULL OR f.product_id = $2::uuid)
		GROUP BY month
		ORDER BY month ASC
	`
	var pID *uuid.UUID
	if productID != "" {
		parsed, err := uuid.Parse(productID)
		if err == nil {
			pID = &parsed
		}
	}
	rows, err := r.pool.Query(ctx, q, months, pID)
	if err != nil {
		return []httpdelivery.SLATrendPoint{}, nil
	}
	defer rows.Close()

	var res []httpdelivery.SLATrendPoint
	for rows.Next() {
		var pt httpdelivery.SLATrendPoint
		if err := rows.Scan(&pt.Month, &pt.CompliancePct); err == nil {
			res = append(res, pt)
		}
	}
	return res, rows.Err()
}

// GetBreachedFindings returns paginated findings that have breached SLA.
func (r *FindingRepo) GetBreachedFindings(ctx context.Context, productID string, page, pageSize int) ([]httpdelivery.SLAFindingItem, error) {
	q := `
		SELECT f.id::text, f.title, f.severity, p.name AS product_name,
		       EXTRACT(DAY FROM NOW() - f.sla_expiration_date)::int AS days_left,
		       COALESCE(TO_CHAR(f.sla_expiration_date, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '') AS expires_at
		FROM findings f JOIN products p ON p.id = f.product_id
		WHERE f.active AND NOT f.duplicate AND f.sla_expiration_date < NOW()
		  AND ($1::uuid IS NULL OR f.product_id = $1::uuid)
		ORDER BY f.sla_expiration_date ASC
		LIMIT $2 OFFSET ($3 - 1) * $2
	`
	var pID *uuid.UUID
	if productID != "" {
		parsed, err := uuid.Parse(productID)
		if err == nil {
			pID = &parsed
		}
	}

	rows, err := r.pool.Query(ctx, q, pID, pageSize, page)
	if err != nil {
		return nil, fmt.Errorf("finding_repo breached_findings: %w", err)
	}
	defer rows.Close()

	var res []httpdelivery.SLAFindingItem
	for rows.Next() {
		var item httpdelivery.SLAFindingItem
		if err := rows.Scan(&item.FindingID, &item.Title, &item.Severity, &item.ProductName, &item.DaysLeft, &item.ExpiresAt); err == nil {
			res = append(res, item)
		}
	}
	return res, nil
}

// GetAtRiskFindings returns findings at risk of breaching SLA within 7 days.
func (r *FindingRepo) GetAtRiskFindings(ctx context.Context, productID string) ([]httpdelivery.SLAFindingItem, error) {
	q := `
		SELECT f.id::text, f.title, f.severity, p.name AS product_name,
		       EXTRACT(DAY FROM f.sla_expiration_date - NOW())::int AS days_left,
		       COALESCE(TO_CHAR(f.sla_expiration_date, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '') AS expires_at
		FROM findings f JOIN products p ON p.id = f.product_id
		WHERE f.active AND NOT f.duplicate 
		  AND f.sla_expiration_date BETWEEN NOW() AND NOW() + INTERVAL '7 days'
		  AND ($1::uuid IS NULL OR f.product_id = $1::uuid)
		ORDER BY f.sla_expiration_date ASC
		LIMIT 50
	`
	var pID *uuid.UUID
	if productID != "" {
		parsed, err := uuid.Parse(productID)
		if err == nil {
			pID = &parsed
		}
	}

	rows, err := r.pool.Query(ctx, q, pID)
	if err != nil {
		return nil, fmt.Errorf("finding_repo at_risk_findings: %w", err)
	}
	defer rows.Close()

	var res []httpdelivery.SLAFindingItem
	for rows.Next() {
		var item httpdelivery.SLAFindingItem
		if err := rows.Scan(&item.FindingID, &item.Title, &item.Severity, &item.ProductName, &item.DaysLeft, &item.ExpiresAt); err == nil {
			res = append(res, item)
		}
	}
	return res, nil
}

// GetSLAByProduct returns SLA compliance overview for all products.
func (r *FindingRepo) GetSLAByProduct(ctx context.Context) ([]httpdelivery.ProductSLAData, error) {
	q := `
		SELECT 
		    p.id::text AS product_id,
		    p.name AS product_name,
		    COALESCE(ROUND(100.0 * COUNT(f.id) FILTER (WHERE f.sla_expiration_date > NOW() OR f.sla_expiration_date IS NULL)
		          / NULLIF(COUNT(f.id), 0), 2), 100.0) AS compliance_pct,
		    COUNT(f.id) FILTER (WHERE f.sla_expiration_date < NOW() AND NOT f.is_mitigated) AS breached,
		    COUNT(f.id) FILTER (WHERE f.sla_expiration_date BETWEEN NOW() AND NOW() + INTERVAL '7 days' AND NOT f.is_mitigated) AS at_risk
		FROM products p
		LEFT JOIN findings f ON p.id = f.product_id AND f.active = true AND f.duplicate = false
		GROUP BY p.id, p.name
		ORDER BY compliance_pct ASC
	`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("finding_repo sla_by_product: %w", err)
	}
	defer rows.Close()

	var res []httpdelivery.ProductSLAData
	for rows.Next() {
		var item httpdelivery.ProductSLAData
		if err := rows.Scan(&item.ProductID, &item.ProductName, &item.CompliancePct, &item.Breached, &item.AtRisk); err == nil {
			res = append(res, item)
		}
	}
	return res, nil
}
