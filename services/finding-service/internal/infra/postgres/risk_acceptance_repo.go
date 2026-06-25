// Package postgres — risk_acceptance_repo.go
// PostgresRiskAcceptanceRepo implements riskacceptance.Repository.
// Stores risk acceptances in the `risk_acceptances` table.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/osv/finding-service/internal/domain/riskacceptance"
)

// PostgresRiskAcceptanceRepo persists RiskAcceptance entities.
type PostgresRiskAcceptanceRepo struct {
	db *pgxpool.Pool
}

// NewRiskAcceptanceRepo creates a new PostgresRiskAcceptanceRepo.
func NewRiskAcceptanceRepo(db *pgxpool.Pool) *PostgresRiskAcceptanceRepo {
	return &PostgresRiskAcceptanceRepo{db: db}
}

// Compile-time check.
var _ riskacceptance.Repository = (*PostgresRiskAcceptanceRepo)(nil)

// Save inserts or updates a risk acceptance.
func (r *PostgresRiskAcceptanceRepo) Save(ctx context.Context, ra *riskacceptance.RiskAcceptance) error {
	if ra.ID == uuid.Nil {
		ra.ID = uuid.New()
	}
	now := time.Now().UTC()
	ra.UpdatedAt = now

	_, err := r.db.Exec(ctx, `
		INSERT INTO risk_acceptances (
			id, name, product_id, accepted_by_id, expiration_date, notes,
			proof_file_key, reactivate_expired, reactivate_note_text,
			restart_sla_on_reactivation, is_expired, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			expiration_date = EXCLUDED.expiration_date,
			notes = EXCLUDED.notes,
			proof_file_key = EXCLUDED.proof_file_key,
			reactivate_expired = EXCLUDED.reactivate_expired,
			reactivate_note_text = EXCLUDED.reactivate_note_text,
			restart_sla_on_reactivation = EXCLUDED.restart_sla_on_reactivation,
			is_expired = EXCLUDED.is_expired,
			updated_at = EXCLUDED.updated_at`,
		ra.ID, ra.Name, ra.ProductID, ra.AcceptedByID, ra.ExpirationDate,
		ra.Notes, ra.ProofFileKey, ra.ReactivateExpired, ra.ReactivateNoteText,
		ra.RestartSLAOnReactivation, ra.IsExpired, ra.CreatedAt, ra.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("risk_acceptance_repo: save: %w", err)
	}

	// Sync linked findings
	if len(ra.FindingIDs) > 0 {
		for _, fid := range ra.FindingIDs {
			_, _ = r.db.Exec(ctx, `
				INSERT INTO risk_acceptance_findings (risk_acceptance_id, finding_id)
				VALUES ($1, $2) ON CONFLICT DO NOTHING`, ra.ID, fid)
		}
	}
	return nil
}

// FindByID returns a risk acceptance by ID, with its finding IDs loaded.
func (r *PostgresRiskAcceptanceRepo) FindByID(ctx context.Context, id uuid.UUID) (*riskacceptance.RiskAcceptance, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, name, product_id, accepted_by_id, expiration_date, notes,
		       proof_file_key, reactivate_expired, reactivate_note_text,
		       restart_sla_on_reactivation, is_expired, created_at, updated_at
		FROM risk_acceptances WHERE id = $1`, id)

	ra, err := scanRA(row)
	if err != nil {
		return nil, err
	}
	ra.FindingIDs, _ = r.loadFindingIDs(ctx, id)
	return ra, nil
}

// ListByProduct returns all risk acceptances for a given product.
func (r *PostgresRiskAcceptanceRepo) ListByProduct(ctx context.Context, productID uuid.UUID) ([]*riskacceptance.RiskAcceptance, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, product_id, accepted_by_id, expiration_date, notes,
		       proof_file_key, reactivate_expired, reactivate_note_text,
		       restart_sla_on_reactivation, is_expired, created_at, updated_at
		FROM risk_acceptances WHERE product_id = $1 ORDER BY created_at DESC`, productID)
	if err != nil {
		return nil, fmt.Errorf("risk_acceptance_repo: list_by_product: %w", err)
	}
	defer rows.Close()

	var result []*riskacceptance.RiskAcceptance
	for rows.Next() {
		ra, err := scanRA(rows)
		if err != nil {
			continue
		}
		ra.FindingIDs, _ = r.loadFindingIDs(ctx, ra.ID)
		result = append(result, ra)
	}
	return result, nil
}

// ListExpiring returns risk acceptances expiring on or before `before`.
func (r *PostgresRiskAcceptanceRepo) ListExpiring(ctx context.Context, before time.Time) ([]*riskacceptance.RiskAcceptance, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, product_id, accepted_by_id, expiration_date, notes,
		       proof_file_key, reactivate_expired, reactivate_note_text,
		       restart_sla_on_reactivation, is_expired, created_at, updated_at
		FROM risk_acceptances
		WHERE expiration_date <= $1 AND is_expired = FALSE`, before)
	if err != nil {
		return nil, fmt.Errorf("risk_acceptance_repo: list_expiring: %w", err)
	}
	defer rows.Close()

	var result []*riskacceptance.RiskAcceptance
	for rows.Next() {
		ra, err := scanRA(rows)
		if err != nil {
			continue
		}
		result = append(result, ra)
	}
	return result, nil
}

// Delete removes a risk acceptance by ID.
func (r *PostgresRiskAcceptanceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM risk_acceptances WHERE id = $1`, id)
	return err
}

// MarkExpired marks a risk acceptance as expired.
func (r *PostgresRiskAcceptanceRepo) MarkExpired(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE risk_acceptances SET is_expired = TRUE, updated_at = NOW() WHERE id = $1`, id)
	return err
}

// AddFinding links a finding to a risk acceptance.
func (r *PostgresRiskAcceptanceRepo) AddFinding(ctx context.Context, raID, findingID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO risk_acceptance_findings (risk_acceptance_id, finding_id)
		VALUES ($1, $2) ON CONFLICT DO NOTHING`, raID, findingID)
	return err
}

// RemoveFinding unlinks a finding from a risk acceptance.
func (r *PostgresRiskAcceptanceRepo) RemoveFinding(ctx context.Context, raID, findingID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM risk_acceptance_findings
		WHERE risk_acceptance_id = $1 AND finding_id = $2`, raID, findingID)
	return err
}

// ── helpers ──────────────────────────────────────────────────────────────────

type scannable interface {
	Scan(dest ...any) error
}

func scanRA(row scannable) (*riskacceptance.RiskAcceptance, error) {
	ra := &riskacceptance.RiskAcceptance{}
	err := row.Scan(
		&ra.ID, &ra.Name, &ra.ProductID, &ra.AcceptedByID, &ra.ExpirationDate,
		&ra.Notes, &ra.ProofFileKey, &ra.ReactivateExpired, &ra.ReactivateNoteText,
		&ra.RestartSLAOnReactivation, &ra.IsExpired, &ra.CreatedAt, &ra.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("risk acceptance not found")
		}
		return nil, fmt.Errorf("risk_acceptance_repo: scan: %w", err)
	}
	return ra, nil
}

func (r *PostgresRiskAcceptanceRepo) loadFindingIDs(ctx context.Context, raID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx,
		`SELECT finding_id FROM risk_acceptance_findings WHERE risk_acceptance_id = $1`, raID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
