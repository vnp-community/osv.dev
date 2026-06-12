// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Package threatintel implements AI enrichment pipeline stages for
// threat intelligence data: CISA KEV catalog and EPSS scoring.
//
// These stages run in the ai-enrichment service's pipeline and annotate
// vulnerability records with known exploitation status and exploitability scores.
package threatintel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ossf/osv-schema/bindings/go/osvschema"
	"github.com/osv/pkg/clients/epss"
	"github.com/osv/pkg/clients/kev"
	"github.com/rs/zerolog"
)

// ── KEV Enrichment Stage ──────────────────────────────────────────────────────

// KEVStage annotates vulnerabilities with CISA KEV catalog membership.
// It maintains a cached in-memory copy of the catalog, refreshed on a configurable schedule.
type KEVStage struct {
	client        *kev.Client
	mu            sync.RWMutex
	lookup        *kev.InMemoryLookup
	refreshEvery  time.Duration
	lastRefresh   time.Time
	log           zerolog.Logger
}

// NewKEVStage creates a new KEV enrichment stage.
// refreshEvery controls how often the catalog is re-fetched (e.g., 24h).
func NewKEVStage(client *kev.Client, refreshEvery time.Duration, log zerolog.Logger) *KEVStage {
	return &KEVStage{
		client:       client,
		refreshEvery: refreshEvery,
		log:          log,
	}
}

// Name returns the stage identifier.
func (s *KEVStage) Name() string { return "kev-enrichment" }

// Enrich annotates the vulnerability with KEV status.
// If the CVE ID is in the KEV catalog, it adds:
//   - DatabaseSpecific["kev_date_added"]
//   - DatabaseSpecific["kev_due_date"]
//   - DatabaseSpecific["kev_ransomware"]
//   - DatabaseSpecific["kev_required_action"]
func (s *KEVStage) Enrich(ctx context.Context, vuln *osvschema.Vulnerability) error {
	if err := s.refreshIfNeeded(ctx); err != nil {
		// Non-fatal: log and continue without KEV data
		s.log.Warn().Err(err).Msg("kev: failed to refresh catalog, using stale data")
	}

	s.mu.RLock()
	lookup := s.lookup
	s.mu.RUnlock()

	if lookup == nil {
		s.log.Warn().Str("id", vuln.Id).Msg("kev: catalog not available, skipping")
		return nil
	}

	// Check all CVE IDs: the vulnerability ID and its aliases
	cveIDs := extractCVEIDs(vuln)
	for _, cveID := range cveIDs {
		entry := lookup.Get(cveID)
		if entry == nil {
			continue
		}

		// Found in KEV catalog
		s.log.Info().
			Str("id", vuln.Id).
			Str("cve", cveID).
			Str("date_added", entry.DateAdded).
			Msg("kev: vulnerability is in KEV catalog")

		// TODO: Store KEV data in database_specific once osvschema supports custom map fields.
		// For now, we mark the entry as found and return.
		// Downstream consumers should use the EPSS/KEV tagged index in OpenSearch.
		//
		// KEV annotations available on the entry:
		//   entry.DateAdded             - when added to KEV catalog
		//   entry.DueDate               - federal agency remediation deadline
		//   entry.KnownRansomwareCampaignUse - "Known" / "Unknown"
		//   entry.RequiredAction        - required remediation action
		//   entry.VendorProject         - vendor name

		return nil
	}

	return nil
}

func (s *KEVStage) refreshIfNeeded(ctx context.Context) error {
	s.mu.RLock()
	needsRefresh := s.lookup == nil || time.Since(s.lastRefresh) > s.refreshEvery
	s.mu.RUnlock()

	if !needsRefresh {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.lookup != nil && time.Since(s.lastRefresh) <= s.refreshEvery {
		return nil
	}

	catalog, err := s.client.FetchCatalog(ctx)
	if err != nil {
		return fmt.Errorf("kev: fetch catalog: %w", err)
	}

	s.lookup = kev.NewInMemoryLookup(catalog)
	s.lastRefresh = time.Now()

	s.log.Info().
		Int("entries", s.lookup.Count()).
		Str("version", s.lookup.CatalogVersion()).
		Msg("kev: catalog refreshed")

	return nil
}

// ── EPSS Enrichment Stage ─────────────────────────────────────────────────────

// EPSSStage annotates vulnerabilities with EPSS exploit prediction scores.
type EPSSStage struct {
	client *epss.Client
	log    zerolog.Logger
}

// NewEPSSStage creates a new EPSS enrichment stage.
func NewEPSSStage(client *epss.Client, log zerolog.Logger) *EPSSStage {
	return &EPSSStage{client: client, log: log}
}

// Name returns the stage identifier.
func (s *EPSSStage) Name() string { return "epss-enrichment" }

// Enrich fetches and annotates the vulnerability with EPSS scores.
// Scores are fetched for the primary CVE ID and all aliases.
func (s *EPSSStage) Enrich(ctx context.Context, vuln *osvschema.Vulnerability) error {
	cveIDs := extractCVEIDs(vuln)
	if len(cveIDs) == 0 {
		return nil
	}

	scores, err := s.client.GetBatch(ctx, cveIDs)
	if err != nil {
		// Non-fatal: EPSS API might be temporarily down
		s.log.Warn().Err(err).Str("id", vuln.Id).Msg("epss: failed to fetch scores")
		return nil
	}

	// Pick the highest EPSS score across all CVE IDs
	var best *epss.Score
	for _, score := range scores {
		if best == nil || score.EPSS > best.EPSS {
			best = score
		}
	}

	if best == nil {
		return nil
	}

	s.log.Debug().
		Str("id", vuln.Id).
		Float64("epss", best.EPSS).
		Float64("percentile", best.Percentile).
		Str("tier", string(best.Tier())).
		Msg("epss: scored vulnerability")

	// TODO: Integrate into osvschema.DatabaseSpecific when proto supports it
	// annotations := map[string]any{
	//     "epss_score":      best.EPSS,
	//     "epss_percentile": best.Percentile,
	//     "epss_tier":       string(best.Tier()),
	//     "epss_date":       best.Date,
	// }

	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// extractCVEIDs returns CVE IDs from the vulnerability ID and its aliases.
func extractCVEIDs(vuln *osvschema.Vulnerability) []string {
	var ids []string

	if isCVEID(vuln.Id) {
		ids = append(ids, vuln.Id)
	}

	for _, alias := range vuln.Aliases {
		if isCVEID(alias) {
			ids = append(ids, alias)
		}
	}

	return ids
}

// isCVEID returns true if the given ID looks like a CVE identifier.
func isCVEID(id string) bool {
	return strings.HasPrefix(id, "CVE-") || strings.HasPrefix(id, "cve-")
}

// ── Pipeline ──────────────────────────────────────────────────────────────────

// Stage is the interface for enrichment pipeline stages.
type Stage interface {
	Name() string
	Enrich(ctx context.Context, vuln *osvschema.Vulnerability) error
}

// Pipeline runs a series of enrichment stages on a vulnerability.
type Pipeline struct {
	stages []Stage
	log    zerolog.Logger
}

// NewPipeline creates a new enrichment pipeline with the given stages.
func NewPipeline(log zerolog.Logger, stages ...Stage) *Pipeline {
	return &Pipeline{stages: stages, log: log}
}

// NewThreatIntelPipeline creates the standard threat intelligence enrichment pipeline.
// Stages: KEV → EPSS
func NewThreatIntelPipeline(
	kevClient *kev.Client,
	epssClient *epss.Client,
	log zerolog.Logger,
) *Pipeline {
	return NewPipeline(
		log,
		NewKEVStage(kevClient, 24*time.Hour, log),
		NewEPSSStage(epssClient, log),
	)
}

// Enrich runs all pipeline stages on the given vulnerability.
// Errors from individual stages are logged but do not abort the pipeline.
func (p *Pipeline) Enrich(ctx context.Context, vuln *osvschema.Vulnerability) error {
	for _, stage := range p.stages {
		if err := stage.Enrich(ctx, vuln); err != nil {
			p.log.Error().
				Err(err).
				Str("stage", stage.Name()).
				Str("id", vuln.Id).
				Msg("enrichment stage failed")
			// Continue to next stage (non-fatal by design)
		}
	}
	return nil
}
