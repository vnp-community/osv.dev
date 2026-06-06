// Package service contains domain services for the report service.
package service

import (
	"sort"

	"github.com/osv/report-service/internal/domain/entity"
)

// severityOrder maps severity strings to numeric rank for comparison.
var severityOrder = map[string]int{
	"UNKNOWN":  0,
	"NONE":     0,
	"LOW":      1,
	"MEDIUM":   2,
	"HIGH":     3,
	"CRITICAL": 4,
}

// severityRank returns the numeric rank of a severity string (case-insensitive).
func severityRank(s string) int {
	if r, ok := severityOrder[s]; ok {
		return r
	}
	return 0
}

// FilterBySeverity keeps only CVEs with severity >= minSeverity.
// Severity order: UNKNOWN < LOW < MEDIUM < HIGH < CRITICAL
func FilterBySeverity(cves []entity.CVEData, minSeverity string) []entity.CVEData {
	if minSeverity == "" || minSeverity == "UNKNOWN" {
		return cves
	}
	minRank := severityRank(minSeverity)
	out := make([]entity.CVEData, 0, len(cves))
	for _, c := range cves {
		if severityRank(c.Severity) >= minRank {
			out = append(out, c)
		}
	}
	return out
}

// FilterByScore keeps only CVEs with Score >= minScore.
// If minScore <= 0, all CVEs are kept.
func FilterByScore(cves []entity.CVEData, minScore float64) []entity.CVEData {
	if minScore <= 0 {
		return cves
	}
	out := make([]entity.CVEData, 0, len(cves))
	for _, c := range cves {
		if c.Score >= minScore {
			out = append(out, c)
		}
	}
	return out
}

// SortBySeverity sorts CVEs by severity descending (CRITICAL first),
// then by score descending as secondary sort.
func SortBySeverity(cves []entity.CVEData) []entity.CVEData {
	sorted := make([]entity.CVEData, len(cves))
	copy(sorted, cves)
	sort.SliceStable(sorted, func(i, j int) bool {
		ri := severityRank(sorted[i].Severity)
		rj := severityRank(sorted[j].Severity)
		if ri != rj {
			return ri > rj // higher severity first
		}
		return sorted[i].Score > sorted[j].Score // higher score first
	})
	return sorted
}

// FilterAndSort applies severity + score filters and sorts CRITICAL-first.
func FilterAndSort(cves []entity.CVEData, minSeverity string, minScore float64) []entity.CVEData {
	filtered := FilterBySeverity(cves, minSeverity)
	filtered = FilterByScore(filtered, minScore)
	return SortBySeverity(filtered)
}

// HasCVEs returns true if any product in the map has at least one CVE.
func HasCVEs(data map[entity.ProductInfo][]entity.CVEData) bool {
	for _, cves := range data {
		if len(cves) > 0 {
			return true
		}
	}
	return false
}
