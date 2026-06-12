// Package repository defines the KEV repository interfaces.
package repository

import (
	"context"

	"github.com/globalcve/mono/internal/kevservice/domain/entity"
)

// KEVRepository defines all KEV persistence operations.
type KEVRepository interface {
	// UpsertBatch inserts or updates multiple KEV entries.
	// Returns counts of inserted and updated entries.
	UpsertBatch(ctx context.Context, entries []*entity.KEVEntry) (inserted, updated int, err error)

	// FindByCVEID retrieves a KEV entry by CVE ID.
	FindByCVEID(ctx context.Context, cveID string) (*entity.KEVEntry, error)

	// List retrieves KEV entries with optional filtering and pagination.
	List(ctx context.Context, filter *entity.KEVFilter) ([]*entity.KEVEntry, int64, error)

	// CheckMany checks a list of CVE IDs and returns which are in KEV.
	CheckMany(ctx context.Context, cveIDs []string) (map[string]bool, error)

	// GetAllIDs returns all CVE IDs in the KEV catalog.
	GetAllIDs(ctx context.Context) ([]string, error)

	// Count returns the total number of KEV entries.
	Count(ctx context.Context) (int64, error)

	// Stats returns statistical information about the KEV catalog.
	Stats(ctx context.Context) (*entity.KEVStats, error)
}
