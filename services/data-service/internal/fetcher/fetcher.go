// Package fetcher defines the Fetcher interface for data source ingestion.
package fetcher

import "context"

// Fetcher defines the interface for all data source fetchers.
type Fetcher interface {
	// Name returns the source identifier: "cve", "cpe", "cwe", "capec", "epss", "via4"
	Name() string

	// FetchAndStore fetches data from the remote source and upserts into MongoDB.
	// Returns the count of documents processed.
	FetchAndStore(ctx context.Context, opts FetchOptions) (int, error)
}

// FetchOptions controls fetch behavior.
type FetchOptions struct {
	// ManualDays: 0 = full import, >0 = only fetch data modified in last N days
	ManualDays int

	// StartYear: for CVE fetcher, first year to import (default 2002)
	StartYear int
}
