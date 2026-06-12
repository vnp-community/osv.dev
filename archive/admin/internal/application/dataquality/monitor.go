// Package dataquality implements data quality monitoring for imported CVE records.
// TASK-06-04: Monitor import quality, flag anomalies, surface in admin API.
package dataquality

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Severity of a data quality issue.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// CheckType identifies the type of quality check.
type CheckType string

const (
	CheckMissingCVSS        CheckType = "missing_cvss"
	CheckMissingDescription CheckType = "missing_description"
	CheckMissingAffected    CheckType = "missing_affected"
	CheckInvalidCPE         CheckType = "invalid_cpe"
	CheckDuplicateCVE       CheckType = "duplicate_cve"
	CheckStaleCVE           CheckType = "stale_cve"      // last modified > 1 year
	CheckOrphanedAlias      CheckType = "orphaned_alias" // alias points to non-existent CVE
	CheckMissingCWE         CheckType = "missing_cwe"
)

// Finding represents a single data quality issue found in a CVE record.
type Finding struct {
	ID         string    `json:"id"`
	CVEID      string    `json:"cve_id"`
	SourceID   string    `json:"source_id"`
	CheckType  CheckType `json:"check_type"`
	Severity   Severity  `json:"severity"`
	Message    string    `json:"message"`
	FoundAt    time.Time `json:"found_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	Resolved   bool      `json:"resolved"`
}

// CVERecord is the minimal data needed for quality checking.
type CVERecord struct {
	ID          string
	SourceID    string
	HasCVSS     bool
	Description string
	AffectedCount int
	CPEs        []string
	CWEIDs      []string
	LastModified time.Time
	Aliases     []string
}

// Monitor runs quality checks and stores findings.
type Monitor struct {
	mu       sync.RWMutex
	findings map[string]Finding // finding ID → Finding
	seq      int
	checks   []CheckFunc
}

// CheckFunc is a function that checks a CVE record for quality issues.
type CheckFunc func(record CVERecord) []Finding

// NewMonitor creates a quality monitor with the default check set.
func NewMonitor() *Monitor {
	m := &Monitor{
		findings: make(map[string]Finding),
	}
	m.checks = []CheckFunc{
		m.checkMissingCVSS,
		m.checkMissingDescription,
		m.checkMissingAffected,
		m.checkStaleCVE,
		m.checkMissingCWE,
	}
	return m
}

// Check runs all quality checks on a CVE record and stores any findings.
func (m *Monitor) Check(_ context.Context, record CVERecord) []Finding {
	var newFindings []Finding
	for _, check := range m.checks {
		findings := check(record)
		newFindings = append(newFindings, findings...)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for i, f := range newFindings {
		m.seq++
		f.ID = fmt.Sprintf("dq-%s-%06d", f.CheckType, m.seq)
		f.FoundAt = time.Now().UTC()
		m.findings[f.ID] = f
		newFindings[i] = f
	}

	return newFindings
}

// ListFindings returns all unresolved (or all) findings, optionally filtered by source.
func (m *Monitor) ListFindings(_ context.Context, sourceID string, includeResolved bool) []Finding {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []Finding
	for _, f := range m.findings {
		if !includeResolved && f.Resolved {
			continue
		}
		if sourceID != "" && f.SourceID != sourceID {
			continue
		}
		results = append(results, f)
	}
	return results
}

// ResolveFinding marks a finding as resolved.
func (m *Monitor) ResolveFinding(_ context.Context, findingID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.findings[findingID]
	if !ok {
		return fmt.Errorf("finding %q not found", findingID)
	}
	if f.Resolved {
		return fmt.Errorf("finding %q already resolved", findingID)
	}
	now := time.Now().UTC()
	f.Resolved = true
	f.ResolvedAt = &now
	m.findings[findingID] = f
	return nil
}

// Summary returns a statistical summary of quality findings.
func (m *Monitor) Summary(_ context.Context) QualitySummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := QualitySummary{
		ByCheck:    make(map[CheckType]int),
		BySeverity: make(map[Severity]int),
	}
	for _, f := range m.findings {
		if f.Resolved {
			summary.Resolved++
			continue
		}
		summary.Open++
		summary.ByCheck[f.CheckType]++
		summary.BySeverity[f.Severity]++
		if f.Severity == SeverityCritical {
			summary.Critical++
		}
	}
	return summary
}

// QualitySummary holds aggregate quality metrics.
type QualitySummary struct {
	Open       int
	Resolved   int
	Critical   int
	ByCheck    map[CheckType]int
	BySeverity map[Severity]int
}

// ---- Built-in checks ----

func (m *Monitor) checkMissingCVSS(r CVERecord) []Finding {
	if r.HasCVSS {
		return nil
	}
	return []Finding{{
		CVEID:     r.ID,
		SourceID:  r.SourceID,
		CheckType: CheckMissingCVSS,
		Severity:  SeverityWarning,
		Message:   fmt.Sprintf("CVE %s has no CVSS score", r.ID),
	}}
}

func (m *Monitor) checkMissingDescription(r CVERecord) []Finding {
	if len(r.Description) >= 20 {
		return nil
	}
	return []Finding{{
		CVEID:     r.ID,
		SourceID:  r.SourceID,
		CheckType: CheckMissingDescription,
		Severity:  SeverityWarning,
		Message:   fmt.Sprintf("CVE %s has missing or very short description (%d chars)", r.ID, len(r.Description)),
	}}
}

func (m *Monitor) checkMissingAffected(r CVERecord) []Finding {
	if r.AffectedCount > 0 {
		return nil
	}
	return []Finding{{
		CVEID:     r.ID,
		SourceID:  r.SourceID,
		CheckType: CheckMissingAffected,
		Severity:  SeverityCritical,
		Message:   fmt.Sprintf("CVE %s has no affected package/version information", r.ID),
	}}
}

func (m *Monitor) checkStaleCVE(r CVERecord) []Finding {
	if r.LastModified.IsZero() {
		return nil
	}
	if time.Since(r.LastModified) < 365*24*time.Hour {
		return nil
	}
	return []Finding{{
		CVEID:     r.ID,
		SourceID:  r.SourceID,
		CheckType: CheckStaleCVE,
		Severity:  SeverityInfo,
		Message: fmt.Sprintf("CVE %s has not been updated in > 1 year (last: %s)",
			r.ID, r.LastModified.Format("2006-01-02")),
	}}
}

func (m *Monitor) checkMissingCWE(r CVERecord) []Finding {
	if len(r.CWEIDs) > 0 {
		return nil
	}
	return []Finding{{
		CVEID:     r.ID,
		SourceID:  r.SourceID,
		CheckType: CheckMissingCWE,
		Severity:  SeverityInfo,
		Message:   fmt.Sprintf("CVE %s has no CWE classification", r.ID),
	}}
}
