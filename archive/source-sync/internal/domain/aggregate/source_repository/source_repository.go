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

// Package source_repository contains the SourceRepository aggregate root.
package source_repository

import (
	"fmt"
	"time"

	pkgerrors "github.com/osv/pkg/errors"
	"github.com/osv/source-sync/internal/domain/valueobject"
)

// SourceRepository is the aggregate root managing a single external vulnerability source.
// It encapsulates both the source configuration and sync state.
type SourceRepository struct {
	name          string
	sourceType    valueobject.SourceType
	repoURL       string // for GIT
	bucket        string // for BUCKET
	restAPIURL    string // for REST_ENDPOINT
	directoryPath string
	extension     string   // ".json" | ".yaml"
	dbPrefixes    []string // ID prefixes owned by this source (e.g., ["GHSA"])

	// Sync state — persisted to Firestore
	lastSyncedHash string
	lastUpdateDate time.Time

	// Behavior flags
	strictValidation    bool
	ignoreGit           bool
	versionsFromRepo    bool
	detectCherryPicks   bool
	considerAllBranches bool
}

// NewSourceRepository creates a new SourceRepository aggregate.
func NewSourceRepository(name string, sourceType valueobject.SourceType) (*SourceRepository, error) {
	if name == "" {
		return nil, fmt.Errorf("source repository name must not be empty")
	}
	return &SourceRepository{
		name:       name,
		sourceType: sourceType,
	}, nil
}

// Name returns the source name.
func (s *SourceRepository) Name() string { return s.name }

// SourceType returns the source type.
func (s *SourceRepository) SourceType() valueobject.SourceType { return s.sourceType }

// LastSyncedHash returns the last successfully synced git hash or ETag.
func (s *SourceRepository) LastSyncedHash() string { return s.lastSyncedHash }

// LastUpdateDate returns the time of the last sync.
func (s *SourceRepository) LastUpdateDate() time.Time { return s.lastUpdateDate }

// RepoURL returns the Git repository URL (for GIT type).
func (s *SourceRepository) RepoURL() string { return s.repoURL }

// Bucket returns the GCS bucket name (for BUCKET type).
func (s *SourceRepository) Bucket() string { return s.bucket }

// RESTAPIURL returns the REST API URL (for REST_ENDPOINT type).
func (s *SourceRepository) RESTAPIURL() string { return s.restAPIURL }

// DirectoryPath returns the path within the repo/bucket to watch.
func (s *SourceRepository) DirectoryPath() string { return s.directoryPath }

// Extension returns the file extension filter (e.g., ".json").
func (s *SourceRepository) Extension() string { return s.extension }

// DBPrefixes returns the vulnerability ID prefixes owned by this source.
func (s *SourceRepository) DBPrefixes() []string { return s.dbPrefixes }

// MarkSynced updates the sync state after a successful sync.
func (s *SourceRepository) MarkSynced(hash string, syncedAt time.Time) {
	s.lastSyncedHash = hash
	s.lastUpdateDate = syncedAt
}

// CheckDeletionSafety returns an error if the proposed deletion count exceeds the threshold.
// Business rule: if toDeleteCount/totalCount >= 10%, reject the deletion.
func (s *SourceRepository) CheckDeletionSafety(toDeleteCount, totalCount int) error {
	if totalCount == 0 {
		return nil
	}
	pct := float64(toDeleteCount) / float64(totalCount) * 100.0
	if pct >= 10.0 {
		return pkgerrors.NewDeletionSafetyError(toDeleteCount, totalCount, 10.0)
	}
	return nil
}

// FromConfig populates the aggregate from a SourceConfig.
func FromConfig(cfg SourceConfig) (*SourceRepository, error) {
	sr, err := NewSourceRepository(cfg.Name, valueobject.SourceType(cfg.Type))
	if err != nil {
		return nil, err
	}
	sr.repoURL = cfg.RepoURL
	sr.bucket = cfg.Bucket
	sr.restAPIURL = cfg.RESTAPIURL
	sr.directoryPath = cfg.DirectoryPath
	sr.extension = cfg.Extension
	sr.dbPrefixes = cfg.DBPrefixes
	sr.strictValidation = cfg.StrictValidation
	sr.ignoreGit = cfg.IgnoreGit
	sr.versionsFromRepo = cfg.VersionsFromRepo
	sr.detectCherryPicks = cfg.DetectCherryPicks
	sr.considerAllBranches = cfg.ConsiderAllBranches
	return sr, nil
}

// SourceConfig holds the declarative configuration for a source, loaded from sources.yaml.
type SourceConfig struct {
	Name                string   `yaml:"name"`
	Type                string   `yaml:"type"` // GIT | BUCKET | REST_ENDPOINT
	RepoURL             string   `yaml:"repo_url,omitempty"`
	Bucket              string   `yaml:"bucket,omitempty"`
	RESTAPIURL          string   `yaml:"rest_api_url,omitempty"`
	DirectoryPath       string   `yaml:"directory_path"`
	Extension           string   `yaml:"extension"`
	DBPrefixes          []string `yaml:"db_prefixes"`
	SyncInterval        string   `yaml:"sync_interval"` // e.g., "5m", "1h"
	StrictValidation    bool     `yaml:"strict_validation"`
	IgnoreGit           bool     `yaml:"ignore_git"`
	VersionsFromRepo    bool     `yaml:"versions_from_repo"`
	DetectCherryPicks   bool     `yaml:"detect_cherry_picks"`
	ConsiderAllBranches bool     `yaml:"consider_all_branches"`
}
