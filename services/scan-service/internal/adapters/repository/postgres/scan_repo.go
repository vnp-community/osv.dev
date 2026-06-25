package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/scan-service/internal/domain/entity"
	"github.com/osv/scan-service/internal/domain/repository"
)

// ScanRepo implements repository.ScanRepository using pgx/v5.
type ScanRepo struct{ db *pgxpool.Pool }

// NewScanRepo creates a ScanRepo backed by the given pgx pool.
func NewScanRepo(db *pgxpool.Pool) *ScanRepo { return &ScanRepo{db: db} }

func (r *ScanRepo) Create(ctx context.Context, s *entity.Scan) error {
	if s.ID == uuid.Nil { s.ID = uuid.New() }
	s.CreatedAt = time.Now().UTC()
	s.UpdatedAt = s.CreatedAt
	opts, _ := json.Marshal(s.Options)
	targets, _ := json.Marshal(s.Targets)
	_, err := r.db.Exec(ctx, `
		INSERT INTO scan.scans
		  (id, user_id, targets, scan_type, status, priority, options, created_at, updated_at)
		VALUES ($1,$2,$3::jsonb,$4,$5,$6,$7::jsonb,$8,$8)`,
		s.ID, s.UserID, targets, s.ScanType, s.Status, s.Priority, opts, s.CreatedAt,
	)
	return err
}

func (r *ScanRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.Scan, error) {
	var s entity.Scan
	var targets, opts []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, targets, scan_type, status, priority, options,
		       progress, finding_count, error_msg,
		       started_at, completed_at, failed_at, created_at, updated_at
		FROM scan.scans WHERE id=$1`, id).Scan(
		&s.ID, &s.UserID, &targets, &s.ScanType, &s.Status, &s.Priority, &opts,
		&s.Progress, &s.FindingCount, &s.ErrorMsg,
		&s.StartedAt, &s.CompletedAt, &s.FailedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil { return nil, err }
	json.Unmarshal(targets, &s.Targets)
	json.Unmarshal(opts, &s.Options)
	return &s, nil
}

func (r *ScanRepo) List(ctx context.Context, f repository.ScanFilter) ([]*entity.Scan, int64, error) {
	query := `SELECT id, user_id, scan_type, status, priority, progress, finding_count,
	           started_at, completed_at, created_at FROM scan.scans WHERE 1=1`
	args := []any{}
	n := 1
	if f.UserID != nil { query += fmt.Sprintf(" AND user_id=$%d", n); args = append(args, *f.UserID); n++ }
	if f.Status != nil { query += fmt.Sprintf(" AND status=$%d", n); args = append(args, *f.Status); n++ }
	if f.ScanType != nil { query += fmt.Sprintf(" AND scan_type=$%d", n); args = append(args, *f.ScanType); n++ }

	page := f.Page; if page < 1 { page = 1 }
	pageSize := f.PageSize; if pageSize < 1 { pageSize = 20 }
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", n, n+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil { return nil, 0, err }
	defer rows.Close()
	var scans []*entity.Scan
	for rows.Next() {
		var s entity.Scan
		rows.Scan(&s.ID, &s.UserID, &s.ScanType, &s.Status, &s.Priority, &s.Progress, &s.FindingCount, &s.StartedAt, &s.CompletedAt, &s.CreatedAt)
		scans = append(scans, &s)
	}
	return scans, int64(len(scans)), rows.Err()
}

func (r *ScanRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.ScanStatus) error {
	now := time.Now().UTC()
	var timeField string
	switch status {
	case entity.ScanStatusRunning:   timeField = "started_at"
	case entity.ScanStatusCompleted: timeField = "completed_at"
	case entity.ScanStatusFailed:    timeField = "failed_at"
	}
	query := fmt.Sprintf("UPDATE scan.scans SET status=$2, updated_at=$3 %s WHERE id=$1",
		func() string {
			if timeField != "" { return fmt.Sprintf(", %s=$3", timeField) }
			return ""
		}())
	_, err := r.db.Exec(ctx, query, id, status, now)
	return err
}

func (r *ScanRepo) UpdateProgress(ctx context.Context, id uuid.UUID, progress int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE scan.scans SET progress=$2, updated_at=NOW() WHERE id=$1`, id, progress)
	return err
}

func (r *ScanRepo) IncrementFindingCount(ctx context.Context, id uuid.UUID, delta int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE scan.scans SET finding_count=finding_count+$2, updated_at=NOW() WHERE id=$1`, id, delta)
	return err
}

func (r *ScanRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM scan.scans WHERE id=$1`, id)
	return err
}

// FindingRepo implements repository.FindingRepository.
type FindingRepo struct{ db *pgxpool.Pool }

func NewFindingRepo(db *pgxpool.Pool) *FindingRepo { return &FindingRepo{db: db} }

func (r *FindingRepo) CreateBatch(ctx context.Context, findings []*entity.Finding) error {
	for _, f := range findings {
		if f.ID == uuid.Nil { f.ID = uuid.New() }
		ports, _ := json.Marshal(f.OpenPorts)
		services, _ := json.Marshal(f.Services)
		cveIDs, _ := json.Marshal(f.CVEIDs)
		_, err := r.db.Exec(ctx, `
			INSERT INTO scan.findings
			  (id, scan_id, ip_address, hostname, os, open_ports, services, cve_ids, severity, created_at)
			VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8::jsonb,$9,NOW())
			ON CONFLICT (scan_id, ip_address) DO UPDATE SET
			  hostname=EXCLUDED.hostname, os=EXCLUDED.os,
			  open_ports=EXCLUDED.open_ports, services=EXCLUDED.services,
			  cve_ids=EXCLUDED.cve_ids, severity=EXCLUDED.severity`,
			f.ID, f.ScanID, f.IPAddress, f.Hostname, f.OS, ports, services, cveIDs, f.Severity,
		)
		if err != nil { return err }
	}
	return nil
}

func (r *FindingRepo) FindByScanID(ctx context.Context, scanID uuid.UUID, page, pageSize int) ([]*entity.Finding, int64, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 50 }
	rows, err := r.db.Query(ctx,
		`SELECT id, scan_id, ip_address, hostname, os, cve_ids, severity, created_at
		 FROM scan.findings WHERE scan_id=$1
		 ORDER BY created_at LIMIT $2 OFFSET $3`,
		scanID, pageSize, (page-1)*pageSize,
	)
	if err != nil { return nil, 0, err }
	defer rows.Close()
	var findings []*entity.Finding
	for rows.Next() {
		var f entity.Finding
		var cveIDs []byte
		rows.Scan(&f.ID, &f.ScanID, &f.IPAddress, &f.Hostname, &f.OS, &cveIDs, &f.Severity, &f.CreatedAt)
		json.Unmarshal(cveIDs, &f.CVEIDs)
		findings = append(findings, &f)
	}
	return findings, int64(len(findings)), rows.Err()
}

func (r *FindingRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.Finding, error) {
	var f entity.Finding
	var cveIDs, openPorts, services []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, scan_id, ip_address, hostname, os, open_ports, services, cve_ids, severity, created_at
		FROM scan.findings WHERE id=$1`, id).Scan(
		&f.ID, &f.ScanID, &f.IPAddress, &f.Hostname, &f.OS,
		&openPorts, &services, &cveIDs, &f.Severity, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(cveIDs, &f.CVEIDs)
	json.Unmarshal(openPorts, &f.OpenPorts)
	json.Unmarshal(services, &f.Services)
	return &f, nil
}

// WebAlertRepo implements repository.WebAlertRepository backed by scan.web_alerts.
type WebAlertRepo struct{ db *pgxpool.Pool }

func NewWebAlertRepo(db *pgxpool.Pool) *WebAlertRepo { return &WebAlertRepo{db: db} }

func (r *WebAlertRepo) CreateBatch(ctx context.Context, alerts []*entity.WebAlert) error {
	for _, a := range alerts {
		if a.ID == uuid.Nil {
			a.ID = uuid.New()
		}
		_, err := r.db.Exec(ctx, `
			INSERT INTO scan.web_alerts
			  (id, scan_id, target_url, alert_name, risk, confidence, description, solution, reference, evidence, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW())
			ON CONFLICT DO NOTHING`,
			a.ID, a.ScanID, a.TargetURL, a.AlertName,
			a.Risk, a.Confidence, a.Description, a.Solution, a.Reference, a.Evidence,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *WebAlertRepo) FindByScanID(ctx context.Context, scanID uuid.UUID) ([]*entity.WebAlert, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, scan_id, target_url, alert_name, risk, confidence, description, solution, reference, evidence, created_at
		FROM scan.web_alerts
		WHERE scan_id=$1
		ORDER BY risk DESC, created_at`,
		scanID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var alerts []*entity.WebAlert
	for rows.Next() {
		var a entity.WebAlert
		rows.Scan(&a.ID, &a.ScanID, &a.TargetURL, &a.AlertName,
			&a.Risk, &a.Confidence, &a.Description, &a.Solution, &a.Reference, &a.Evidence, &a.CreatedAt)
		alerts = append(alerts, &a)
	}
	return alerts, rows.Err()
}
