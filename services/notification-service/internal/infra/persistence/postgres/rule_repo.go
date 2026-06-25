// Package postgres — rule_repo.go
// PostgresRuleRepo implements rule.Repository using PostgreSQL.
//
// This is ADDITIVE — the Firestore implementation (firestore/repos.go) is preserved.
// Backend selection is done at startup via RULE_BACKEND env var.
//
// Schema: notification_rules table from migration 005_notification_rules.up.sql
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/osv/notification-service/internal/domain/rule"
)

// PostgresRuleRepo implements rule.Repository using pgxpool.
type PostgresRuleRepo struct {
	db  *pgxpool.Pool
	log zerolog.Logger
}

// NewRuleRepo creates a new PostgresRuleRepo.
func NewRuleRepo(db *pgxpool.Pool, log zerolog.Logger) *PostgresRuleRepo {
	return &PostgresRuleRepo{db: db, log: log}
}

// Compile-time check: PostgresRuleRepo must satisfy rule.Repository.
var _ rule.Repository = (*PostgresRuleRepo)(nil)

// ── rule.Repository implementation ──────────────────────────────────────────

const ruleSelectCols = `
	id, user_id, product_id,
	scan_added, finding_added, finding_status_changed,
	jira_update, engagement_added, engagement_closed,
	risk_acceptance_expiration, sla_breach, sla_expiring_soon,
	is_active, created_at, updated_at`

// FindMatchingRules returns active rules that match the given event type and optional product.
// Matches: system-wide rules (user_id IS NULL) + product-scoped rules.
func (r *PostgresRuleRepo) FindMatchingRules(ctx context.Context, eventType rule.EventType, productID *string) ([]*rule.NotificationRule, error) {
	var pid *uuid.UUID
	if productID != nil {
		parsed, err := uuid.Parse(*productID)
		if err != nil {
			return nil, fmt.Errorf("rule_repo: invalid product_id %q: %w", *productID, err)
		}
		pid = &parsed
	}

	query := `
		SELECT ` + ruleSelectCols + `
		FROM notification_rules
		WHERE is_active = TRUE
		  AND (product_id IS NULL OR product_id = $1)
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, pid)
	if err != nil {
		return nil, fmt.Errorf("rule_repo: FindMatchingRules: %w", err)
	}
	defer rows.Close()

	return scanRules(rows, r.log)
}

// GetSystemRule returns the system-wide rule (user_id IS NULL, product_id IS NULL).
func (r *PostgresRuleRepo) GetSystemRule(ctx context.Context) (*rule.NotificationRule, error) {
	query := `
		SELECT ` + ruleSelectCols + `
		FROM notification_rules
		WHERE user_id IS NULL AND product_id IS NULL AND is_active = TRUE
		LIMIT 1
	`
	row := r.db.QueryRow(ctx, query)
	nr, err := scanRule(row)
	if err != nil {
		return nil, fmt.Errorf("rule_repo: GetSystemRule: %w", err)
	}
	return nr, nil
}

// Create persists a new NotificationRule.
func (r *PostgresRuleRepo) Create(ctx context.Context, nr *rule.NotificationRule) error {
	query := `
		INSERT INTO notification_rules (
			user_id, product_id,
			scan_added, finding_added, finding_status_changed,
			jira_update, engagement_added, engagement_closed,
			risk_acceptance_expiration, sla_breach, sla_expiring_soon,
			is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, TRUE)
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		nr.UserID, nr.ProductID,
		channelsToStrings(nr.ScanAdded),
		channelsToStrings(nr.FindingAdded),
		channelsToStrings(nr.FindingStatusChanged),
		channelsToStrings(nr.JIRAUpdate),
		channelsToStrings(nr.EngagementAdded),
		channelsToStrings(nr.EngagementClosed),
		channelsToStrings(nr.RiskAcceptanceExpiration),
		channelsToStrings(nr.SLABreach),
		channelsToStrings(nr.SLAExpiringSoon),
	).Scan(&nr.ID, &nr.CreatedAt, &nr.UpdatedAt)
}

// Update updates mutable channel lists on an existing rule.
func (r *PostgresRuleRepo) Update(ctx context.Context, nr *rule.NotificationRule) error {
	query := `
		UPDATE notification_rules SET
			scan_added = $2, finding_added = $3, finding_status_changed = $4,
			jira_update = $5, engagement_added = $6, engagement_closed = $7,
			risk_acceptance_expiration = $8, sla_breach = $9, sla_expiring_soon = $10,
			updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query,
		nr.ID,
		channelsToStrings(nr.ScanAdded),
		channelsToStrings(nr.FindingAdded),
		channelsToStrings(nr.FindingStatusChanged),
		channelsToStrings(nr.JIRAUpdate),
		channelsToStrings(nr.EngagementAdded),
		channelsToStrings(nr.EngagementClosed),
		channelsToStrings(nr.RiskAcceptanceExpiration),
		channelsToStrings(nr.SLABreach),
		channelsToStrings(nr.SLAExpiringSoon),
	)
	return err
}

// Delete soft-deletes a rule (sets is_active = FALSE).
func (r *PostgresRuleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE notification_rules SET is_active = FALSE, updated_at = NOW() WHERE id = $1`,
		id,
	)
	return err
}

// ListForUser returns all active rules belonging to a user.
func (r *PostgresRuleRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]*rule.NotificationRule, error) {
	query := `
		SELECT ` + ruleSelectCols + `
		FROM notification_rules
		WHERE user_id = $1 AND is_active = TRUE
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("rule_repo: ListForUser: %w", err)
	}
	defer rows.Close()

	return scanRules(rows, r.log)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func scanRules(rows pgx.Rows, log zerolog.Logger) ([]*rule.NotificationRule, error) {
	var rules []*rule.NotificationRule
	for rows.Next() {
		nr, err := scanRule(rows)
		if err != nil {
			log.Warn().Err(err).Msg("rule_repo: skip row on scan error")
			continue
		}
		rules = append(rules, nr)
	}
	return rules, rows.Err()
}

// scanRule scans a pgx Row or Rows into a NotificationRule.
// Accepts both pgx.Row and pgx.Rows via the shared Scan interface.
func scanRule(scanner interface{ Scan(...any) error }) (*rule.NotificationRule, error) {
	nr := &rule.NotificationRule{}

	var (
		userID    *uuid.UUID
		productID *uuid.UUID
		isActive  bool // not in domain struct — filtered by query

		// Channel lists from Postgres TEXT[] arrays
		scanAdded                []string
		findingAdded             []string
		findingStatusChanged     []string
		jiraUpdate               []string
		engagementAdded          []string
		engagementClosed         []string
		riskAcceptanceExpiration []string
		slaBreach                []string
		slaExpiringSoon          []string
	)

	err := scanner.Scan(
		&nr.ID, &userID, &productID,
		&scanAdded, &findingAdded, &findingStatusChanged,
		&jiraUpdate, &engagementAdded, &engagementClosed,
		&riskAcceptanceExpiration, &slaBreach, &slaExpiringSoon,
		&isActive, &nr.CreatedAt, &nr.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = isActive // infra-only field, not propagated to domain

	nr.UserID = userID
	nr.ProductID = productID
	nr.ScanAdded = stringsToChannels(scanAdded)
	nr.FindingAdded = stringsToChannels(findingAdded)
	nr.FindingStatusChanged = stringsToChannels(findingStatusChanged)
	nr.JIRAUpdate = stringsToChannels(jiraUpdate)
	nr.EngagementAdded = stringsToChannels(engagementAdded)
	nr.EngagementClosed = stringsToChannels(engagementClosed)
	nr.RiskAcceptanceExpiration = stringsToChannels(riskAcceptanceExpiration)
	nr.SLABreach = stringsToChannels(slaBreach)
	nr.SLAExpiringSoon = stringsToChannels(slaExpiringSoon)

	return nr, nil
}

// channelsToStrings converts []rule.Channel → []string for Postgres TEXT[].
func channelsToStrings(channels []rule.Channel) []string {
	strs := make([]string, 0, len(channels))
	for _, ch := range channels {
		strs = append(strs, string(ch))
	}
	return strs
}

// stringsToChannels converts []string from Postgres TEXT[] → []rule.Channel.
func stringsToChannels(strs []string) []rule.Channel {
	if len(strs) == 0 {
		return nil
	}
	channels := make([]rule.Channel, 0, len(strs))
	for _, s := range strs {
		channels = append(channels, rule.Channel(s))
	}
	return channels
}
