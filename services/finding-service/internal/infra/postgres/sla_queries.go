// Package postgres — sla_queries.go
// Adds FindSLABreached and FindSLADueSoon methods to FindingRepo.
// These extend the existing FindingRepo (finding_repo.go) additively.
//
// Fields used: id, product_id, severity, sla_expiration_date, active, risk_accepted
// All confirmed from existing ListForReport scan columns.
package postgres

import (
	"context"
	"time"

	"github.com/osv/finding-service/internal/domain/finding"
)

// FindSLABreached returns active findings whose SLA date has already passed.
// Excludes risk_accepted findings (they have an exemption).
// Capped at 1000 rows to avoid OOM.
func (r *FindingRepo) FindSLABreached(ctx context.Context) ([]finding.Finding, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, product_id, severity, sla_expiration_date
		FROM findings
		WHERE sla_expiration_date < NOW()
		  AND active = TRUE
		  AND risk_accepted = FALSE
		ORDER BY sla_expiration_date ASC
		LIMIT 1000
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSLAFindings(rows)
}

// FindSLADueSoon returns active findings whose SLA expires before deadline.
// Only returns findings that are NOT already breached (sla_expiration_date >= NOW()).
func (r *FindingRepo) FindSLADueSoon(ctx context.Context, deadline time.Time) ([]finding.Finding, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, product_id, severity, sla_expiration_date
		FROM findings
		WHERE sla_expiration_date >= NOW()
		  AND sla_expiration_date <= $1
		  AND active = TRUE
		  AND risk_accepted = FALSE
		ORDER BY sla_expiration_date ASC
		LIMIT 1000
	`, deadline)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSLAFindings(rows)
}

// scanSLAFindings scans minimal fields needed for SLA publishing.
func scanSLAFindings(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]finding.Finding, error) {
	var findings []finding.Finding
	for rows.Next() {
		f := finding.Finding{}
		var slaDate *time.Time
		if err := rows.Scan(&f.ID, &f.ProductID, &f.Severity, &slaDate); err != nil {
			continue
		}
		f.SLAExpirationDate = slaDate
		findings = append(findings, f)
	}
	return findings, rows.Err()
}
