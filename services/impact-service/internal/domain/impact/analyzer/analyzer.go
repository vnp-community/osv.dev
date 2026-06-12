// Package analyzer ports the core analyze() function from osv/impact.py.
// TASK-10-01b: Orchestrates version enumeration + git range analysis for a vulnerability.
package analyzer

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/osv/impact-service/internal/domain/impact/rangecollector"
	"github.com/osv/impact-service/internal/domain/impact/valueobject"
)

// RangeType classifies a version range.
type RangeType string

const (
	RangeTypeGIT       RangeType = "GIT"
	RangeTypeEcosystem RangeType = "ECOSYSTEM"
	RangeTypeSemver    RangeType = "SEMVER"
)

// Event represents a single version event (introduced/fixed/last_affected/limit).
type Event struct {
	Introduced   string
	Fixed        string
	LastAffected string
	Limit        string
}

// AffectedRange is one range block from a vulnerability record.
type AffectedRange struct {
	Type      RangeType
	RepoURL   string // only for GIT ranges
	Events    []Event
}

// AffectedPackage is one package entry from a vulnerability record.
type AffectedPackage struct {
	Name      string
	Ecosystem string
	Ranges    []AffectedRange
	// Current known versions (from the OSV record, if any)
	Versions []string
}

// AnalyzeRequest is the input to the Analyzer.
type AnalyzeRequest struct {
	VulnID            string
	Affected          []AffectedPackage
	AnalyzeGit        bool // whether to perform git commit analysis
	DetectCherryPicks bool
	ConsiderAllBranches bool
	VersionsFromRepo  bool // add repo-derived tags to affected.versions
}

// AnalyzeResult captures what changed after analysis.
type AnalyzeResult struct {
	VulnID          string
	HasChanges      bool
	AffectedCommits []string // git commit SHAs
	NewVersions     map[string][]string // package → new versions found
	NewIntroduced   []string // cherrypick-detected introduced commits
	NewFixed        []string // cherrypick-detected fixed commits
	Notes           []string
}

// VersionEnumerator enumerates package versions for ECOSYSTEM/SEMVER ranges.
type VersionEnumerator interface {
	// EnumerateVersions returns all versions in [introduced, fixed) or [introduced, lastAffected].
	EnumerateVersions(ctx context.Context, ecosystem, pkg, introduced, fixed, lastAffected string, limits []string) ([]string, error)
	// IsSupported returns true if the ecosystem is supported.
	IsSupported(ecosystem string) bool
}

// GitAnalyzer performs git range analysis (commit enumeration + cherrypick detection).
type GitAnalyzer interface {
	// Analyze enumerates affected commits and tags for a GIT range.
	Analyze(ctx context.Context, repoURL string, events []valueobject.VersionRangeEvent, opts GitAnalyzeOptions) (*GitAnalyzeResult, error)
}

// GitAnalyzeOptions controls git analysis behavior.
type GitAnalyzeOptions struct {
	DetectCherryPicks   bool
	ConsiderAllBranches bool
	MaxCommits          int
}

// GitAnalyzeResult is the output of git range analysis.
type GitAnalyzeResult struct {
	Commits       []string // affected commit SHAs
	Tags          []string // affected version tags
	Ranges        []rangecollector.CommitRange
	NewIntroduced []string // cherrypick-detected introduced commits
	NewFixed      []string // cherrypick-detected fixed commits
}

// Analyzer orchestrates version enumeration and git analysis.
// Port of osv/impact.py:analyze()
type Analyzer struct {
	versionEnumerator VersionEnumerator
	gitAnalyzer       GitAnalyzer // may be nil (skips git analysis)
}

// NewAnalyzer creates a new impact Analyzer.
func NewAnalyzer(ve VersionEnumerator, ga GitAnalyzer) *Analyzer {
	return &Analyzer{
		versionEnumerator: ve,
		gitAnalyzer:       ga,
	}
}

// Analyze performs impact analysis on a vulnerability.
// Port of osv/impact.py:analyze() — orchestrates version enumeration and git range analysis.
func (a *Analyzer) Analyze(ctx context.Context, req AnalyzeRequest) (*AnalyzeResult, error) {
	logger := log.Ctx(ctx).With().Str("vuln_id", req.VulnID).Logger()
	result := &AnalyzeResult{
		VulnID:      req.VulnID,
		NewVersions: make(map[string][]string),
	}

	commitSet := make(map[string]bool)
	introSet := make(map[string]bool)
	fixedSet := make(map[string]bool)

	for _, affected := range req.Affected {
		var pkgVersions []string

		for _, affRange := range affected.Ranges {
			switch affRange.Type {
			case RangeTypeEcosystem, RangeTypeSemver:
				if a.versionEnumerator == nil || !a.versionEnumerator.IsSupported(affected.Ecosystem) {
					logger.Warn().
						Str("ecosystem", affected.Ecosystem).
						Msg("no version enumerator for ecosystem, skipping")
					result.Notes = append(result.Notes, fmt.Sprintf(
						"no version enumerator for %s (pkg=%s)", affected.Ecosystem, affected.Name))
					continue
				}

				vers, err := a.enumerateVersionRange(ctx, affected.Ecosystem, affected.Name, affRange)
				if err != nil {
					logger.Warn().Err(err).
						Str("ecosystem", affected.Ecosystem).
						Str("package", affected.Name).
						Msg("version enumeration failed")
					result.Notes = append(result.Notes, fmt.Sprintf(
						"enumerate %s/%s: %v", affected.Ecosystem, affected.Name, err))
					continue
				}
				pkgVersions = append(pkgVersions, vers...)

			case RangeTypeGIT:
				if !req.AnalyzeGit || a.gitAnalyzer == nil {
					continue
				}

				events := eventsToValueObjects(affRange.Events)
				gitResult, err := a.gitAnalyzer.Analyze(ctx, affRange.RepoURL, events, GitAnalyzeOptions{
					DetectCherryPicks:   req.DetectCherryPicks,
					ConsiderAllBranches: req.ConsiderAllBranches,
				})
				if err != nil {
					logger.Warn().Err(err).Str("repo", affRange.RepoURL).Msg("git analysis failed")
					result.Notes = append(result.Notes, fmt.Sprintf("git analysis %s: %v", affRange.RepoURL, err))
					continue
				}

				for _, c := range gitResult.Commits {
					commitSet[c] = true
				}
				for _, tag := range gitResult.Tags {
					if req.VersionsFromRepo {
						pkgVersions = append(pkgVersions, tag)
					}
				}
				for _, intro := range gitResult.NewIntroduced {
					introSet[intro] = true
				}
				for _, fix := range gitResult.NewFixed {
					fixedSet[fix] = true
				}
			}
		}

		// Store new versions for this package (deduplicated + sorted)
		if len(pkgVersions) > 0 {
			pkgKey := affected.Ecosystem + "/" + affected.Name
			existing := make(map[string]bool)
			for _, v := range affected.Versions {
				existing[v] = true
			}
			var newVers []string
			for _, v := range pkgVersions {
				if !existing[v] {
					existing[v] = true
					newVers = append(newVers, v)
				}
			}
			if len(newVers) > 0 {
				sort.Strings(newVers)
				result.NewVersions[pkgKey] = newVers
				result.HasChanges = true
			}
		}
	}

	// Collect affected commits
	for c := range commitSet {
		result.AffectedCommits = append(result.AffectedCommits, c)
	}
	sort.Strings(result.AffectedCommits)
	if len(result.AffectedCommits) > 0 {
		result.HasChanges = true
	}

	// Collect cherry-pick detections
	for c := range introSet {
		result.NewIntroduced = append(result.NewIntroduced, c)
	}
	for c := range fixedSet {
		result.NewFixed = append(result.NewFixed, c)
	}
	sort.Strings(result.NewIntroduced)
	sort.Strings(result.NewFixed)

	logger.Info().
		Bool("has_changes", result.HasChanges).
		Int("commits", len(result.AffectedCommits)).
		Int("new_versions_pkgs", len(result.NewVersions)).
		Msg("impact analysis complete")

	return result, nil
}

// enumerateVersionRange enumerates versions from an ECOSYSTEM/SEMVER range.
// Port of osv/impact.py:enumerate_versions()
func (a *Analyzer) enumerateVersionRange(ctx context.Context, ecosystem, pkg string, r AffectedRange) ([]string, error) {
	var limits []string
	var zeroEvent *Event
	var sortedEvents []Event

	// Separate '0' introduced events and limit events (port of Python logic)
	for _, ev := range r.Events {
		ev := ev // capture
		if ev.Introduced == "0" {
			zeroEvent = &ev
			continue
		}
		if ev.Limit != "" {
			limits = append(limits, ev.Limit)
			continue
		}
		if ev.Introduced != "" || ev.Fixed != "" || ev.LastAffected != "" {
			sortedEvents = append(sortedEvents, ev)
		}
	}

	// Prepend zero event if present
	if zeroEvent != nil {
		sortedEvents = append([]Event{*zeroEvent}, sortedEvents...)
	}

	versions := make(map[string]bool)

	var lastIntroduced string
	for _, ev := range sortedEvents {
		if ev.Introduced != "" && lastIntroduced == "" {
			lastIntroduced = ev.Introduced
		}
		if lastIntroduced != "" && ev.Fixed != "" {
			vers, err := a.versionEnumerator.EnumerateVersions(ctx, ecosystem, pkg, lastIntroduced, ev.Fixed, "", limits)
			if err != nil {
				return nil, err
			}
			for _, v := range vers {
				versions[v] = true
			}
			lastIntroduced = ""
		}
		if lastIntroduced != "" && ev.LastAffected != "" {
			vers, err := a.versionEnumerator.EnumerateVersions(ctx, ecosystem, pkg, lastIntroduced, "", ev.LastAffected, limits)
			if err != nil {
				return nil, err
			}
			for _, v := range vers {
				versions[v] = true
			}
			lastIntroduced = ""
		}
	}

	// Open-ended range (no fixed)
	if lastIntroduced != "" {
		vers, err := a.versionEnumerator.EnumerateVersions(ctx, ecosystem, pkg, lastIntroduced, "", "", limits)
		if err != nil {
			return nil, err
		}
		for _, v := range vers {
			versions[v] = true
		}
	}

	result := make([]string, 0, len(versions))
	for v := range versions {
		result = append(result, v)
	}
	sort.Strings(result)
	return result, nil
}

// eventsToValueObjects converts Event slice to valueobject.VersionRangeEvent slice.
func eventsToValueObjects(events []Event) []valueobject.VersionRangeEvent {
	var out []valueobject.VersionRangeEvent
	for _, ev := range events {
		if ev.Introduced != "" {
			out = append(out, valueobject.VersionRangeEvent{
				Type:    valueobject.RangeEventIntroduced,
				Version: ev.Introduced,
			})
		}
		if ev.Fixed != "" {
			out = append(out, valueobject.VersionRangeEvent{
				Type:    valueobject.RangeEventFixed,
				Version: ev.Fixed,
			})
		}
		if ev.LastAffected != "" {
			out = append(out, valueobject.VersionRangeEvent{
				Type:    valueobject.RangeEventLastAffected,
				Version: ev.LastAffected,
			})
		}
		if ev.Limit != "" {
			out = append(out, valueobject.VersionRangeEvent{
				Type:    valueobject.RangeEventLimit,
				Version: ev.Limit,
			})
		}
	}
	return out
}

// IsSemverEcosystem returns true for ecosystems that use semver ordering.
// Mirrors osv/ecosystems.is_semver().
func IsSemverEcosystem(ecosystem string) bool {
	semverEcosystems := []string{
		"npm", "cargo", "go", "ruby", "crates.io",
		"golang", "packagist", "hex", "pub",
	}
	lower := strings.ToLower(ecosystem)
	for _, e := range semverEcosystems {
		if lower == e {
			return true
		}
	}
	return false
}
