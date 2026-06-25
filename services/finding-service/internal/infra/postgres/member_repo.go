package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/member"
)

// MemberRepo implements member.ProductMemberRepository using pgxpool.
type MemberRepo struct {
	pool *pgxpool.Pool
}

// NewMemberRepo creates a new MemberRepo.
func NewMemberRepo(pool *pgxpool.Pool) *MemberRepo {
	return &MemberRepo{pool: pool}
}

// Save inserts or updates a ProductMember (upsert by product_id, user_id).
func (r *MemberRepo) Save(ctx context.Context, m *member.ProductMember) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO product_members (id, product_id, user_id, role, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (product_id, user_id) DO UPDATE SET role = EXCLUDED.role
	`, m.ID, m.ProductID, m.UserID, string(m.Role), m.CreatedAt)
	return err
}

// FindByProductAndUser finds a member by product and user IDs.
func (r *MemberRepo) FindByProductAndUser(ctx context.Context, productID, userID uuid.UUID) (*member.ProductMember, error) {
	m := &member.ProductMember{}
	var roleStr string
	err := r.pool.QueryRow(ctx,
		`SELECT id, product_id, user_id, role, created_at FROM product_members WHERE product_id=$1 AND user_id=$2`,
		productID, userID,
	).Scan(&m.ID, &m.ProductID, &m.UserID, &roleStr, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("member not found: %w", err)
	}
	m.Role = member.Role(roleStr)
	return m, nil
}

// ListByProduct returns all members of a product.
func (r *MemberRepo) ListByProduct(ctx context.Context, productID uuid.UUID) ([]*member.ProductMember, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, product_id, user_id, role, created_at FROM product_members WHERE product_id=$1 ORDER BY created_at`,
		productID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*member.ProductMember
	for rows.Next() {
		m := &member.ProductMember{}
		var roleStr string
		if err := rows.Scan(&m.ID, &m.ProductID, &m.UserID, &roleStr, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Role = member.Role(roleStr)
		members = append(members, m)
	}
	return members, rows.Err()
}

// Delete removes a member from a product.
func (r *MemberRepo) Delete(ctx context.Context, productID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM product_members WHERE product_id=$1 AND user_id=$2`,
		productID, userID,
	)
	return err
}

// GetRole returns the role of a user in a product, or nil if not a member.
func (r *MemberRepo) GetRole(ctx context.Context, productID, userID uuid.UUID) (*member.Role, error) {
	var roleStr string
	err := r.pool.QueryRow(ctx,
		`SELECT role FROM product_members WHERE product_id=$1 AND user_id=$2`,
		productID, userID,
	).Scan(&roleStr)
	if err != nil {
		return nil, nil // not a member
	}
	role := member.Role(roleStr)
	return &role, nil
}
