// Package repository defines persistence interfaces for the Version Index Service.
package repository

import (
	"context"
	"time"
)

// BucketMatch is returned by QueryByBucketHash.
type BucketMatch struct {
	RepoURL string
	Version string
}

// RepoIndexRepository persists and queries repo index metadata.
type RepoIndexRepository interface {
	// Save writes a repo index metadata document.
	Save(ctx context.Context, repoURL, tag string, fileCount int, indexedAt time.Time) error

	// ExistsForTag returns true if this repo@tag is already indexed.
	ExistsForTag(ctx context.Context, repoURL, tag string) (bool, error)

	// GetByRepoVersion returns the file count for a specific repo@version.
	GetByRepoVersion(ctx context.Context, repoURL, version string) (fileCount int, err error)
}

// RepoIndexBucketRepository persists and queries bucket hash entries.
type RepoIndexBucketRepository interface {
	// Save writes a single bucket entry (repoURL, tag, bucketIdx, bucketHash).
	Save(ctx context.Context, repoURL, tag string, bucketIdx int, bucketHash string) error

	// QueryByBucketHash returns repo+version pairs matching bucket_idx + bucket_hash.
	// Used by DetermineVersion to find candidate versions.
	QueryByBucketHash(ctx context.Context, bucketIdx int, bucketHash string, limit int) ([]BucketMatch, error)
}
