package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
	"github.com/defectdojo/finding-service/internal/domain/finding"
)

// FindingRepo implements finding.Repository using pgxpool.
type FindingRepo struct {
	pool *pgxpool.Pool
}

func NewFindingRepo(pool *pgxpool.Pool) *FindingRepo {
	return &FindingRepo{pool: pool}
}

func (r *FindingRepo) Create(ctx context.Context, f *finding.Finding) error {
	_, err := r.pool.Exec(ctx, insertSQL(),
		f.ID, f.Title, f.Description, f.Mitigation, f.Impact, f.References,
		string(f.Severity), f.NumericalSeverity, f.CVE, f.CWE, f.VulnIDFromTool,
		f.CVSSv3, f.CVSSv3Score, f.Active, f.Verified, f.FalsePositive,
		f.Duplicate, f.OutOfScope, f.IsMitigated, f.RiskAccepted,
		f.Date, f.MitigatedAt, f.MitigatedByID, f.SLAExpirationDate,
		f.TestID, f.EngagementID, f.ProductID,
		f.ComponentName, f.ComponentVersion, f.Service, f.FilePath, f.LineNumber,
		f.HashCode, f.Tags, f.InheritedTags, f.CreatedAt, f.UpdatedAt,
	)
	return err
}

func (r *FindingRepo) BulkCreate(ctx context.Context, findings []*finding.Finding) ([]string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	ids := make([]string, 0, len(findings))
	for _, f := range findings {
		_, err := tx.Exec(ctx, insertSQL(),
			f.ID, f.Title, f.Description, f.Mitigation, f.Impact, f.References,
			string(f.Severity), f.NumericalSeverity, f.CVE, f.CWE, f.VulnIDFromTool,
			f.CVSSv3, f.CVSSv3Score, f.Active, f.Verified, f.FalsePositive,
			f.Duplicate, f.OutOfScope, f.IsMitigated, f.RiskAccepted,
			f.Date, f.MitigatedAt, f.MitigatedByID, f.SLAExpirationDate,
			f.TestID, f.EngagementID, f.ProductID,
			f.ComponentName, f.ComponentVersion, f.Service, f.FilePath, f.LineNumber,
			f.HashCode, f.Tags, f.InheritedTags, f.CreatedAt, f.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("bulk insert finding %s: %w", f.ID, err)
		}
		ids = append(ids, f.ID.String())
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *FindingRepo) FindByID(ctx context.Context, id uuid.UUID) (*finding.Finding, error) {
	f := &finding.Finding{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, title, description, mitigation, impact, severity,
		       numerical_severity, cve, cwe, active, verified, false_positive,
		       duplicate, out_of_scope, is_mitigated, risk_accepted,
		       date, mitigated_at, mitigated_by_id, sla_expiration_date,
		       test_id, engagement_id, product_id,
		       component_name, component_version, service, file_path, line_number,
		       hash_code, tags, created_at, updated_at
		FROM findings WHERE id = $1`, id).Scan(
		&f.ID, &f.Title, &f.Description, &f.Mitigation, &f.Impact, &f.Severity,
		&f.NumericalSeverity, &f.CVE, &f.CWE, &f.Active, &f.Verified, &f.FalsePositive,
		&f.Duplicate, &f.OutOfScope, &f.IsMitigated, &f.RiskAccepted,
		&f.Date, &f.MitigatedAt, &f.MitigatedByID, &f.SLAExpirationDate,
		&f.TestID, &f.EngagementID, &f.ProductID,
		&f.ComponentName, &f.ComponentVersion, &f.Service, &f.FilePath, &f.LineNumber,
		&f.HashCode, &f.Tags, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("finding not found: %w", err)
	}
	return f, nil
}

func (r *FindingRepo) FindByHashCode(ctx context.Context, hashCode string, testID uuid.UUID, onEngagement bool, engagementID *uuid.UUID, productID *uuid.UUID) (*finding.Finding, error) {
	var q string
	var args []interface{}

	if onEngagement && engagementID != nil {
		q = `SELECT id, severity, active, false_positive, is_mitigated, risk_accepted, duplicate, out_of_scope, test_id, engagement_id, product_id, hash_code, date
			 FROM findings WHERE hash_code = $1 AND engagement_id = $2 LIMIT 1`
		args = []interface{}{hashCode, *engagementID}
	} else if productID != nil {
		q = `SELECT id, severity, active, false_positive, is_mitigated, risk_accepted, duplicate, out_of_scope, test_id, engagement_id, product_id, hash_code, date
			 FROM findings WHERE hash_code = $1 AND product_id = $2 LIMIT 1`
		args = []interface{}{hashCode, *productID}
	} else {
		q = `SELECT id, severity, active, false_positive, is_mitigated, risk_accepted, duplicate, out_of_scope, test_id, engagement_id, product_id, hash_code, date
			 FROM findings WHERE hash_code = $1 AND test_id = $2 LIMIT 1`
		args = []interface{}{hashCode, testID}
	}

	f := &finding.Finding{}
	err := r.pool.QueryRow(ctx, q, args...).Scan(
		&f.ID, &f.Severity, &f.Active, &f.FalsePositive, &f.IsMitigated, &f.RiskAccepted,
		&f.Duplicate, &f.OutOfScope, &f.TestID, &f.EngagementID, &f.ProductID, &f.HashCode, &f.Date,
	)
	if err != nil {
		return nil, fmt.Errorf("not found: %w", err)
	}
	return f, nil
}

func (r *FindingRepo) FindActiveByTest(ctx context.Context, testID uuid.UUID, excludeIDs []uuid.UUID) ([]*finding.Finding, error) {
	q := `SELECT id FROM findings WHERE test_id = $1 AND active = TRUE`
	args := []interface{}{testID}
	if len(excludeIDs) > 0 {
		placeholders := make([]string, len(excludeIDs))
		for i, id := range excludeIDs {
			args = append(args, id)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		q += " AND id NOT IN (" + strings.Join(placeholders, ",") + ")"
	}

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var findings []*finding.Finding
	for rows.Next() {
		f := &finding.Finding{}
		if err := rows.Scan(&f.ID); err != nil {
			return nil, err
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}

func (r *FindingRepo) List(ctx context.Context, filter finding.FindingFilter) ([]*finding.Finding, int, error) {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM findings WHERE product_id = COALESCE($1, product_id)`,
		filter.ProductID).Scan(&total)

	rows, err := r.pool.Query(ctx, `
		SELECT id, title, severity, active, verified, duplicate, is_mitigated, date, sla_expiration_date,
		       product_id, test_id, cve, hash_code, created_at
		FROM findings
		WHERE ($1::uuid IS NULL OR product_id = $1)
		  AND ($2::uuid IS NULL OR engagement_id = $2)
		  AND ($3::boolean IS FALSE OR active = TRUE)
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5`,
		filter.ProductID, filter.EngagementID, filter.ActiveOnly, filter.Limit, filter.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var findings []*finding.Finding
	for rows.Next() {
		f := &finding.Finding{}
		if err := rows.Scan(&f.ID, &f.Title, &f.Severity, &f.Active, &f.Verified,
			&f.Duplicate, &f.IsMitigated, &f.Date, &f.SLAExpirationDate,
			&f.ProductID, &f.TestID, &f.CVE, &f.HashCode, &f.CreatedAt); err != nil {
			return nil, 0, err
		}
		findings = append(findings, f)
	}
	return findings, total, rows.Err()
}

func (r *FindingRepo) Save(ctx context.Context, f *finding.Finding) error {
	f.UpdatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE findings SET
			active=$2, verified=$3, false_positive=$4, duplicate=$5,
			out_of_scope=$6, is_mitigated=$7, risk_accepted=$8,
			mitigated_at=$9, mitigated_by_id=$10, last_status_update=NOW(),
			sla_expiration_date=$11, updated_at=$12
		WHERE id=$1`,
		f.ID, f.Active, f.Verified, f.FalsePositive, f.Duplicate,
		f.OutOfScope, f.IsMitigated, f.RiskAccepted,
		f.MitigatedAt, f.MitigatedByID, f.SLAExpirationDate, f.UpdatedAt,
	)
	return err
}

func (r *FindingRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM findings WHERE id=$1`, id)
	return err
}

func (r *FindingRepo) BulkSetMitigated(ctx context.Context, ids []uuid.UUID, mitigatedByID uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now().UTC()
	placeholders := make([]string, len(ids))
	args := []interface{}{false, true, now, mitigatedByID, now}
	for i, id := range ids {
		args = append(args, id)
		placeholders[i] = fmt.Sprintf("$%d", len(args))
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE findings SET active=$1, is_mitigated=$2, mitigated_at=$3, mitigated_by_id=$4, updated_at=$5
		 WHERE id IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	)
	return err
}

func (r *FindingRepo) BulkUpdateSLADates(ctx context.Context, updates []finding.SLADateUpdate) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, u := range updates {
		var expDate *time.Time
		if t, ok := u.ExpirationDate.(time.Time); ok {
			expDate = &t
		}
		_, err := tx.Exec(ctx,
			`UPDATE findings SET sla_expiration_date=$2, updated_at=NOW() WHERE id=$1`,
			u.FindingID, expDate,
		)
		if err != nil {
			return fmt.Errorf("update sla date for %s: %w", u.FindingID, err)
		}
	}
	return tx.Commit(ctx)
}

func (r *FindingRepo) ListForSLACheck(ctx context.Context, ids []uuid.UUID, activeOnly, hasSLADate bool) ([]*finding.Finding, error) {
	var q string
	var args []interface{}

	if len(ids) > 0 {
		placeholders := make([]string, len(ids))
		for i, id := range ids {
			args = append(args, id)
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		q = `SELECT id, severity, date, product_id, sla_expiration_date FROM findings WHERE id IN (` +
			strings.Join(placeholders, ",") + `)`
	} else {
		q = `SELECT id, severity, date, product_id, sla_expiration_date FROM findings WHERE 1=1`
		if activeOnly {
			q += ` AND active = TRUE`
		}
		if hasSLADate {
			q += ` AND sla_expiration_date IS NOT NULL`
		}
	}

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var findings []*finding.Finding
	for rows.Next() {
		f := &finding.Finding{}
		if err := rows.Scan(&f.ID, &f.Severity, &f.Date, &f.ProductID, &f.SLAExpirationDate); err != nil {
			return nil, err
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}

// ListForReport streams findings via a channel to avoid loading all into memory.
func (r *FindingRepo) ListForReport(ctx context.Context, filter finding.FindingFilter) (<-chan *finding.Finding, error) {
	ch := make(chan *finding.Finding, 50)

	go func() {
		defer close(ch)
		rows, err := r.pool.Query(ctx, `
			SELECT id, title, severity, active, verified, duplicate, is_mitigated,
			       cve, component_name, component_version, file_path, hash_code, date, sla_expiration_date,
			       product_id, test_id, engagement_id
			FROM findings
			WHERE ($1::uuid IS NULL OR product_id = $1)
			  AND ($2::boolean IS FALSE OR active = TRUE)
			ORDER BY severity DESC, date DESC`,
			filter.ProductID, filter.ActiveOnly,
		)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			if ctx.Err() != nil {
				return
			}
			f := &finding.Finding{}
			if err := rows.Scan(&f.ID, &f.Title, &f.Severity, &f.Active, &f.Verified, &f.Duplicate,
				&f.IsMitigated, &f.CVE, &f.ComponentName, &f.ComponentVersion, &f.FilePath, &f.HashCode,
				&f.Date, &f.SLAExpirationDate, &f.ProductID, &f.TestID, &f.EngagementID); err != nil {
				return
			}
			ch <- f
		}
	}()
	return ch, nil
}

// insertSQL returns the full INSERT statement for findings.
func insertSQL() string {
	return `INSERT INTO findings (
		id, title, description, mitigation, impact, references,
		severity, numerical_severity, cve, cwe, vuln_id_from_tool,
		cvss_v3, cvss_v3_score, active, verified, false_positive,
		duplicate, out_of_scope, is_mitigated, risk_accepted,
		date, mitigated_at, mitigated_by_id, sla_expiration_date,
		test_id, engagement_id, product_id,
		component_name, component_version, service, file_path, line_number,
		hash_code, tags, inherited_tags, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,$37)`
}
