// Package index_repository implements the indexing pipeline for a git repository version.
// Worker mode: clone at tag → walk files → hash → write 512 bucket entries to Firestore.
package index_repository

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/osv/version-index/internal/domain/repository"
	"github.com/osv/version-index/internal/domain/service"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

// Command triggers indexing of a specific repository tag.
type Command struct {
	RepoURL  string
	Tag      string
	CommitID string // optional, for pinned commits
}

// RepoIndex holds metadata written to Firestore after indexing.
type RepoIndex struct {
	RepoURL   string
	Tag       string
	FileCount int
	IndexedAt time.Time
}

// Handler clones a repo at a tag, computes 512-bucket hashes, and writes to Firestore.
type Handler struct {
	bucketRepo  repository.RepoIndexBucketRepository
	indexRepo   repository.RepoIndexRepository
	hasher      *service.BucketHasher
	cloneDir    string
	tracer      trace.Tracer
	log         zerolog.Logger
}

// NewHandler creates a new IndexRepository handler.
func NewHandler(
	bucketRepo repository.RepoIndexBucketRepository,
	indexRepo repository.RepoIndexRepository,
	hasher *service.BucketHasher,
	cloneDir string,
	tracer trace.Tracer,
	log zerolog.Logger,
) *Handler {
	return &Handler{
		bucketRepo: bucketRepo,
		indexRepo:  indexRepo,
		hasher:     hasher,
		cloneDir:   cloneDir,
		tracer:     tracer,
		log:        log,
	}
}

// Handle executes the full indexing pipeline for a repo@tag.
func (h *Handler) Handle(ctx context.Context, cmd Command) error {
	ctx, span := h.tracer.Start(ctx, "IndexRepository")
	defer span.End()

	h.log.Info().Str("repo", cmd.RepoURL).Str("tag", cmd.Tag).Msg("indexing started")

	// 1. Clone or open the repo
	repoDir := filepath.Join(h.cloneDir, repoSlug(cmd.RepoURL))
	repo, err := h.cloneOrOpen(ctx, cmd.RepoURL, repoDir)
	if err != nil {
		return fmt.Errorf("clone %s: %w", cmd.RepoURL, err)
	}

	// 2. Checkout tag
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	tagRef, err := repo.Tag(cmd.Tag)
	if err != nil {
		// Try refs/tags/ prefix
		tagRef, err = repo.Reference(plumbing.NewTagReferenceName(cmd.Tag), true)
		if err != nil {
			return fmt.Errorf("resolve tag %s: %w", cmd.Tag, err)
		}
	}

	checkoutOpts := &git.CheckoutOptions{Hash: tagRef.Hash()}
	if err := worktree.Checkout(checkoutOpts); err != nil {
		return fmt.Errorf("checkout %s@%s: %w", cmd.RepoURL, cmd.Tag, err)
	}

	// 3. Walk all files and compute MD5 hashes
	fileHashes, err := h.walkFiles(worktree)
	if err != nil {
		return fmt.Errorf("walk files: %w", err)
	}

	h.log.Debug().Str("repo", cmd.RepoURL).Str("tag", cmd.Tag).Int("files", len(fileHashes)).Msg("files hashed")

	// 4. Compute 512-bucket hash set
	bucketSet := h.hasher.Hash(fileHashes)

	// 5. Write bucket entries to Firestore (batch write for performance)
	if err := h.writeBuckets(ctx, cmd.RepoURL, cmd.Tag, bucketSet); err != nil {
		return fmt.Errorf("write buckets: %w", err)
	}

	// 6. Write repo index metadata
	idx := &RepoIndex{
		RepoURL:   cmd.RepoURL,
		Tag:       cmd.Tag,
		FileCount: len(fileHashes),
		IndexedAt: time.Now().UTC(),
	}
	if err := h.indexRepo.Save(ctx, idx.RepoURL, idx.Tag, idx.FileCount, idx.IndexedAt); err != nil {
		return fmt.Errorf("save index: %w", err)
	}

	h.log.Info().Str("repo", cmd.RepoURL).Str("tag", cmd.Tag).Int("files", len(fileHashes)).Msg("indexing complete")
	return nil
}

// walkFiles returns a map of file path → MD5 hex hash for all files in the worktree.
func (h *Handler) walkFiles(wt *git.Worktree) (map[string]string, error) {
	fs := wt.Filesystem
	fileHashes := make(map[string]string)

	var walk func(dir string) error
	walk = func(dir string) error {
		entries, err := fs.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				if entry.Name() == ".git" {
					continue
				}
				if err := walk(path); err != nil {
					return err
				}
				continue
			}

			f, err := fs.Open(path)
			if err != nil {
				continue
			}
			h := md5.New()
			buf := make([]byte, 32*1024)
			for {
				n, err := f.Read(buf)
				if n > 0 {
					h.Write(buf[:n])
				}
				if err != nil {
					break
				}
			}
			f.Close()
			fileHashes[path] = hex.EncodeToString(h.Sum(nil))
		}
		return nil
	}

	if err := walk("/"); err != nil {
		return nil, err
	}
	return fileHashes, nil
}

// writeBuckets writes all non-empty bucket entries to Firestore.
func (h *Handler) writeBuckets(ctx context.Context, repoURL, tag string, bs *service.BucketSet) error {
	for i, hashes := range bs.Buckets {
		if len(hashes) == 0 {
			continue
		}
		bucketHash := h.hasher.ComputeBucketHash(hashes)
		if err := h.bucketRepo.Save(ctx, repoURL, tag, i, bucketHash); err != nil {
			return fmt.Errorf("save bucket %d: %w", i, err)
		}
	}
	return nil
}

func (h *Handler) cloneOrOpen(ctx context.Context, repoURL, dir string) (*git.Repository, error) {
	repo, err := git.PlainOpen(dir)
	if err == nil {
		// Pull latest
		wt, _ := repo.Worktree()
		wt.Pull(&git.PullOptions{RemoteName: "origin"}) //nolint:errcheck
		return repo, nil
	}

	// Clone
	repo, err = git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{
		URL:   repoURL,
		Depth: 0, // full clone needed for tag checkout
	})
	if err != nil {
		return nil, fmt.Errorf("git clone: %w", err)
	}
	return repo, nil
}

func repoSlug(repoURL string) string {
	h := md5.Sum([]byte(repoURL))
	return hex.EncodeToString(h[:8])
}

// Force object.File reference for linter
var _ = (*object.File)(nil)
