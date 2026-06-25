// Package fetcher defines the Fetcher interface and Registry for data source ingestion.
package fetcher

import (
	"context"
	"time"
)

// SourceName — canonical identifier for each data source.
type SourceName string

const (
	// Core sources (existing)
	SourceNVD    SourceName = "NVD"
	SourceEPSS   SourceName = "EPSS"
	SourceNVDCPE SourceName = "NVD-CPE"
	SourceCAPEC  SourceName = "CAPEC"
	SourceCWE    SourceName = "CWE"
	SourceVIA4   SourceName = "VIA4"
	SourceCIRCL  SourceName = "CIRCL"

	// New sources from CR-GCV-001
	SourceJVN       SourceName = "JVN"
	SourceExploitDB SourceName = "EXPLOITDB"
	SourceCVEOrg    SourceName = "CVE.ORG"
	SourceCNNVD     SourceName = "CNNVD"

	// Beta / future
	SourceAndroid SourceName = "ANDROID"
	SourceCERTFR  SourceName = "CERT-FR"
)

// String returns the string value of the SourceName.
func (s SourceName) String() string { return string(s) }


// Fetcher defines the interface for all data source fetchers.
type Fetcher interface {
	// Name returns the source identifier: "NVD", "CIRCL", "JVN", etc.
	Name() string

	// FetchAndStore fetches data from the remote source and upserts into MongoDB.
	// Returns the count of documents processed.
	FetchAndStore(ctx context.Context, opts FetchOptions) (int, error)
}

// IncrementalFetcher extends Fetcher for sources that support delta sync.
type IncrementalFetcher interface {
	Fetcher
	// FetchSince fetches only data changed since the given time.
	FetchSince(ctx context.Context, since time.Time) (int, error)
}

// FetchOptions controls fetch behavior.
type FetchOptions struct {
	// ManualDays: 0 = full import, >0 = only fetch data modified in last N days
	ManualDays int

	// StartYear: for CVE fetcher, first year to import (default 2002)
	StartYear int
}

// FetchResult aggregates statistics from a fetch operation.
type FetchResult struct {
	Source    SourceName
	Fetched   int
	Upserted  int
	Skipped   int
	Errors    int
	Duration  time.Duration
	StartedAt time.Time
}
