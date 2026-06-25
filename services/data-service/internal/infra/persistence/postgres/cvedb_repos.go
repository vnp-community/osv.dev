// Package postgres — cvedb_repos.go
// [FIX TASK-HC-015] PostgreSQL implementations of the CVEDB repository interfaces
// used by lookupcves, populatedb, initdb, importdb, exportdb, backupdb use cases.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/data-service/internal/domain/entity"
	"github.com/osv/data-service/internal/domain/repository"
)

// ──────────────────────────────────────────────────────────────────────────────
// CVEBinToolRepo — implements repository.CVEBinToolRepository
// ──────────────────────────────────────────────────────────────────────────────

// CVEBinToolRepo implements repository.CVEBinToolRepository using PostgreSQL.
type CVEBinToolRepo struct{ pool *pgxpool.Pool }

// NewCVEBinToolRepo creates a CVEBinToolRepo.
func NewCVEBinToolRepo(pool *pgxpool.Pool) *CVEBinToolRepo {
	return &CVEBinToolRepo{pool: pool}
}

func (r *CVEBinToolRepo) FindSeverities(ctx context.Context, cveNumbers []string) ([]entity.CVESeverity, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cve_number, severity, score, cvss_vector, cvss_version
		FROM cve_severities WHERE cve_number = ANY($1)
	`, cveNumbers)
	if err != nil {
		return nil, fmt.Errorf("CVEBinToolRepo.FindSeverities: %w", err)
	}
	defer rows.Close()
	var results []entity.CVESeverity
	for rows.Next() {
		var s entity.CVESeverity
		var cvssVer int
		if err := rows.Scan(&s.CVENumber, &s.Severity, &s.Score, &s.CVSSVector, &cvssVer); err != nil {
			continue
		}
		s.CVSSVersion = cvssVer
		results = append(results, s)
	}
	return results, rows.Err()
}

func (r *CVEBinToolRepo) FindExactVersion(ctx context.Context, vendor, product, version string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT cve_number FROM cve_ranges
		WHERE vendor = $1 AND product = $2 AND version = $3
	`, vendor, product, version)
	if err != nil {
		return nil, fmt.Errorf("CVEBinToolRepo.FindExactVersion: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

func (r *CVEBinToolRepo) FindRanges(ctx context.Context, vendor, product string) ([]entity.CVERange, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cve_number, vendor, product, version,
		       version_start_including, version_start_excluding,
		       version_end_including, version_end_excluding, data_source
		FROM cve_ranges WHERE vendor = $1 AND product = $2
	`, vendor, product)
	if err != nil {
		return nil, fmt.Errorf("CVEBinToolRepo.FindRanges: %w", err)
	}
	defer rows.Close()
	return scanRanges(rows)
}

func (r *CVEBinToolRepo) FindAllRanges(ctx context.Context) ([]entity.CVERange, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cve_number, vendor, product, version,
		       version_start_including, version_start_excluding,
		       version_end_including, version_end_excluding, data_source
		FROM cve_ranges ORDER BY vendor, product
	`)
	if err != nil {
		return nil, fmt.Errorf("CVEBinToolRepo.FindAllRanges: %w", err)
	}
	defer rows.Close()
	return scanRanges(rows)
}

func (r *CVEBinToolRepo) UpsertSeverity(ctx context.Context, s entity.CVESeverity) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO cve_severities
		  (cve_number, severity, score, cvss_vector, cvss_version, data_source)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (cve_number) DO UPDATE
		  SET severity=$2, score=$3, cvss_vector=$4, cvss_version=$5, data_source=$6
	`, s.CVENumber, s.Severity, s.Score, s.CVSSVector, s.CVSSVersion, s.DataSource)
	return err
}

func (r *CVEBinToolRepo) UpsertSeverities(ctx context.Context, severities []entity.CVESeverity) error {
	for _, s := range severities {
		if err := r.UpsertSeverity(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func (r *CVEBinToolRepo) UpsertRange(ctx context.Context, rng entity.CVERange) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO cve_ranges
		  (cve_number, vendor, product, version,
		   version_start_including, version_start_excluding,
		   version_end_including, version_end_excluding, data_source)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (cve_number, vendor, product, version_start_including, version_end_excluding)
		DO UPDATE
		  SET version_start_excluding=$6, version_end_including=$7, data_source=$9
	`, rng.CVENumber, rng.Vendor, rng.Product, rng.Version,
		rng.VersionStartIncluding, rng.VersionStartExcluding,
		rng.VersionEndIncluding, rng.VersionEndExcluding, rng.DataSource)
	return err
}

func (r *CVEBinToolRepo) UpsertRanges(ctx context.Context, ranges []entity.CVERange) error {
	for _, rng := range ranges {
		if err := r.UpsertRange(ctx, rng); err != nil {
			return err
		}
	}
	return nil
}

type rangeScanner interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func scanRanges(rows rangeScanner) ([]entity.CVERange, error) {
	var result []entity.CVERange
	for rows.Next() {
		var rng entity.CVERange
		if err := rows.Scan(
			&rng.CVENumber, &rng.Vendor, &rng.Product, &rng.Version,
			&rng.VersionStartIncluding, &rng.VersionStartExcluding,
			&rng.VersionEndIncluding, &rng.VersionEndExcluding, &rng.DataSource,
		); err != nil {
			continue
		}
		result = append(result, rng)
	}
	return result, rows.Err()
}

// ──────────────────────────────────────────────────────────────────────────────
// ExploitRepo — implements repository.ExploitRepository
// ──────────────────────────────────────────────────────────────────────────────

// ExploitRepo implements repository.ExploitRepository using kev_vulnerabilities.
type ExploitRepo struct{ pool *pgxpool.Pool }

// NewExploitRepo creates an ExploitRepo.
func NewExploitRepo(pool *pgxpool.Pool) *ExploitRepo {
	return &ExploitRepo{pool: pool}
}

func (r *ExploitRepo) FindExploited(ctx context.Context, cveNumbers []string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cve_id FROM kev_vulnerabilities WHERE cve_id = ANY($1)
	`, cveNumbers)
	if err != nil {
		return nil, fmt.Errorf("ExploitRepo.FindExploited: %w", err)
	}
	defer rows.Close()
	var results []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			results = append(results, id)
		}
	}
	return results, rows.Err()
}

func (r *ExploitRepo) UpsertExploited(ctx context.Context, cveNumbers []string) error {
	now := time.Now().UTC()
	for _, id := range cveNumbers {
		_, err := r.pool.Exec(ctx, `
			INSERT INTO kev_vulnerabilities (cve_id, date_added, date_published)
			VALUES ($1, $2, $2)
			ON CONFLICT (cve_id) DO NOTHING
		`, id, now)
		if err != nil {
			return fmt.Errorf("ExploitRepo.UpsertExploited: %w", err)
		}
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// MetricRepo — implements repository.MetricRepository
// ──────────────────────────────────────────────────────────────────────────────

// MetricRepo implements repository.MetricRepository using epss_scores.
type MetricRepo struct{ pool *pgxpool.Pool }

// NewMetricRepo creates a MetricRepo.
func NewMetricRepo(pool *pgxpool.Pool) *MetricRepo {
	return &MetricRepo{pool: pool}
}

func (r *MetricRepo) GetEPSS(ctx context.Context, cveNumbers []string) (map[string]entity.EPSSData, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cve_id, epss_score, epss_percentile
		FROM epss_scores WHERE cve_id = ANY($1)
	`, cveNumbers)
	if err != nil {
		return nil, fmt.Errorf("MetricRepo.GetEPSS: %w", err)
	}
	defer rows.Close()
	result := make(map[string]entity.EPSSData)
	for rows.Next() {
		var cveID string
		var d entity.EPSSData
		if err := rows.Scan(&cveID, &d.Probability, &d.Percentile); err != nil {
			continue
		}
		result[cveID] = d
	}
	return result, rows.Err()
}

func (r *MetricRepo) UpsertMetrics(ctx context.Context, metrics []entity.CVEMetric) error {
	for _, m := range metrics {
		_, err := r.pool.Exec(ctx, `
			INSERT INTO epss_scores (cve_id, epss_score, epss_percentile)
			VALUES ($1, $2, $3)
			ON CONFLICT (cve_id) DO UPDATE
			  SET epss_score = $2, epss_percentile = $3
		`, m.CVENumber, m.MetricScore, m.MetricField)
		if err != nil {
			return fmt.Errorf("MetricRepo.UpsertMetrics: %w", err)
		}
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// PURL2CPERepo — implements repository.PURL2CPERepository
// ──────────────────────────────────────────────────────────────────────────────

// PURL2CPERepo implements repository.PURL2CPERepository.
type PURL2CPERepo struct{ pool *pgxpool.Pool }

// NewPURL2CPERepo creates a PURL2CPERepo.
func NewPURL2CPERepo(pool *pgxpool.Pool) *PURL2CPERepo {
	return &PURL2CPERepo{pool: pool}
}

func (r *PURL2CPERepo) LookupCPE(ctx context.Context, purl string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT cpe FROM purl2cpe_mappings WHERE purl = $1`, purl)
	if err != nil {
		return nil, fmt.Errorf("PURL2CPERepo.LookupCPE: %w", err)
	}
	defer rows.Close()
	var cpes []string
	for rows.Next() {
		var cpe string
		if err := rows.Scan(&cpe); err == nil {
			cpes = append(cpes, cpe)
		}
	}
	return cpes, rows.Err()
}

func (r *PURL2CPERepo) UpsertMappings(ctx context.Context, mappings []entity.PURL2CPE) error {
	for _, m := range mappings {
		_, err := r.pool.Exec(ctx, `
			INSERT INTO purl2cpe_mappings (purl, cpe) VALUES ($1, $2)
			ON CONFLICT (purl, cpe) DO NOTHING
		`, m.PURL, m.CPE)
		if err != nil {
			return fmt.Errorf("PURL2CPERepo.UpsertMappings: %w", err)
		}
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// DBAdminRepo — implements repository.DBAdminRepository
// ──────────────────────────────────────────────────────────────────────────────

// DBAdminRepo implements repository.DBAdminRepository using PostgreSQL.
type DBAdminRepo struct{ pool *pgxpool.Pool }

// NewDBAdminRepo creates a DBAdminRepo.
func NewDBAdminRepo(pool *pgxpool.Pool) *DBAdminRepo {
	return &DBAdminRepo{pool: pool}
}

func (r *DBAdminRepo) InitSchema(ctx context.Context, forceRebuild bool) error {
	if forceRebuild {
		_, err := r.pool.Exec(ctx, `
			DROP TABLE IF EXISTS cve_ranges, cve_severities, purl2cpe_mappings CASCADE;
		`)
		if err != nil {
			return fmt.Errorf("DBAdminRepo.InitSchema(force): %w", err)
		}
	}
	_, err := r.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS cve_severities (
			cve_number   VARCHAR(32) PRIMARY KEY,
			severity     VARCHAR(16),
			score        FLOAT8,
			cvss_vector  TEXT,
			cvss_version INT,
			data_source  VARCHAR(32),
			description  TEXT
		);
		CREATE TABLE IF NOT EXISTS cve_ranges (
			id                       SERIAL PRIMARY KEY,
			cve_number               VARCHAR(32) NOT NULL,
			vendor                   TEXT NOT NULL,
			product                  TEXT NOT NULL,
			version                  TEXT,
			version_start_including  TEXT,
			version_start_excluding  TEXT,
			version_end_including    TEXT,
			version_end_excluding    TEXT,
			data_source              VARCHAR(32),
			UNIQUE(cve_number, vendor, product, version_start_including, version_end_excluding)
		);
		CREATE TABLE IF NOT EXISTS purl2cpe_mappings (
			purl TEXT NOT NULL,
			cpe  TEXT NOT NULL,
			PRIMARY KEY (purl, cpe)
		);
		CREATE INDEX IF NOT EXISTS idx_cve_ranges_vendor_product ON cve_ranges(vendor, product);
		CREATE INDEX IF NOT EXISTS idx_cve_ranges_cve_number ON cve_ranges(cve_number);
	`)
	return err
}

func (r *DBAdminRepo) GetStatus(ctx context.Context) (entity.DBState, error) {
	var state entity.DBState
	state.SchemaVersion = "v1"
	var cveCount, rangeCount int64
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM cve_severities`).Scan(&cveCount)
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM cve_ranges`).Scan(&rangeCount)
	state.CVECount = cveCount
	state.RangeCount = rangeCount
	state.LastUpdated = time.Now().UTC().Format(time.RFC3339)
	return state, nil
}

func (r *DBAdminRepo) Backup(ctx context.Context, destPath string) error {
	cfg := r.pool.Config()
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
		cfg.ConnConfig.Host, cfg.ConnConfig.Port,
		cfg.ConnConfig.User, cfg.ConnConfig.Password,
		cfg.ConnConfig.Database)
	cmd := exec.CommandContext(ctx, "pg_dump", dsn, "-f", destPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *DBAdminRepo) Restore(ctx context.Context, srcPath string) error {
	cfg := r.pool.Config()
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
		cfg.ConnConfig.Host, cfg.ConnConfig.Port,
		cfg.ConnConfig.User, cfg.ConnConfig.Password,
		cfg.ConnConfig.Database)
	cmd := exec.CommandContext(ctx, "psql", dsn, "-f", srcPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *DBAdminRepo) ExportJSON(ctx context.Context, year int) ([]byte, error) {
	query := `SELECT cve_number, severity, score FROM cve_severities`
	if year > 0 {
		query += fmt.Sprintf(` WHERE cve_number LIKE 'CVE-%d-%%'`, year)
	}
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("DBAdminRepo.ExportJSON: %w", err)
	}
	defer rows.Close()
	type Row struct {
		CVENumber string  `json:"cve_number"`
		Severity  string  `json:"severity"`
		Score     float64 `json:"score"`
	}
	var result []Row
	for rows.Next() {
		var row Row
		if err := rows.Scan(&row.CVENumber, &row.Severity, &row.Score); err == nil {
			result = append(result, row)
		}
	}
	return json.Marshal(result)
}

func (r *DBAdminRepo) ImportJSON(ctx context.Context, data []byte) error {
	type Row struct {
		CVENumber string  `json:"cve_number"`
		Severity  string  `json:"severity"`
		Score     float64 `json:"score"`
	}
	var rows []Row
	if err := json.Unmarshal(data, &rows); err != nil {
		return fmt.Errorf("DBAdminRepo.ImportJSON: %w", err)
	}
	for _, row := range rows {
		_, err := r.pool.Exec(ctx, `
			INSERT INTO cve_severities (cve_number, severity, score)
			VALUES ($1, $2, $3)
			ON CONFLICT (cve_number) DO UPDATE SET severity=$2, score=$3
		`, row.CVENumber, row.Severity, row.Score)
		if err != nil {
			return err
		}
	}
	return nil
}

// Compile-time interface satisfaction checks.
var (
	_ repository.CVEBinToolRepository = (*CVEBinToolRepo)(nil)
	_ repository.ExploitRepository    = (*ExploitRepo)(nil)
	_ repository.MetricRepository     = (*MetricRepo)(nil)
	_ repository.PURL2CPERepository   = (*PURL2CPERepo)(nil)
	_ repository.DBAdminRepository    = (*DBAdminRepo)(nil)
)
