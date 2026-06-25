// Package postgres provides PostgreSQL implementations for asset-service repositories.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/google/osv.dev/services/asset-service/internal/domain/entity"
)

type assetRepo struct {
	db *sql.DB
}

// NewAssetRepository creates a PostgreSQL-backed asset repository.
func NewAssetRepository(db *sql.DB) *assetRepo {
	return &assetRepo{db: db}
}

func marshalJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func (r *assetRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.Asset, error) {
	var a entity.Asset
	var services, labels []byte
	var tags pq.StringArray

	err := r.db.QueryRowContext(ctx, `
		SELECT id, ip_address, hostname, os, mac_address, services, tags, labels, risk_score, finding_count, status, last_seen_at, created_at, updated_at
		FROM osv_asset.assets
		WHERE id = $1
	`, id).Scan(
		&a.ID, &a.IPAddress, &a.Hostname, &a.OS, &a.MACAddress,
		&services, &tags, &labels, &a.RiskScore, &a.FindingCount,
		&a.Status, &a.LastSeenAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(services, &a.Services)
	_ = json.Unmarshal(labels, &a.Labels)
	a.Tags = tags
	return &a, nil
}

func (r *assetRepo) FindAll(ctx context.Context, filter entity.AssetFilter) ([]*entity.Asset, int, error) {
	args := []interface{}{}
	where := []string{}
	argIdx := 1

	if filter.Tag != "" {
		where = append(where, fmt.Sprintf("$%d = ANY(tags)", argIdx))
		args = append(args, filter.Tag)
		argIdx++
	}
	if filter.OS != "" {
		where = append(where, fmt.Sprintf("os ILIKE $%d", argIdx))
		args = append(args, "%"+filter.OS+"%")
		argIdx++
	}
	if filter.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(filter.Status))
		argIdx++
	}
	if filter.Query != "" {
		where = append(where, fmt.Sprintf(
			"(ip_address::text ILIKE $%d OR hostname ILIKE $%d OR os ILIKE $%d)",
			argIdx, argIdx, argIdx,
		))
		args = append(args, "%"+filter.Query+"%")
		argIdx++
	}
	if filter.HasPort != nil {
		where = append(where, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM jsonb_array_elements(services) s WHERE (s->>'port')::int = $%d)",
			argIdx,
		))
		args = append(args, *filter.HasPort)
		argIdx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Count total
	var total int
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM osv_asset.assets %s", whereClause)
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count assets: %w", err)
	}

	// Paginate
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	querySQL := fmt.Sprintf(`
		SELECT id, ip_address, hostname, os, mac_address, services, tags, labels,
		       risk_score, finding_count, status, last_seen_at, created_at, updated_at
		FROM osv_asset.assets
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query assets: %w", err)
	}
	defer rows.Close()

	assets := make([]*entity.Asset, 0)
	for rows.Next() {
		var a entity.Asset
		var services, labels []byte
		var tags pq.StringArray
		if err := rows.Scan(
			&a.ID, &a.IPAddress, &a.Hostname, &a.OS, &a.MACAddress,
			&services, &tags, &labels, &a.RiskScore, &a.FindingCount,
			&a.Status, &a.LastSeenAt, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan asset: %w", err)
		}
		_ = json.Unmarshal(services, &a.Services)
		_ = json.Unmarshal(labels, &a.Labels)
		a.Tags = tags
		assets = append(assets, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}
	return assets, total, nil
}


func (r *assetRepo) Update(ctx context.Context, asset *entity.Asset) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE osv_asset.assets
		SET hostname=$1, os=$2, mac_address=$3, services=$4, tags=$5, labels=$6, risk_score=$7, finding_count=$8, status=$9, updated_at=NOW()
		WHERE id=$10
	`, asset.Hostname, asset.OS, asset.MACAddress, marshalJSON(asset.Services), pq.Array(asset.Tags), marshalJSON(asset.Labels),
		asset.RiskScore, asset.FindingCount, asset.Status, asset.ID)
	return err
}

func (r *assetRepo) Create(ctx context.Context, asset *entity.Asset) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO osv_asset.assets (id, ip_address, hostname, os, mac_address, services, tags, labels, status, created_at, updated_at)
		VALUES ($1, $2::INET, $3, $4, $5, $6, $7, $8, 'active', NOW(), NOW())
	`, asset.ID, asset.IPAddress, asset.Hostname, asset.OS, asset.MACAddress, marshalJSON(asset.Services), pq.Array(asset.Tags), marshalJSON(asset.Labels))
	return err
}

func (r *assetRepo) CreateBulk(ctx context.Context, assets []*entity.Asset, updateExisting bool) ([]entity.BulkAssetResult, error) {
	conflictSQL := "ON CONFLICT (ip_address) DO NOTHING"
	if updateExisting {
		conflictSQL = `ON CONFLICT (ip_address) DO UPDATE
			SET hostname=EXCLUDED.hostname, os=EXCLUDED.os, mac_address=EXCLUDED.mac_address, services=EXCLUDED.services,
				tags=EXCLUDED.tags, labels=EXCLUDED.labels, updated_at=NOW()`
	}

	results := make([]entity.BulkAssetResult, 0, len(assets))
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for _, a := range assets {
		var id uuid.UUID
		err := tx.QueryRowContext(ctx, fmt.Sprintf(`
			INSERT INTO osv_asset.assets (id, ip_address, hostname, os, mac_address, services, tags, labels, status, created_at, updated_at)
			VALUES ($1, $2::INET, $3, $4, $5, $6, $7, $8, 'active', NOW(), NOW())
			%s RETURNING id
		`, conflictSQL),
			a.ID, a.IPAddress, a.Hostname, a.OS, a.MACAddress,
			marshalJSON(a.Services), pq.Array(a.Tags), marshalJSON(a.Labels)).Scan(&id)

		if err == sql.ErrNoRows {
			// DO NOTHING hit conflict — IP exists, not updated
			results = append(results, entity.BulkAssetResult{IPAddress: a.IPAddress, Status: "skipped"})
			continue
		}
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				// Handle ON CONFLICT DO NOTHING without RETURNING edge cases
				results = append(results, entity.BulkAssetResult{IPAddress: a.IPAddress, Status: "skipped"})
			} else {
				results = append(results, entity.BulkAssetResult{IPAddress: a.IPAddress, Status: "error", Message: err.Error()})
			}
			continue
		}

		status := "created"
		if updateExisting {
			// Without checking row affected via RETURNING properly (e.g. xmin tricks), we just mark it as created/updated
			status = "updated" 
		}
		results = append(results, entity.BulkAssetResult{IPAddress: a.IPAddress, Status: status, ID: &id})
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *assetRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM osv_asset.assets WHERE id = $1", id)
	return err
}

func (r *assetRepo) AddVulnerabilities(ctx context.Context, assetID uuid.UUID, vulns []entity.Vulnerability) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, v := range vulns {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO osv_asset.asset_vulnerabilities (asset_id, cve_id, severity, cvss, detected_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING
		`, assetID, v.CveID, v.Severity, v.Cvss, v.DetectedAt)
		if err != nil {
			return err
		}
	}
	// Update finding_count
	_, err = tx.ExecContext(ctx,
		`UPDATE osv_asset.assets SET finding_count = (
			SELECT COUNT(*) FROM osv_asset.asset_vulnerabilities WHERE asset_id=$1
		 ), updated_at=NOW() WHERE id=$1`, assetID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *assetRepo) ListVulnerabilities(ctx context.Context, assetID uuid.UUID) ([]entity.Vulnerability, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, asset_id, cve_id, severity, cvss, detected_at
		FROM asset_vulnerabilities
		WHERE asset_id = $1
		ORDER BY detected_at DESC`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	vulns := make([]entity.Vulnerability, 0) // FIX: never nil → JSON [] not null
	for rows.Next() {
		var v entity.Vulnerability
		if err := rows.Scan(&v.ID, &v.AssetID, &v.CveID, &v.Severity, &v.Cvss, &v.DetectedAt); err != nil {
			return nil, err
		}
		vulns = append(vulns, v)
	}
	return vulns, rows.Err()
}


