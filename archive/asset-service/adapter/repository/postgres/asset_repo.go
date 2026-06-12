package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/asset-service/internal/domain/entity"
	"github.com/osv/asset-service/internal/domain/repository"
)

// AssetRepo implements repository.AssetRepository.
type AssetRepo struct{ db *pgxpool.Pool }

func NewAssetRepo(db *pgxpool.Pool) *AssetRepo { return &AssetRepo{db: db} }

func (r *AssetRepo) Create(ctx context.Context, a *entity.Asset) error {
	if a.ID == uuid.Nil { a.ID = uuid.New() }
	a.CreatedAt = time.Now().UTC(); a.UpdatedAt = a.CreatedAt
	svc, _ := json.Marshal(a.Services)
	wt, _ := json.Marshal(a.WebTech)
	labels, _ := json.Marshal(a.Labels)
	_, err := r.db.Exec(ctx, `
		INSERT INTO asset.assets (id, ip_address, hostname, os, mac_address, services, web_tech, labels, last_scanned_at, created_at, updated_at)
		VALUES ($1,$2::inet,$3,$4,$5,$6::jsonb,$7::jsonb,$8::jsonb,$9,$10,$10)`,
		a.ID, a.IPAddress, a.Hostname, a.OS, a.MACAddress, svc, wt, labels, a.LastScannedAt, a.CreatedAt,
	)
	return err
}

func (r *AssetRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.Asset, error) {
	return r.scanOne(ctx, `SELECT id,ip_address,hostname,os,mac_address,services,web_tech,labels,last_scanned_at,created_at,updated_at FROM asset.assets WHERE id=$1`, id)
}

func (r *AssetRepo) FindByIPAddress(ctx context.Context, ip string) (*entity.Asset, error) {
	return r.scanOne(ctx, `SELECT id,ip_address,hostname,os,mac_address,services,web_tech,labels,last_scanned_at,created_at,updated_at FROM asset.assets WHERE ip_address=$1::inet`, ip)
}

func (r *AssetRepo) scanOne(ctx context.Context, query string, arg any) (*entity.Asset, error) {
	var a entity.Asset
	var svc, wt, labels []byte
	err := r.db.QueryRow(ctx, query, arg).Scan(
		&a.ID, &a.IPAddress, &a.Hostname, &a.OS, &a.MACAddress,
		&svc, &wt, &labels, &a.LastScannedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil { return nil, err }
	json.Unmarshal(svc, &a.Services)
	json.Unmarshal(wt, &a.WebTech)
	json.Unmarshal(labels, &a.Labels)
	return &a, nil
}

func (r *AssetRepo) Update(ctx context.Context, a *entity.Asset) error {
	a.UpdatedAt = time.Now().UTC()
	svc, _ := json.Marshal(a.Services)
	wt, _ := json.Marshal(a.WebTech)
	_, err := r.db.Exec(ctx, `
		UPDATE asset.assets SET hostname=$2,os=$3,services=$4::jsonb,web_tech=$5::jsonb,last_scanned_at=$6,updated_at=$7 WHERE id=$1`,
		a.ID, a.Hostname, a.OS, svc, wt, a.LastScannedAt, a.UpdatedAt,
	)
	return err
}

func (r *AssetRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM asset.assets WHERE id=$1`, id)
	return err
}

func (r *AssetRepo) List(ctx context.Context, f repository.AssetFilter) ([]*entity.Asset, int64, error) {
	q := `SELECT id,ip_address,hostname,os,created_at,updated_at FROM asset.assets WHERE 1=1`
	args := []any{}; n := 1
	if f.Search != "" { q += fmt.Sprintf(" AND (hostname ILIKE $%d OR ip_address::text ILIKE $%d)", n, n); args = append(args, "%"+f.Search+"%"); n++ }
	if f.OS != "" { q += fmt.Sprintf(" AND os ILIKE $%d", n); args = append(args, "%"+f.OS+"%"); n++ }
	page := f.Page; if page < 1 { page = 1 }
	ps := f.PageSize; if ps < 1 { ps = 20 }
	q += fmt.Sprintf(" ORDER BY updated_at DESC LIMIT $%d OFFSET $%d", n, n+1)
	args = append(args, ps, (page-1)*ps)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, 0, err }
	defer rows.Close()
	var assets []*entity.Asset
	for rows.Next() {
		var a entity.Asset
		rows.Scan(&a.ID, &a.IPAddress, &a.Hostname, &a.OS, &a.CreatedAt, &a.UpdatedAt)
		assets = append(assets, &a)
	}
	return assets, int64(len(assets)), rows.Err()
}

func (r *AssetRepo) UpdateLastScanned(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE asset.assets SET last_scanned_at=NOW(),updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *AssetRepo) AttachTag(ctx context.Context, assetID, tagID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `INSERT INTO asset.asset_tags(asset_id,tag_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, assetID, tagID)
	return err
}

func (r *AssetRepo) DetachTag(ctx context.Context, assetID, tagID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM asset.asset_tags WHERE asset_id=$1 AND tag_id=$2`, assetID, tagID)
	return err
}

// VulnerabilityRepo implements repository.VulnerabilityRepository.
type VulnerabilityRepo struct{ db *pgxpool.Pool }

func NewVulnerabilityRepo(db *pgxpool.Pool) *VulnerabilityRepo { return &VulnerabilityRepo{db: db} }

func (r *VulnerabilityRepo) Upsert(ctx context.Context, v *entity.Vulnerability) error {
	if v.ID == uuid.Nil { v.ID = uuid.New() }
	_, err := r.db.Exec(ctx, `
		INSERT INTO asset.vulnerabilities (id,asset_id,cve_id,summary,severity,cvss,scan_id,detected_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())
		ON CONFLICT (asset_id,cve_id) DO UPDATE SET
		  summary=EXCLUDED.summary, severity=EXCLUDED.severity, cvss=EXCLUDED.cvss, detected_at=NOW()`,
		v.ID, v.AssetID, v.CVEID, v.Summary, v.Severity, v.CVSS, v.ScanID,
	)
	return err
}

func (r *VulnerabilityRepo) FindByAssetID(ctx context.Context, assetID uuid.UUID, page, pageSize int) ([]*entity.Vulnerability, int64, error) {
	if page < 1 { page = 1 }; if pageSize < 1 { pageSize = 50 }
	rows, err := r.db.Query(ctx, `
		SELECT id,asset_id,cve_id,summary,severity,cvss,detected_at,remediated_at
		FROM asset.vulnerabilities WHERE asset_id=$1 ORDER BY detected_at DESC LIMIT $2 OFFSET $3`,
		assetID, pageSize, (page-1)*pageSize,
	)
	if err != nil { return nil, 0, err }
	defer rows.Close()
	var vs []*entity.Vulnerability
	for rows.Next() {
		var v entity.Vulnerability
		rows.Scan(&v.ID, &v.AssetID, &v.CVEID, &v.Summary, &v.Severity, &v.CVSS, &v.DetectedAt, &v.RemediatedAt)
		vs = append(vs, &v)
	}
	return vs, int64(len(vs)), rows.Err()
}

func (r *VulnerabilityRepo) MarkRemediated(ctx context.Context, assetID uuid.UUID, cveID string) error {
	_, err := r.db.Exec(ctx, `UPDATE asset.vulnerabilities SET remediated_at=NOW() WHERE asset_id=$1 AND cve_id=$2`, assetID, cveID)
	return err
}

func (r *VulnerabilityRepo) GetSummaryByAsset(ctx context.Context, assetID uuid.UUID) (*entity.VulnSummary, error) {
	rows, err := r.db.Query(ctx, `
		SELECT severity, COUNT(*) FROM asset.vulnerabilities
		WHERE asset_id=$1 AND remediated_at IS NULL GROUP BY severity`, assetID)
	if err != nil { return nil, err }
	defer rows.Close()
	summary := &entity.VulnSummary{}
	for rows.Next() {
		var sev string; var cnt int
		rows.Scan(&sev, &cnt)
		switch entity.Severity(sev) {
		case entity.SeverityCritical: summary.Critical = cnt
		case entity.SeverityHigh:     summary.High = cnt
		case entity.SeverityMedium:   summary.Medium = cnt
		case entity.SeverityLow:      summary.Low = cnt
		}
	}
	return summary, rows.Err()
}
