// Package firestore implements RepoIndexRepository and RepoIndexBucketRepository.
package firestore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/osv/impact-service/internal/domain/index/repository"
	"github.com/rs/zerolog"
)

const (
	collIndexes  = "repo_indexes"
	collBuckets  = "repo_index_buckets"
)

// RepoIndexRepo persists RepoIndex documents in Firestore.
type RepoIndexRepo struct {
	client *firestore.Client
	log    zerolog.Logger
}

// NewRepoIndexRepo creates a Firestore-backed RepoIndexRepository.
func NewRepoIndexRepo(client *firestore.Client, log zerolog.Logger) repository.RepoIndexRepository {
	return &RepoIndexRepo{client: client, log: log}
}

// Save writes a repo index metadata document.
// Document ID: "{slug(repoURL)}_{tag}"
func (r *RepoIndexRepo) Save(ctx context.Context, repoURL, tag string, fileCount int, indexedAt time.Time) error {
	docID := slugID(repoURL, tag)
	_, err := r.client.Collection(collIndexes).Doc(docID).Set(ctx, map[string]interface{}{
		"repo_url":   repoURL,
		"tag":        tag,
		"file_count": fileCount,
		"indexed_at": indexedAt,
	})
	if err != nil {
		return fmt.Errorf("save index %s@%s: %w", repoURL, tag, err)
	}
	return nil
}

// ExistsForTag returns true if the repo at the given tag is already indexed.
func (r *RepoIndexRepo) ExistsForTag(ctx context.Context, repoURL, tag string) (bool, error) {
	docID := slugID(repoURL, tag)
	doc, err := r.client.Collection(collIndexes).Doc(docID).Get(ctx)
	if err != nil {
		return false, nil // treat as not found
	}
	return doc.Exists(), nil
}

// GetByRepoVersion retrieves the index metadata for a specific repo+tag.
func (r *RepoIndexRepo) GetByRepoVersion(ctx context.Context, repoURL, tag string) (fileCount int, err error) {
	docID := slugID(repoURL, tag)
	doc, err := r.client.Collection(collIndexes).Doc(docID).Get(ctx)
	if err != nil {
		return 0, fmt.Errorf("get index %s@%s: %w", repoURL, tag, err)
	}

	data := doc.Data()
	if fc, ok := data["file_count"].(int64); ok {
		return int(fc), nil
	}
	return 0, nil
}

// ── RepoIndexBucketRepo ───────────────────────────────────────────────────────

// RepoIndexBucketRepo persists bucket hash entries in Firestore.
type RepoIndexBucketRepo struct {
	client *firestore.Client
	log    zerolog.Logger
}

// NewRepoIndexBucketRepo creates a Firestore-backed RepoIndexBucketRepository.
func NewRepoIndexBucketRepo(client *firestore.Client, log zerolog.Logger) repository.RepoIndexBucketRepository {
	return &RepoIndexBucketRepo{client: client, log: log}
}

// Save writes a single bucket hash entry.
// Document ID: "{bucketIdx}_{bucketHash}" for fast lookup by hash.
func (r *RepoIndexBucketRepo) Save(
	ctx context.Context,
	repoURL, tag string,
	bucketIdx int,
	bucketHash string,
) error {
	docID := fmt.Sprintf("%03d_%s", bucketIdx, bucketHash[:8])
	_, err := r.client.Collection(collBuckets).Doc(docID).Set(ctx, map[string]interface{}{
		"repo_url":    repoURL,
		"tag":         tag,
		"bucket_idx":  bucketIdx,
		"bucket_hash": bucketHash,
	}, firestore.MergeAll)
	if err != nil {
		return fmt.Errorf("save bucket %d: %w", bucketIdx, err)
	}
	return nil
}

// QueryByBucketHash returns all repo+version pairs that have the given hash in the given bucket.
func (r *RepoIndexBucketRepo) QueryByBucketHash(
	ctx context.Context,
	bucketIdx int,
	bucketHash string,
	limit int,
) ([]repository.BucketMatch, error) {
	docs, err := r.client.Collection(collBuckets).
		Where("bucket_idx", "==", bucketIdx).
		Where("bucket_hash", "==", bucketHash).
		Limit(limit).
		Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("query bucket %d: %w", bucketIdx, err)
	}

	matches := make([]repository.BucketMatch, 0, len(docs))
	for _, doc := range docs {
		data := doc.Data()
		matches = append(matches, repository.BucketMatch{
			RepoURL: stringField(data, "repo_url"),
			Version: stringField(data, "tag"),
		})
	}
	return matches, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func slugID(repoURL, tag string) string {
	import_md5 := func(s string) string {
		import_crypto_md5 := fmt.Sprintf("%x", []byte(s)[:8])
		return import_crypto_md5
	}
	return import_md5(repoURL) + "_" + tag
}

func stringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
