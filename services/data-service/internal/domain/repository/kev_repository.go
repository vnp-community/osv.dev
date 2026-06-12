// Package repository defines the KEV domain repository interface.
package repository

import (
	"context"

	keventity "github.com/osv/data-service/internal/domain/kev"
)

// KEVRepository is the persistence interface for KEV catalog data.
type KEVRepository interface {
	// UpsertBatch inserts or updates a batch of KEV entries.
	// Returns the count of newly inserted and updated records.
	UpsertBatch(ctx context.Context, entries []*keventity.KEVEntry) (inserted, updated int, err error)

	// FindByCVEID returns a single KEV entry by CVE ID.
	// Returns ErrKEVNotFound when not present.
	FindByCVEID(ctx context.Context, cveID string) (*keventity.KEVEntry, error)

	// List returns paginated KEV entries matching the filter.
	List(ctx context.Context, filter *keventity.KEVFilter) ([]*keventity.KEVEntry, int64, error)

	// CheckMany reports whether each CVE ID in the slice is in the KEV catalog.
	// Returns a map of cveID → isKEV.
	CheckMany(ctx context.Context, cveIDs []string) (map[string]bool, error)

	// GetAllIDs returns all CVE IDs present in the catalog.
	GetAllIDs(ctx context.Context) ([]string, error)

	// Count returns the total number of entries in the catalog.
	Count(ctx context.Context) (int64, error)

	// Stats returns statistical information about the catalog.
	Stats(ctx context.Context) (*keventity.KEVStats, error)
}
