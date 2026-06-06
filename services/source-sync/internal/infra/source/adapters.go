// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Package source provides concrete source adapters for the source-sync service.
// Each adapter implements the sync_source.Source interface.
package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/source-sync/internal/application/command/sync_source"
)

// ─── OSV GCS Source ──────────────────────────────────────────────────────────

// OSVGCSSource reads OSV vulnerability files from a GCS bucket.
type OSVGCSSource struct {
	bucketName string
	log        zerolog.Logger
}

// NewOSVGCSSource creates an adapter backed by a GCS bucket.
func NewOSVGCSSource(bucketName string, log zerolog.Logger) *OSVGCSSource {
	return &OSVGCSSource{bucketName: bucketName, log: log}
}

// Name returns the source identifier.
func (s *OSVGCSSource) Name() string { return "osv-gcs" }

// FetchChanges lists changed OSV files since the last sync.
// TODO: implement using cloud.google.com/go/storage bucket listing.
func (s *OSVGCSSource) FetchChanges(ctx context.Context, since time.Time) ([]sync_source.ChangedFile, error) {
	s.log.Debug().Str("bucket", s.bucketName).Time("since", since).
		Msg("osv-gcs: listing changed objects (stub)")
	return nil, nil
}

// ─── GitHub Advisory Source ──────────────────────────────────────────────────

// githubAdvisory represents a single item from the GitHub advisory API.
type githubAdvisory struct {
	GHSAID      string    `json:"ghsa_id"`
	CVEId       string    `json:"cve_id"`
	HTMLUrl     string    `json:"html_url"`
	Summary     string    `json:"summary"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GitHubAdvisorySource reads GitHub Security Advisories via the REST API.
type GitHubAdvisorySource struct {
	token      string
	httpClient *http.Client
	log        zerolog.Logger
}

// NewGitHubAdvisorySource creates an adapter backed by the GitHub Advisories API.
func NewGitHubAdvisorySource(token string, log zerolog.Logger) *GitHubAdvisorySource {
	return &GitHubAdvisorySource{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: log,
	}
}

// Name returns the source identifier.
func (s *GitHubAdvisorySource) Name() string { return "ghsa" }

// FetchChanges lists GitHub advisories updated since the given time.
func (s *GitHubAdvisorySource) FetchChanges(ctx context.Context, since time.Time) ([]sync_source.ChangedFile, error) {
	url := fmt.Sprintf(
		"https://api.github.com/advisories?type=reviewed&updated_after=%s&per_page=100",
		since.UTC().Format(time.RFC3339),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ghsa: create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ghsa: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("ghsa: unexpected status %d: %s", resp.StatusCode, body)
	}

	var advisories []githubAdvisory
	if err := json.NewDecoder(resp.Body).Decode(&advisories); err != nil {
		return nil, fmt.Errorf("ghsa: decode response: %w", err)
	}

	// Sort by updated_at ascending for deterministic processing.
	sort.Slice(advisories, func(i, j int) bool {
		return advisories[i].UpdatedAt.Before(advisories[j].UpdatedAt)
	})

	var changes []sync_source.ChangedFile
	for _, adv := range advisories {
		id := adv.GHSAID
		if adv.CVEId != "" {
			id = adv.CVEId
		}
		path := strings.ToLower(id) + ".json"
		data, _ := json.Marshal(adv)
		changes = append(changes, sync_source.ChangedFile{
			Path:    path,
			Content: data,
			Hash:    fmt.Sprintf("%d", adv.UpdatedAt.UnixNano()),
		})
	}

	s.log.Info().
		Int("count", len(changes)).
		Time("since", since).
		Msg("ghsa: fetched advisories")

	return changes, nil
}
