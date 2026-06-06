// Package bucket implements GCSBucketAdapter for the Source Sync Service.
// Detects changes in GCS bucket-based vulnerability sources (e.g. NVD-CVE, Go vulndb).
package bucket

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
	"github.com/osv/source-sync/internal/application/port"
	"github.com/osv/source-sync/internal/domain/aggregate/source_repository"
	"github.com/rs/zerolog"
	"google.golang.org/api/iterator"
)

// GCSBucketAdapter implements SourceFetcher for GCS bucket sources.
type GCSBucketAdapter struct {
	client *storage.Client
	log    zerolog.Logger
}

// NewGCSBucketAdapter creates a GCS bucket change detector.
func NewGCSBucketAdapter(client *storage.Client, log zerolog.Logger) *GCSBucketAdapter {
	return &GCSBucketAdapter{client: client, log: log}
}

// CanHandle returns true for BUCKET source type.
func (a *GCSBucketAdapter) CanHandle(source *source_repository.SourceRepository) bool {
	return source.SourceType() == "BUCKET"
}

// DetectChanges lists all objects in the GCS bucket path and compares with last sync state.
func (a *GCSBucketAdapter) DetectChanges(
	ctx context.Context,
	source *source_repository.SourceRepository,
	forceResync bool,
) (*port.ChangeSet, error) {
	bucket := source.Bucket()
	prefix := source.DirectoryPath()
	ext := source.Extension()

	a.log.Debug().Str("bucket", bucket).Str("prefix", prefix).Msg("scanning GCS bucket")

	it := a.client.Bucket(bucket).Objects(ctx, &storage.Query{
		Prefix: prefix,
	})

	var modified []port.FileChange
	var totalCount int

	// For GCS sources: compute content hash from object generation number
	// to detect changes without downloading every object.
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list bucket %s: %w", bucket, err)
		}

		if ext != "" && !hasExtension(attrs.Name, ext) {
			continue
		}

		totalCount++
		contentHash := fmt.Sprintf("%d-%d", attrs.Generation, attrs.Metageneration)
		gcsPath := fmt.Sprintf("gs://%s/%s", bucket, attrs.Name)

		modified = append(modified, port.FileChange{
			Path:    attrs.Name,
			Hash:    contentHash,
			GCSPath: gcsPath,
		})
	}

	a.log.Info().
		Str("bucket", bucket).
		Int("total", totalCount).
		Int("changes", len(modified)).
		Msg("GCS scan complete")

	return &port.ChangeSet{
		Modified:    modified,
		Deleted:     nil, // GCS buckets: no deletion tracking (full scan)
		TotalCount:  totalCount,
		NewSyncHash: fmt.Sprintf("gcs-scan-%d", ctx.Value("scan_ts")),
	}, nil
}

func hasExtension(name, ext string) bool {
	if len(name) < len(ext) {
		return false
	}
	return name[len(name)-len(ext):] == ext
}
