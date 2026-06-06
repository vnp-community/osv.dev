// Package models — Bug struct porting osv/models.py Bug(ndb.Model).
// This is the Go equivalent of the Python Bug Datastore model.
package models

import (
	"strings"
	"time"
	"unicode"

	"github.com/ossf/osv-schema/bindings/go/osvschema"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BugStatus represents the processing status of a vulnerability record.
type BugStatus int

const (
	BugStatusUnprocessed BugStatus = 0
	BugStatusProcessed   BugStatus = 1
	BugStatusInvalid     BugStatus = 2
	BugStatusDeleted     BugStatus = 3
)

func (s BugStatus) String() string {
	switch s {
	case BugStatusUnprocessed:
		return "UNPROCESSED"
	case BugStatusProcessed:
		return "PROCESSED"
	case BugStatusInvalid:
		return "INVALID"
	case BugStatusDeleted:
		return "DELETED"
	default:
		return "UNKNOWN"
	}
}

// AffectedPackage is a simplified representation of an affected package entry.
type AffectedPackage struct {
	Package   PackageRef
	Ecosystem string
	Ranges    []VersionRange
	Versions  []string
	PURL      string
}

// PackageRef identifies a package within an ecosystem.
type PackageRef struct {
	Name      string
	Ecosystem string
	PURL      string
}

// VersionRange represents a version range for an affected package.
type VersionRange struct {
	Type   string // SEMVER, ECOSYSTEM, GIT
	Events []RangeEvent
}

// RangeEvent is a single introduced/fixed/last_affected event.
type RangeEvent struct {
	Introduced   string
	Fixed        string
	LastAffected string
	Limit        string
}

// Bug is the Go equivalent of Python's Bug(ndb.Model).
// It represents a single vulnerability record with all computed indexes.
type Bug struct {
	// ── Identifiers ──────────────────────────────────────────────────────────
	DBID     string    // e.g. "OSV-2023-xxxx", "GHSA-xxxx", "CVE-xxxx"
	Source   string    // e.g. "ghsa", "cve", "debian", "ossfuzz"
	SourceID string    // original ID in the source

	// ── Status ───────────────────────────────────────────────────────────────
	Status BugStatus // UNPROCESSED, PROCESSED, INVALID, DELETED

	// ── Timestamps ───────────────────────────────────────────────────────────
	Timestamp    time.Time
	LastModified time.Time
	Withdrawn    *time.Time

	// ── Content ──────────────────────────────────────────────────────────────
	Summary  string
	Details  string
	Aliases  []string
	Related  []string

	// ── Package data ─────────────────────────────────────────────────────────
	AffectedPackages []AffectedPackage

	// ── Computed search indexes (pre_put_hook equivalent) ────────────────────
	Project        []string // Package names (for search)
	Ecosystem      []string // Ecosystem names (deduplicated)
	PURL           []string // PURLs for affected packages
	SearchIndices  []string // Tokenized IDs and package names
	AffectedFuzzy  []string // Version strings for fuzzy matching
	SemverFixed    []string // Normalized semver fixed versions

	// ── Computed flags ───────────────────────────────────────────────────────
	HasAffected bool
	IsFixed     bool
	IsPublic    bool

	// ── Severity ─────────────────────────────────────────────────────────────
	CVSSScore    float64
	CVSSSeverity string // CRITICAL, HIGH, MEDIUM, LOW, NONE
	CVSSVector   string
}

// ── ComputeIndexes is the Go equivalent of Python's _pre_put_hook ─────────────

// ComputeIndexes rebuilds all computed index fields from the Bug's content.
// Must be called before saving to any store.
func ComputeIndexes(b *Bug) {
	seenEco := make(map[string]bool)
	seenProject := make(map[string]bool)
	seenPURL := make(map[string]bool)
	seenVersion := make(map[string]bool)
	seenSemver := make(map[string]bool)

	b.HasAffected = len(b.AffectedPackages) > 0
	b.IsFixed = false

	for _, pkg := range b.AffectedPackages {
		// Ecosystems
		if pkg.Ecosystem != "" && !seenEco[pkg.Ecosystem] {
			seenEco[pkg.Ecosystem] = true
			b.Ecosystem = append(b.Ecosystem, pkg.Ecosystem)
		}
		// Package names
		name := pkg.Package.Name
		if name != "" && !seenProject[name] {
			seenProject[name] = true
			b.Project = append(b.Project, name)
		}
		// PURLs
		if pkg.PURL != "" && !seenPURL[pkg.PURL] {
			seenPURL[pkg.PURL] = true
			b.PURL = append(b.PURL, pkg.PURL)
		}
		// Version strings for fuzzy match
		for _, v := range pkg.Versions {
			if !seenVersion[v] {
				seenVersion[v] = true
				b.AffectedFuzzy = append(b.AffectedFuzzy, v)
			}
		}
		// Fixed versions for semver matching
		for _, r := range pkg.Ranges {
			for _, ev := range r.Events {
				if ev.Fixed != "" && !seenSemver[ev.Fixed] {
					seenSemver[ev.Fixed] = true
					b.SemverFixed = append(b.SemverFixed, ev.Fixed)
					b.IsFixed = true
				}
			}
		}
	}

	// SearchIndices: tokenize DBID + aliases + package names
	b.SearchIndices = nil
	for _, tok := range tokenize(b.DBID) {
		b.SearchIndices = append(b.SearchIndices, tok)
	}
	for _, alias := range b.Aliases {
		for _, tok := range tokenize(alias) {
			b.SearchIndices = append(b.SearchIndices, tok)
		}
	}
	for _, name := range b.Project {
		for _, tok := range tokenize(name) {
			b.SearchIndices = append(b.SearchIndices, tok)
		}
	}
	b.SearchIndices = dedup(b.SearchIndices)

	// IsPublic: not withdrawn and status is PROCESSED
	b.IsPublic = b.Withdrawn == nil && b.Status == BugStatusProcessed
}

// ── ConvertToOSVSchema ─────────────────────────────────────────────────────────

// ConvertToOSVSchema converts a Bug to the canonical OSV schema Vulnerability.
// Equivalent to Python's Bug.to_vulnerability().
func ConvertToOSVSchema(b *Bug) *osvschema.Vulnerability {
	vuln := &osvschema.Vulnerability{
		SchemaVersion: "1.5.0",
		Id:            b.DBID,
		Published:     timestamppb.New(b.Timestamp),
		Modified:      timestamppb.New(b.LastModified),
		Summary:       b.Summary,
		Details:       b.Details,
		Aliases:       b.Aliases,
		Related:       b.Related,
	}

	if b.Withdrawn != nil {
		vuln.Withdrawn = timestamppb.New(*b.Withdrawn)
	}

	// Severity
	if b.CVSSVector != "" {
		vuln.Severity = []*osvschema.Severity{
			{
				Type:  osvschema.Severity_CVSS_V3,
				Score: b.CVSSVector,
			},
		}
	}

	// Affected packages
	for _, pkg := range b.AffectedPackages {
		affected := &osvschema.Affected{
			Package: &osvschema.Package{
				Name:      pkg.Package.Name,
				Ecosystem: pkg.Ecosystem,
				Purl:      pkg.PURL,
			},
			Versions: pkg.Versions,
		}
		// Convert ranges
		for _, r := range pkg.Ranges {
			osvsRange := &osvschema.Range{
				Type: osvschema.Range_Type(osvschema.Range_Type_value[r.Type]),
			}
			for _, ev := range r.Events {
				event := &osvschema.Event{}
				switch {
				case ev.Introduced != "":
					event.Introduced = ev.Introduced
				case ev.Fixed != "":
					event.Fixed = ev.Fixed
				case ev.LastAffected != "":
					event.LastAffected = ev.LastAffected
				case ev.Limit != "":
					event.Limit = ev.Limit
				}
				osvsRange.Events = append(osvsRange.Events, event)
			}
			affected.Ranges = append(affected.Ranges, osvsRange)
		}
		vuln.Affected = append(vuln.Affected, affected)
	}

	return vuln
}

// ── UpdateFromOSVSchema ────────────────────────────────────────────────────────

// UpdateFromOSVSchema updates a Bug from an OSV schema Vulnerability.
// Equivalent to Python's Bug.update_from_vulnerability().
func UpdateFromOSVSchema(b *Bug, vuln *osvschema.Vulnerability) {
	b.DBID = vuln.Id
	b.Summary = vuln.Summary
	b.Details = vuln.Details
	b.Aliases = vuln.Aliases
	b.Related = vuln.Related

	if vuln.Modified != nil {
		b.LastModified = vuln.Modified.AsTime()
	}
	if vuln.Published != nil && !vuln.Published.AsTime().IsZero() {
		b.Timestamp = vuln.Published.AsTime()
	}
	if vuln.Withdrawn != nil {
		t := vuln.Withdrawn.AsTime()
		b.Withdrawn = &t
	}

	// Extract severity
	for _, sev := range vuln.Severity {
		if sev.Type == osvschema.Severity_CVSS_V3 || sev.Type == osvschema.Severity_CVSS_V4 {
			b.CVSSVector = sev.Score
		}
	}

	// Extract affected packages
	b.AffectedPackages = nil
	for _, aff := range vuln.Affected {
		pkg := AffectedPackage{
			Versions: aff.Versions,
		}
		if aff.Package != nil {
			pkg.Package = PackageRef{
				Name:      aff.Package.Name,
				Ecosystem: aff.Package.Ecosystem,
				PURL:      aff.Package.Purl,
			}
			pkg.Ecosystem = aff.Package.Ecosystem
			pkg.PURL = aff.Package.Purl
		}
		for _, r := range aff.Ranges {
			vr := VersionRange{Type: r.Type.String()}
			for _, ev := range r.Events {
				vr.Events = append(vr.Events, RangeEvent{
					Introduced:   ev.Introduced,
					Fixed:        ev.Fixed,
					LastAffected: ev.LastAffected,
					Limit:        ev.Limit,
				})
			}
			pkg.Ranges = append(pkg.Ranges, vr)
		}
		b.AffectedPackages = append(b.AffectedPackages, pkg)
	}

	// Recompute indexes after update
	ComputeIndexes(b)
}

// ── tokenize helper ────────────────────────────────────────────────────────────

// tokenize splits a string into searchable tokens.
// Equivalent to Python's Bug.tokenize() logic.
func tokenize(s string) []string {
	s = strings.ToLower(s)
	var tokens []string
	current := strings.Builder{}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// dedup removes duplicate strings from a slice preserving order.
func dedup(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
