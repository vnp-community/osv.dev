// Package firestore implements SourceRepositoryRepo backed by Cloud Firestore.
package firestore

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"
	"github.com/osv/source-sync/internal/domain/aggregate/source_repository"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const collectionSources = "source_repositories"

// SourceRepositoryRepo persists SourceRepository aggregate state in Firestore.
type SourceRepositoryRepo struct {
	client *firestore.Client
	log    zerolog.Logger
}

// NewSourceRepositoryRepo creates a Firestore-backed SourceRepositoryRepo.
func NewSourceRepositoryRepo(client *firestore.Client, log zerolog.Logger) *SourceRepositoryRepo {
	return &SourceRepositoryRepo{client: client, log: log}
}

// GetByName retrieves a SourceRepository by its name.
func (r *SourceRepositoryRepo) GetByName(ctx context.Context, name string) (*source_repository.SourceRepository, error) {
	doc, err := r.client.Collection(collectionSources).Doc(name).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, fmt.Errorf("source %s not found", name)
		}
		return nil, fmt.Errorf("get source %s: %w", name, err)
	}

	data := doc.Data()
	repo := source_repository.ReconstitueFromStore(
		name,
		stringField(data, "source_type"),
		stringField(data, "repo_url"),
		stringField(data, "bucket"),
		stringField(data, "rest_api_url"),
		stringField(data, "directory_path"),
		stringField(data, "extension"),
		stringField(data, "last_synced_hash"),
		boolField(data, "strict_validation"),
	)
	return repo, nil
}

// Save persists the current state of a SourceRepository.
func (r *SourceRepositoryRepo) Save(ctx context.Context, repo *source_repository.SourceRepository) error {
	_, err := r.client.Collection(collectionSources).Doc(repo.Name()).Set(ctx, map[string]interface{}{
		"name":             repo.Name(),
		"source_type":      repo.SourceType(),
		"repo_url":         repo.RepoURL(),
		"bucket":           repo.BucketName(),
		"rest_api_url":     repo.RESTURL(),
		"directory_path":   repo.DirectoryPath(),
		"extension":        repo.Extension(),
		"last_synced_hash": repo.LastSyncedHash(),
		"last_update_date": repo.LastUpdateDate(),
		"strict_validation": repo.StrictValidation(),
	})
	if err != nil {
		return fmt.Errorf("save source %s: %w", repo.Name(), err)
	}
	r.log.Debug().Str("source", repo.Name()).Msg("source state saved")
	return nil
}

// List returns all configured sources.
func (r *SourceRepositoryRepo) List(ctx context.Context) ([]*source_repository.SourceRepository, error) {
	docs, err := r.client.Collection(collectionSources).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}

	repos := make([]*source_repository.SourceRepository, 0, len(docs))
	for _, doc := range docs {
		data := doc.Data()
		repos = append(repos, source_repository.ReconstitueFromStore(
			doc.Ref.ID,
			stringField(data, "source_type"),
			stringField(data, "repo_url"),
			stringField(data, "bucket"),
			stringField(data, "rest_api_url"),
			stringField(data, "directory_path"),
			stringField(data, "extension"),
			stringField(data, "last_synced_hash"),
			boolField(data, "strict_validation"),
		))
	}
	return repos, nil
}

func stringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func boolField(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}
