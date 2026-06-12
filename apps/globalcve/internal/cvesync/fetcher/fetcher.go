// Package fetcher defines the Fetcher interface and options for data source ingestion.
// Adapted from ingestion-service/internal/fetcher/fetcher.go.
package fetcher

import (
	"context"
	"time"

	cveentity "github.com/globalcve/mono/internal/cvesearch/domain/entity"
)

// Fetcher defines the interface for all CVE data source fetchers.
type Fetcher interface {
	// Source returns the data source identifier.
	Source() cveentity.SourceName

	// Fetch retrieves data from the remote source and upserts into PostgreSQL.
	// Returns the count of records processed and any error.
	Fetch(ctx context.Context, opts FetchOptions) (int, error)
}

// FetchOptions controls fetch behavior.
type FetchOptions struct {
	// Since: nil = full sync, non-nil = only fetch records modified after this time.
	Since *time.Time

	// Force: bypass idempotency checks and re-fetch everything.
	Force bool

	// StartYear: for CVE fetchers, the first year to import (default 2002).
	StartYear int

	// ManualDays: if > 0, fetch only records from the last N days.
	ManualDays int
}
