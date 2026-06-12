// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package git provides the go-git-based implementation of the SourceFetcher for GIT sources.
package git

import (
	"context"
	"fmt"
	"strings"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/rs/zerolog/log"

	"github.com/osv/source-sync/internal/domain/aggregate/source_repository"
	"github.com/osv/source-sync/internal/domain/valueobject"
)

// GitSyncAdapter implements SourceFetcher for GIT-type sources using go-git.
type GitSyncAdapter struct {
	cloneDir string // base directory for git clones (e.g., "/tmp/osv-git-repos")
}

// NewGitSyncAdapter creates a new GitSyncAdapter.
func NewGitSyncAdapter(cloneDir string) *GitSyncAdapter {
	return &GitSyncAdapter{cloneDir: cloneDir}
}

// DetectChanges clones or fetches the repository and returns all changed files
// since the last synced hash.
func (a *GitSyncAdapter) DetectChanges(
	ctx context.Context,
	source *source_repository.SourceRepository,
	forceResync bool,
) (*valueobject.ChangeSet, error) {
	logger := log.Ctx(ctx).With().Str("source", source.Name()).Logger()

	repoDir := fmt.Sprintf("%s/%s", a.cloneDir, source.Name())

	// Clone or open existing repo
	var repo *gogit.Repository
	var err error

	repo, err = gogit.PlainOpen(repoDir)
	if err == gogit.ErrRepositoryNotExists {
		logger.Info().Str("url", source.RepoURL()).Msg("cloning repository")
		repo, err = gogit.PlainCloneContext(ctx, repoDir, &gogit.CloneOptions{
			URL:      source.RepoURL(),
			Progress: nil,
			Depth:    0,
		})
		if err != nil {
			return nil, fmt.Errorf("git clone %q: %w", source.RepoURL(), err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("git open %q: %w", repoDir, err)
	} else {
		// Fetch latest
		w, _ := repo.Worktree()
		err = w.PullContext(ctx, &gogit.PullOptions{RemoteName: "origin"})
		if err != nil && err != gogit.NoErrAlreadyUpToDate {
			return nil, fmt.Errorf("git fetch %q: %w", source.Name(), err)
		}
	}

	// Get HEAD
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("git head %q: %w", source.Name(), err)
	}
	headHash := ref.Hash().String()

	// If forceResync or no last synced hash, return all files
	lastHash := source.LastSyncedHash()
	if forceResync || lastHash == "" {
		changeSet, err := a.allFiles(repo, source.DirectoryPath(), source.Extension())
		if err != nil {
			return nil, err
		}
		changeSet.NewSyncHash = headHash
		return changeSet, nil
	}

	// Walk commits from lastHash to HEAD
	changeSet, err := a.walkCommits(repo, lastHash, headHash, source.DirectoryPath(), source.Extension())
	if err != nil {
		return nil, err
	}
	changeSet.NewSyncHash = headHash
	return changeSet, nil
}

func (a *GitSyncAdapter) walkCommits(
	repo *gogit.Repository,
	fromHash, toHash string,
	dirPath, extension string,
) (*valueobject.ChangeSet, error) {
	fromCommit, err := repo.CommitObject(plumbing.NewHash(fromHash))
	if err != nil {
		return nil, fmt.Errorf("resolve from commit %q: %w", fromHash, err)
	}
	toCommit, err := repo.CommitObject(plumbing.NewHash(toHash))
	if err != nil {
		return nil, fmt.Errorf("resolve to commit %q: %w", toHash, err)
	}

	patch, err := fromCommit.Patch(toCommit)
	if err != nil {
		return nil, fmt.Errorf("git diff %q..%q: %w", fromHash, toHash, err)
	}

	modified := map[string]valueobject.FileChange{}
	deleted := map[string]valueobject.FileChange{}

	for _, fp := range patch.FilePatches() {
		from, to := fp.Files()

		if to == nil {
			// Deleted file
			if from != nil && matchesFilter(from.Path(), dirPath, extension) {
				deleted[from.Path()] = valueobject.FileChange{Path: from.Path()}
			}
			continue
		}

		toPath := to.Path()
		if !matchesFilter(toPath, dirPath, extension) {
			continue
		}

		// Skip [no-update] commits
		content := getFileContent(repo, to.Hash().String())
		modified[toPath] = valueobject.FileChange{
			Path:    toPath,
			Hash:    to.Hash().String(),
			Content: content,
		}
	}

	mods := make([]valueobject.FileChange, 0, len(modified))
	for _, f := range modified {
		mods = append(mods, f)
	}
	dels := make([]valueobject.FileChange, 0, len(deleted))
	for _, f := range deleted {
		dels = append(dels, f)
	}

	return &valueobject.ChangeSet{
		Modified:   mods,
		Deleted:    dels,
		TotalCount: len(mods) + len(dels),
	}, nil
}

func (a *GitSyncAdapter) allFiles(repo *gogit.Repository, dirPath, extension string) (*valueobject.ChangeSet, error) {
	ref, _ := repo.Head()
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	var files []valueobject.FileChange
	err = tree.Files().ForEach(func(f *object.File) error {
		if !matchesFilter(f.Name, dirPath, extension) {
			return nil
		}
		content, _ := f.Contents()
		files = append(files, valueobject.FileChange{
			Path:    f.Name,
			Hash:    f.Hash.String(),
			Content: []byte(content),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &valueobject.ChangeSet{
		Modified:   files,
		TotalCount: len(files),
	}, nil
}

func matchesFilter(path, dirPath, extension string) bool {
	if dirPath != "" && !strings.HasPrefix(path, dirPath) {
		return false
	}
	if extension != "" && !strings.HasSuffix(path, extension) {
		return false
	}
	return true
}

func getFileContent(repo *gogit.Repository, blobHash string) []byte {
	blob, err := repo.BlobObject(plumbing.NewHash(blobHash))
	if err != nil {
		return nil
	}
	r, err := blob.Reader()
	if err != nil {
		return nil
	}
	defer r.Close()
	buf := make([]byte, blob.Size)
	_, _ = r.Read(buf)
	return buf
}
