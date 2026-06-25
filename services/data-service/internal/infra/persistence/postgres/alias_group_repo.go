// Package postgres implements AliasGroupRepository backed by PostgreSQL.
// This is an additive alternative to the Firestore implementation.
// Selection is controlled by the ALIAS_GROUP_BACKEND env var:
//
//	ALIAS_GROUP_BACKEND=firestore (default) → uses infra/persistence/firestore/alias_group_repo.go
//	ALIAS_GROUP_BACKEND=postgres            → uses this file
//
// The database schema mirrors the Firestore two-collection approach:
//   - alias_groups        → group documents (group_id, bug_ids[], canonical_id, detection_method)
//   - alias_group_members → denormalized member index (vuln_id → group_id)
//
// Migration: migrations/005_create_alias_groups.up.sql
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	aliasgroup "github.com/osv/data-service/internal/domain/aggregate/alias_group"
	"github.com/osv/data-service/internal/domain/repository"
	"github.com/osv/data-service/internal/domain/valueobject"
)

// AliasGroupRepository implements repository.AliasGroupRepository using PostgreSQL.
type AliasGroupRepository struct {
	db *pgxpool.Pool
}

// NewAliasGroupRepository creates a PostgreSQL-backed AliasGroupRepository.
// Satisfies the same repository.AliasGroupRepository interface as the Firestore repo.
func NewAliasGroupRepository(db *pgxpool.Pool) repository.AliasGroupRepository {
	return &AliasGroupRepository{db: db}
}

// GetByMemberID finds the AliasGroup containing the given vulnID.
// Uses the alias_group_members index for O(1) lookup — mirrors Firestore collectionGroupMembers.
// Returns (nil, nil) if no group is found (matches Firestore behaviour).
func (r *AliasGroupRepository) GetByMemberID(ctx context.Context, vulnID string) (*aliasgroup.AliasGroup, error) {
	var groupID string
	err := r.db.QueryRow(ctx,
		`SELECT group_id FROM alias_group_members WHERE vuln_id = $1`,
		vulnID,
	).Scan(&groupID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // not found — matches Firestore (nil, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("get member index for %s: %w", vulnID, err)
	}

	return r.GetByGroupID(ctx, groupID)
}

// GetByGroupID retrieves an AliasGroup by its group ID.
// Returns (nil, nil) if not found — matches Firestore behaviour.
func (r *AliasGroupRepository) GetByGroupID(ctx context.Context, groupID string) (*aliasgroup.AliasGroup, error) {
	var (
		bugIDs          []string
		lastModified    time.Time
		detectionMethod string
	)

	err := r.db.QueryRow(ctx,
		`SELECT bug_ids, last_modified, detection_method
		 FROM alias_groups
		 WHERE group_id = $1`,
		groupID,
	).Scan(&bugIDs, &lastModified, &detectionMethod)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // not found
	}
	if err != nil {
		return nil, fmt.Errorf("get alias group %s: %w", groupID, err)
	}

	return aliasgroup.ReconstitueAliasGroup(
		groupID,
		bugIDs,
		lastModified,
		valueobject.DetectionMethod(detectionMethod),
	), nil
}

// Save persists an AliasGroup and its member index entries.
// Mirrors the Firestore batch.Set behaviour — upserts both the group and member rows.
func (r *AliasGroupRepository) Save(ctx context.Context, group *aliasgroup.AliasGroup) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	bugIDs := group.BugIDs()

	// 1. Upsert the group document
	_, err = tx.Exec(ctx, `
		INSERT INTO alias_groups (group_id, bug_ids, canonical_id, detection_method, last_modified)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (group_id) DO UPDATE SET
			bug_ids          = EXCLUDED.bug_ids,
			canonical_id     = EXCLUDED.canonical_id,
			detection_method = EXCLUDED.detection_method,
			last_modified    = EXCLUDED.last_modified
	`,
		group.ID(),
		bugIDs,
		group.CanonicalID(),
		string(group.DetectionMethod()),
		group.LastModified(),
	)
	if err != nil {
		return fmt.Errorf("upsert alias group %s: %w", group.ID(), err)
	}

	// 2. Upsert each member in the member index
	// This replicates the Firestore batch.Set on collectionGroupMembers
	for _, vulnID := range bugIDs {
		_, err = tx.Exec(ctx, `
			INSERT INTO alias_group_members (vuln_id, group_id)
			VALUES ($1, $2)
			ON CONFLICT (vuln_id) DO UPDATE SET group_id = EXCLUDED.group_id
		`,
			vulnID,
			group.ID(),
		)
		if err != nil {
			return fmt.Errorf("upsert member index for %s: %w", vulnID, err)
		}
	}

	return tx.Commit(ctx)
}

// Delete removes an AliasGroup by ID.
// CASCADE on alias_group_members ensures member rows are deleted automatically.
func (r *AliasGroupRepository) Delete(ctx context.Context, groupID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM alias_groups WHERE group_id = $1`,
		groupID,
	)
	if err != nil {
		return fmt.Errorf("delete alias group %s: %w", groupID, err)
	}
	return nil
}

// DeleteMember removes the member index entry for a vuln ID.
// Mirrors Firestore collectionGroupMembers.Doc(vulnID).Delete().
func (r *AliasGroupRepository) DeleteMember(ctx context.Context, vulnID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM alias_group_members WHERE vuln_id = $1`,
		vulnID,
	)
	if err != nil {
		return fmt.Errorf("delete member index for %s: %w", vulnID, err)
	}
	return nil
}
