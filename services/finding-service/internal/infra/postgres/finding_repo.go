package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/finding"
	"github.com/osv/finding-service/internal/usecase"
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
		f.HashCode, f.Tags, f.InheritedTags, f.CreatedAt, f.UpdatedAt, f.CreatedBy,
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
			f.HashCode, f.Tags, f.InheritedTags, f.CreatedAt, f.UpdatedAt, f.CreatedBy,
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
		SELECT id, title, COALESCE(description, ''), COALESCE(mitigation, ''), COALESCE(impact, ''), severity,
		       numerical_severity, COALESCE(cve, ''), COALESCE(cwe, 0), active, verified, false_positive,
		       duplicate, out_of_scope, is_mitigated, risk_accepted,
		       date, mitigated_at, mitigated_by_id, sla_expiration_date,
		       test_id, engagement_id, product_id,
		       COALESCE(component_name, ''), COALESCE(component_version, ''), COALESCE(service, ''), COALESCE(file_path, ''), line_number,
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

func (r *FindingRepo) List(ctx context.Context, filter finding.FindingFilter) (*finding.FindingListResult, error) {
	q := `WITH filtered AS (
    SELECT
        f.id, f.title, COALESCE(f.description, '') AS description, COALESCE(f.cve, '') AS cve, f.severity,
        f.cvss_v3_score, f.epss_score, f.is_kev,
        f.is_mitigated, f.false_positive, f.risk_accepted,
        f.out_of_scope, f.duplicate, f.duplicate_finding_id,
        f.sla_expiration_date, f.created_at, f.updated_at,
        f.mitigated_at,
        COALESCE(f.assigned_to::text, '') AS assigned_to,
        COALESCE(f.created_by, '') AS created_by,
        COALESCE(f.component_name, '') AS component_name, COALESCE(f.component_version, '') AS component_version,
        f.asset_ip::text, f.asset_hostname,
        f.product_id, f.engagement_id, f.test_id,
        p.name AS product_name,
        ji.jira_key AS jira_issue_key,
        CASE WHEN ji.jira_key IS NOT NULL
             THEN jc.server_url || '/browse/' || ji.jira_key
        END AS jira_url
    FROM findings f
    LEFT JOIN products p ON p.id = f.product_id
    LEFT JOIN jira_issues ji ON ji.finding_id = f.id
    LEFT JOIN jira_configs jc ON jc.product_id = f.product_id
    WHERE
        ($1::uuid IS NULL OR f.product_id = $1)
        AND ($2::text IS NULL OR f.severity = $2)
        AND ($3::boolean IS FALSE OR (NOT f.is_mitigated AND NOT f.false_positive AND NOT f.risk_accepted AND NOT f.out_of_scope AND NOT f.duplicate))
),
agg AS (
    SELECT
        COUNT(*) AS total,
        COUNT(*) FILTER (WHERE severity = 'Critical') AS sev_critical,
        COUNT(*) FILTER (WHERE severity = 'High')     AS sev_high,
        COUNT(*) FILTER (WHERE severity = 'Medium')   AS sev_medium,
        COUNT(*) FILTER (WHERE severity = 'Low')      AS sev_low,
        COUNT(*) FILTER (WHERE NOT is_mitigated AND NOT false_positive AND NOT risk_accepted AND NOT out_of_scope) AS status_active,
        COUNT(*) FILTER (WHERE is_mitigated)           AS status_mitigated,
        COUNT(*) FILTER (WHERE false_positive)         AS status_fp,
        COUNT(*) FILTER (WHERE risk_accepted)          AS status_risk,
        COUNT(*) FILTER (WHERE sla_expiration_date < NOW() AND NOT is_mitigated) AS sla_breached,
        COUNT(*) FILTER (WHERE sla_expiration_date BETWEEN NOW() AND NOW() + INTERVAL '7 days' AND NOT is_mitigated) AS sla_at_risk
    FROM filtered
)
SELECT
    f.id, f.title, f.description, f.cve, f.severity,
    f.cvss_v3_score, f.epss_score, f.is_kev,
    f.is_mitigated, f.false_positive, f.risk_accepted,
    f.out_of_scope, f.duplicate, f.duplicate_finding_id,
    f.sla_expiration_date, f.created_at, f.updated_at,
    f.mitigated_at, f.assigned_to, f.created_by,
    f.component_name, f.component_version,
    f.asset_ip, f.asset_hostname,
    f.product_id, f.engagement_id, f.test_id,
    f.product_name, f.jira_issue_key, f.jira_url,
    a.total, a.sev_critical, a.sev_high, a.sev_medium, a.sev_low,
    a.status_active, a.status_mitigated, a.status_fp, a.status_risk,
    a.sla_breached, a.sla_at_risk
FROM filtered f, agg a
ORDER BY
    CASE f.severity WHEN 'Critical' THEN 0 WHEN 'High' THEN 1 WHEN 'Medium' THEN 2 ELSE 3 END,
    f.sla_expiration_date ASC NULLS LAST
LIMIT $4 OFFSET $5;`

	var sevFilter interface{}
	if len(filter.Severity) > 0 {
		sevFilter = string(filter.Severity[0])
	}

	rows, err := r.pool.Query(ctx, q,
		filter.ProductID, sevFilter, filter.ActiveOnly, filter.Limit, filter.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := &finding.FindingListResult{}
	first := true

	for rows.Next() {
		fm := &finding.FindingWithMeta{Finding: &finding.Finding{}}
		f := fm.Finding

		var sev string
		var assetIP *string
		var productName *string // NULL-safe: LEFT JOIN products may produce NULL
		// assigned_to and created_by are TEXT (COALESCE → always non-NULL string)
		var assignedTo, createdBy string
		err := rows.Scan(
			&f.ID, &f.Title, &f.Description, &f.CVE, &sev,
			&f.CVSSv3Score, &f.EPSSScore, &f.IsKEV,
			&f.IsMitigated, &f.FalsePositive, &f.RiskAccepted,
			&f.OutOfScope, &f.Duplicate, &f.DuplicateFindingID,
			&f.SLAExpirationDate, &f.CreatedAt, &f.UpdatedAt,
			&f.MitigatedAt, &assignedTo, &createdBy,
			&f.ComponentName, &f.ComponentVersion,
			&assetIP, &f.AssetHostname,
			&f.ProductID, &f.EngagementID, &f.TestID,
			&productName, &fm.JiraIssueKey, &fm.JiraURL,
			&res.Total, &res.SevCritical, &res.SevHigh, &res.SevMedium, &res.SevLow,
			&res.StatusActive, &res.StatusMitigated, &res.StatusFP, &res.StatusRisk,
			&res.SLABreached, &res.SLAAtRisk,
		)
		if err != nil {
			return nil, err
		}
		// Map intermediate string vars → *string fields on entity
		if assignedTo != "" {
			f.AssignedTo = &assignedTo
		}
		if createdBy != "" {
			f.CreatedBy = &createdBy
		}
		if productName != nil {
			fm.ProductName = *productName
		}
		f.Severity = finding.Severity(sev)
		f.AssetIP = assetIP
		res.Findings = append(res.Findings, fm)
		first = false
	}
	if first && rows.Err() == nil {
		// No findings found. We can't get aggregations because filtered CTE was empty.
		// That's fine, Total will be 0.
	}

	return res, rows.Err()
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
			       COALESCE(cve, ''), COALESCE(component_name, ''), COALESCE(component_version, ''), COALESCE(file_path, ''), hash_code, date, sla_expiration_date,
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
		id, title, description, mitigation, impact, "references",
		severity, numerical_severity, cve, cwe, vuln_id_from_tool,
		cvss_v3, cvss_v3_score, active, verified, false_positive,
		duplicate, out_of_scope, is_mitigated, risk_accepted,
		date, mitigated_at, mitigated_by_id, sla_expiration_date,
		test_id, engagement_id, product_id,
		component_name, component_version, service, file_path, line_number,
		hash_code, tags, inherited_tags, created_at, updated_at, created_by
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,$37,$38)`
}

// BulkReactivate marks findings as active again (e.g., when a previously-mitigated vuln reappears).
func (r *FindingRepo) BulkReactivate(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now().UTC()
	placeholders := make([]string, len(ids))
	args := []interface{}{true, false, now, now}
	for i, id := range ids {
		args = append(args, id)
		placeholders[i] = fmt.Sprintf("$%d", len(args))
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE findings SET active=$1, is_mitigated=$2, mitigated_at=NULL,
		 last_status_update=$3, updated_at=$4
		 WHERE id IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	)
	return err
}

// ExistsFalsePositiveByHash returns true if a false-positive finding with this hash code
// exists in the given product. Used by the dedup engine to auto-mark recurring FPs.
func (r *FindingRepo) ExistsFalsePositiveByHash(ctx context.Context, hashCode string, productID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM findings
			WHERE hash_code = $1 AND product_id = $2 AND false_positive = TRUE
		)`, hashCode, productID).Scan(&exists)
	return exists, err
}

// GetSeverityCounts returns counts of findings grouped by severity for a product.
// If activeOnly is true, only active findings are counted.
func (r *FindingRepo) GetSeverityCounts(ctx context.Context, productID uuid.UUID, activeOnly bool) (map[string]int, error) {
	q := `SELECT severity, COUNT(*) FROM findings
		WHERE product_id = $1`
	if activeOnly {
		q += ` AND active = TRUE`
	}
	q += ` GROUP BY severity`

	rows, err := r.pool.Query(ctx, q, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{
		"Critical": 0, "High": 0, "Medium": 0, "Low": 0, "Info": 0,
	}
	for rows.Next() {
		var sev string
		var count int
		if err := rows.Scan(&sev, &count); err != nil {
			return nil, err
		}
		counts[sev] = count
	}
	return counts, rows.Err()
}


// --- Dashboard Stats Methods ---
// These implement usecase.FindingRepository

func (r *FindingRepo) CountBySeverity(ctx context.Context) (*usecase.SeverityCounts, error) {
	q := `SELECT
		COUNT(*) FILTER (WHERE severity = 'Critical'),
		COUNT(*) FILTER (WHERE severity = 'High'),
		COUNT(*) FILTER (WHERE severity = 'Medium'),
		COUNT(*) FILTER (WHERE severity = 'Low')
	FROM findings
	WHERE active = true AND duplicate = false;`

	var counts usecase.SeverityCounts
	err := r.pool.QueryRow(ctx, q).Scan(&counts.Critical, &counts.High, &counts.Medium, &counts.Low)
	return &counts, err
}



func (r *FindingRepo) GetMonthlyTrend(ctx context.Context, interval string) ([]usecase.TrendPoint, error) {
	q := `SELECT
		TO_CHAR(DATE_TRUNC('month', created_at), 'YYYY-MM') AS month,
		COUNT(*) FILTER (WHERE severity = 'Critical'),
		COUNT(*) FILTER (WHERE severity = 'High'),
		COUNT(*) FILTER (WHERE severity = 'Medium'),
		COUNT(*) FILTER (WHERE severity = 'Low')
	FROM findings
	WHERE created_at >= NOW() - $1::interval
	GROUP BY month
	ORDER BY month ASC;`

	rows, err := r.pool.Query(ctx, q, interval)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trend []usecase.TrendPoint
	for rows.Next() {
		var p usecase.TrendPoint
		if err := rows.Scan(&p.Month, &p.Critical, &p.High, &p.Medium, &p.Low); err != nil {
			return nil, err
		}
		trend = append(trend, p)
	}
	return trend, rows.Err()
}

func (r *FindingRepo) GetProductGrades(ctx context.Context) ([]usecase.ProductGrade, error) {
	q := `SELECT
		p.id, p.name,
		COUNT(f.id) FILTER (WHERE f.severity = 'Critical' AND f.active AND NOT f.duplicate) AS critical_count,
		COUNT(f.id) FILTER (WHERE f.severity = 'High' AND f.active AND NOT f.duplicate) AS high_count
	FROM products p
	LEFT JOIN findings f ON f.product_id = p.id
	GROUP BY p.id, p.name
	ORDER BY critical_count DESC, high_count DESC;`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var grades []usecase.ProductGrade
	for rows.Next() {
		var g usecase.ProductGrade
		var id uuid.UUID
		if err := rows.Scan(&id, &g.Name, &g.CriticalCount, &g.HighCount); err != nil {
			return nil, err
		}
		g.ID = id.String()
		// Calculate grade from counts
		if g.CriticalCount > 0 {
			g.Grade = "F"
			g.Score = 30
		} else if g.HighCount > 5 {
			g.Grade = "D"
			g.Score = 50
		} else if g.HighCount > 0 {
			g.Grade = "C"
			g.Score = 65
		} else {
			g.Grade = "B"
			g.Score = 80
		}
		g.Trend = "stable" // Default trend
		grades = append(grades, g)
	}
	return grades, rows.Err()
}

func (r *FindingRepo) GetSLABreaches(ctx context.Context, limit int) ([]usecase.SLABreachItem, error) {
	q := `SELECT
		f.id, f.title, f.severity, p.name,
		EXTRACT(DAY FROM NOW() - f.sla_expiration_date)::INT,
		f.sla_expiration_date
	FROM findings f
	JOIN products p ON p.id = f.product_id
	WHERE f.active = true AND f.duplicate = false AND f.sla_expiration_date < NOW()
	ORDER BY f.sla_expiration_date ASC
	LIMIT $1;`

	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var breaches []usecase.SLABreachItem
	for rows.Next() {
		var b usecase.SLABreachItem
		var id uuid.UUID
		var exp time.Time
		if err := rows.Scan(&id, &b.Title, &b.Severity, &b.ProductName, &b.DaysOverdue, &exp); err != nil {
			return nil, err
		}
		b.FindingID = id.String()
		b.ExpiresAt = exp.Format(time.RFC3339)
		breaches = append(breaches, b)
	}
	return breaches, rows.Err()
}

func (r *FindingRepo) CountActiveByCVEIds(ctx context.Context, cveIds []string) (int, error) {
	if len(cveIds) == 0 {
		return 0, nil
	}
	q := `SELECT COUNT(DISTINCT id) FROM findings WHERE active = true AND cve = ANY($1)`
	var count int
	err := r.pool.QueryRow(ctx, q, cveIds).Scan(&count)
	return count, err
}

func (r *FindingRepo) BulkUpdateAssignee(ctx context.Context, findingIDs []string, assignedTo string) (int, error) {
	if len(findingIDs) == 0 {
		return 0, nil
	}
	
	// Convert strings to uuids
	uuids := make([]uuid.UUID, 0, len(findingIDs))
	for _, idStr := range findingIDs {
		if id, err := uuid.Parse(idStr); err == nil {
			uuids = append(uuids, id)
		}
	}
	if len(uuids) == 0 {
		return 0, nil
	}

	q := `UPDATE findings
		SET assigned_to = $2, updated_at = NOW()
		WHERE id = ANY($1::uuid[])
		RETURNING id;`

	rows, err := r.pool.Query(ctx, q, uuids, assignedTo)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	return count, rows.Err()
}

func (r *FindingRepo) GetStats(ctx context.Context, productID string) (map[string]interface{}, error) {
	var pid *uuid.UUID
	if productID != "" {
		id, err := uuid.Parse(productID)
		if err == nil {
			pid = &id
		}
	}
	var count int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM findings WHERE ($1::uuid IS NULL OR product_id = $1)", pid).Scan(&count)
	return map[string]interface{}{"total_findings": count}, err
}

// CloseOldFindings marks as inactive all findings in a test/engagement/product
// whose hash codes are NOT in activeHashes (i.e., they disappeared from the latest scan).
func (r *FindingRepo) CloseOldFindings(ctx context.Context, testID, engagementID, productID string, activeHashes []string) (int, error) {
	tid, _ := uuid.Parse(testID)
	pid, _ := uuid.Parse(productID)

	var q string
	var args []interface{}
	now := time.Now().UTC()

	if len(activeHashes) == 0 {
		// Close all findings in the test
		q = `UPDATE findings SET active=false, is_mitigated=true, mitigated_at=$1, updated_at=$1
		     WHERE test_id=$2 AND product_id=$3 AND active=true`
		args = []interface{}{now, tid, pid}
	} else {
		q = `UPDATE findings SET active=false, is_mitigated=true, mitigated_at=$1, updated_at=$1
		     WHERE test_id=$2 AND product_id=$3 AND active=true
		       AND hash_code <> ALL($4::text[])`
		args = []interface{}{now, tid, pid, activeHashes}
	}

	tag, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// ReactivateByHashes marks findings matching the given hashes as active again.
func (r *FindingRepo) ReactivateByHashes(ctx context.Context, hashes []string, productID string) (int, error) {
	if len(hashes) == 0 {
		return 0, nil
	}
	pid, err := uuid.Parse(productID)
	if err != nil {
		return 0, fmt.Errorf("invalid product_id: %w", err)
	}
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx, `
		UPDATE findings SET active=true, is_mitigated=false, mitigated_at=NULL, updated_at=$1
		WHERE hash_code = ANY($2::text[]) AND product_id=$3 AND active=false`,
		now, hashes, pid)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// ApplyTags adds tags to specified findings (merge with existing).
func (r *FindingRepo) ApplyTags(ctx context.Context, findingIDs []string, tags []string) (int, error) {
	if len(findingIDs) == 0 || len(tags) == 0 {
		return 0, nil
	}
	uuids := make([]uuid.UUID, 0, len(findingIDs))
	for _, idStr := range findingIDs {
		if id, err := uuid.Parse(idStr); err == nil {
			uuids = append(uuids, id)
		}
	}
	if len(uuids) == 0 {
		return 0, nil
	}
	now := time.Now().UTC()
	// Merge tags using array union
	tag, err := r.pool.Exec(ctx, `
		UPDATE findings
		SET tags = ARRAY(SELECT DISTINCT unnest(tags || $1::text[])),
		    updated_at = $2
		WHERE id = ANY($3::uuid[])`,
		tags, now, uuids)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// CountFindings returns total count of findings for a product.
func (r *FindingRepo) Count(ctx context.Context, productID string, activeOnly bool) (int64, error) {
	pid, err := uuid.Parse(productID)
	if err != nil {
		return 0, fmt.Errorf("invalid product_id: %w", err)
	}
	q := `SELECT COUNT(*) FROM findings WHERE product_id=$1`
	if activeOnly {
		q += ` AND active=true`
	}
	var count int64
	err = r.pool.QueryRow(ctx, q, pid).Scan(&count)
	return count, err
}

// CountBySeverityForProduct returns counts per severity for gRPC use.
func (r *FindingRepo) CountBySeverityForProduct(ctx context.Context, productID string, activeOnly bool) (critical, high, medium, low, info int32, err error) {
	pid, parseErr := uuid.Parse(productID)
	if parseErr != nil {
		err = fmt.Errorf("invalid product_id: %w", parseErr)
		return
	}
	q := `SELECT
		COUNT(*) FILTER (WHERE severity='Critical'),
		COUNT(*) FILTER (WHERE severity='High'),
		COUNT(*) FILTER (WHERE severity='Medium'),
		COUNT(*) FILTER (WHERE severity='Low'),
		COUNT(*) FILTER (WHERE severity='Info')
	FROM findings WHERE product_id=$1`
	if activeOnly {
		q += ` AND active=true`
	}
	err = r.pool.QueryRow(ctx, q, pid).Scan(&critical, &high, &medium, &low, &info)
	return
}
