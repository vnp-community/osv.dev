package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/osv/search-service/internal/domain/entity"
	"github.com/osv/search-service/internal/domain/repository"
)

// pgCWERepository implements CWERepository using PostgreSQL.
type pgCWERepository struct{ db *sqlx.DB }

func NewCWERepository(db *sqlx.DB) repository.CWERepository {
	return &pgCWERepository{db: db}
}

func (r *pgCWERepository) List(ctx context.Context, q string, page, limit int) ([]*entity.CWEEntry, int64, error) {
	offset := page * limit
	args := []interface{}{limit, offset}
	where := ""
	if q != "" {
		where = "WHERE lower(name) LIKE lower($3) OR lower(id) LIKE lower($3)"
		args = append(args, "%"+q+"%")
	}

	var entries []*entity.CWEEntry
	query := fmt.Sprintf(`SELECT id, name, description, abstraction, status, updated_at
        FROM cwe_weaknesses %s ORDER BY id LIMIT $1 OFFSET $2`, where)
	err := r.db.SelectContext(ctx, &entries, query, args...)

	var total int64
	r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cwe_weaknesses "+where, args[2:]...).Scan(&total)

	return entries, total, err
}

func (r *pgCWERepository) FindByID(ctx context.Context, id string) (*entity.CWEEntry, error) {
	var entry entity.CWEEntry
	err := r.db.GetContext(ctx, &entry, `SELECT id, name, description, abstraction, status, updated_at
        FROM cwe_weaknesses WHERE id = $1`, id)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	return &entry, err
}

// pgCAPECRepository implements CAPECRepository.
type pgCAPECRepository struct{ db *sqlx.DB }

func NewCAPECRepository(db *sqlx.DB) repository.CAPECRepository {
	return &pgCAPECRepository{db: db}
}

func (r *pgCAPECRepository) List(ctx context.Context, q, cweID string, page, limit int) ([]*entity.CAPECEntry, int64, error) {
	conditions := []string{}
	args := []interface{}{limit, page * limit}

	if q != "" {
		args = append(args, "%"+q+"%")
		conditions = append(conditions, fmt.Sprintf("(lower(name) LIKE lower($%d) OR lower(id) LIKE lower($%d))", len(args), len(args)))
	}
	if cweID != "" {
		args = append(args, cweID)
		conditions = append(conditions, fmt.Sprintf("$%d = ANY(cwe_ids)", len(args)))
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var entries []*entity.CAPECEntry
	query := fmt.Sprintf(`SELECT id, name, description, likelihood, severity, updated_at
        FROM capec_patterns %s ORDER BY id LIMIT $1 OFFSET $2`, where)
	err := r.db.SelectContext(ctx, &entries, query, args...)

	var total int64
	if len(args) > 2 {
		r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM capec_patterns "+where, args[2:]...).Scan(&total)
	} else {
		r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM capec_patterns "+where).Scan(&total)
	}

	return entries, total, err
}

func (r *pgCAPECRepository) FindByID(ctx context.Context, id string) (*entity.CAPECEntry, error) {
	var entry entity.CAPECEntry
	err := r.db.GetContext(ctx, &entry, `SELECT id, name, description, likelihood, severity, updated_at
        FROM capec_patterns WHERE id = $1`, id)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	return &entry, err
}
