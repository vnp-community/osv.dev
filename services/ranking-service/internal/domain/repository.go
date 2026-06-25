package domain

import "context"

// RankingRepository defines data access for CPE ranking entries.
type RankingRepository interface {
	// FindByCPE performs loosy CPE lookup.
	// Splits CPE string into parts and finds the first matching entry.
	// Returns error if no ranking found for any CPE part.
	FindByCPE(ctx context.Context, cpe string) (*LookupResult, error)

	// FindExact returns a ranking entry by exact CPE string match.
	FindExact(ctx context.Context, cpe string) (*RankingEntry, error)

	// List returns paginated ranking entries.
	List(ctx context.Context, limit, skip int) ([]*RankingEntry, int64, error)

	// Save upserts a ranking entry (by CPE — idempotent).
	Save(ctx context.Context, entry *RankingEntry) (*RankingEntry, error)

	// Delete removes a ranking entry by ID.
	// Returns error if not found.
	Delete(ctx context.Context, id string) error
}
