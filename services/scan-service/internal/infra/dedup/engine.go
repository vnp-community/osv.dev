// Package dedupinfra implements the deduplication engine for the scan import pipeline.
// It supports three algorithms: hash_code (SHA-256), unique_id_from_tool, and legacy.
package dedupinfra

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"sort"
	"strings"

	importuc "github.com/osv/scan-service/internal/usecase/import"
)

// AlgorithmPerScanner maps scan types to their preferred dedup algorithm.
// Mirrors Django DefectDojo DEDUPLICATION_ALGORITHM_PER_PARSER config.
var AlgorithmPerScanner = map[string]string{
	"Snyk Scan":                           "unique_id_from_tool",
	"SonarQube Scan detailed":             "unique_id_from_tool",
	"Dependency Check Scan":               "unique_id_from_tool",
	"Nuclei Scan":                         "unique_id_from_tool",
	"Semgrep JSON Report":                 "unique_id_from_tool",
	"GitLab SAST Report":                  "unique_id_from_tool",
	"GitLab Dependency Scanning Report":   "unique_id_from_tool",
	// All others → hash_code (default)
}

// FindingLookupClient provides read access to existing findings for dedup comparison.
type FindingLookupClient interface {
	// FindByHashCode returns existing finding IDs matching the hash code.
	// If engagementID is non-empty, scope is limited to that engagement.
	FindByHashCode(ctx context.Context, hashCode, productID string, engagementID string) ([]string, error)
	// FindByUniqueID returns the finding ID matching the vuln_id_from_tool.
	FindByUniqueID(ctx context.Context, uniqueID, productID string) (string, bool, error)
	// ExistsFalsePositiveByHash returns true if a false-positive finding with this hash exists.
	ExistsFalsePositiveByHash(ctx context.Context, hashCode, productID string) (bool, error)
	// GetFindingStatus returns the current lifecycle status of a finding.
	GetFindingStatus(ctx context.Context, findingID string) (active bool, mitigated bool, err error)
	// ReactivateFinding marks a mitigated finding as active again.
	ReactivateFinding(ctx context.Context, findingIDs []string) error
}

// Engine implements the three-algorithm deduplication strategy.
type Engine struct {
	client FindingLookupClient
}

// NewEngine creates a new deduplication Engine.
func NewEngine(client FindingLookupClient) *Engine {
	return &Engine{client: client}
}

// Deduplicate classifies each parsed finding into: new, duplicate, reactivated, untouched.
// The result.NewFindings contains only the findings that need to be created.
// The result.Reactivated contains finding IDs that need to be reactivated via gRPC.
func (e *Engine) Deduplicate(ctx context.Context, findings []*importuc.ParsedFinding, dc *importuc.DedupContext) (*importuc.DedupResult, error) {
	result := &importuc.DedupResult{}

	// Determine algorithm for this scan type
	algorithm := dc.OnEngagement // reuse field for now
	_ = algorithm

	toReactivate := make([]string, 0)

	for _, f := range findings {
		// Always compute hash code for use in CloseOldFindings step
		if f.HashCode == "" {
			f.HashCode = ComputeHashCode(f)
		}

		// Determine which algorithm to use for THIS finding's scan type
		algo := AlgorithmPerScanner[dc.ScanType]
		if algo == "" {
			algo = "hash_code"
		}

		var existingIDs []string

		switch algo {
		case "unique_id_from_tool":
			if f.VulnIDFromTool != "" {
				id, found, err := e.client.FindByUniqueID(ctx, f.VulnIDFromTool, dc.ProductID)
				if err == nil && found {
					existingIDs = []string{id}
				}
			}
			// Fallback to hash if no unique ID match
			if len(existingIDs) == 0 {
				var scope string
				if dc.OnEngagement {
					scope = dc.EngagementID
				}
				existingIDs, _ = e.client.FindByHashCode(ctx, f.HashCode, dc.ProductID, scope)
			}

		default: // hash_code
			var scope string
			if dc.OnEngagement {
				scope = dc.EngagementID
			}
			existingIDs, _ = e.client.FindByHashCode(ctx, f.HashCode, dc.ProductID, scope)
		}

		if len(existingIDs) == 0 {
			// No existing finding — check FP history
			if isFP, _ := e.client.ExistsFalsePositiveByHash(ctx, f.HashCode, dc.ProductID); isFP {
				f.FalsePositive = true
			}
			result.NewFindings = append(result.NewFindings, f)
			continue
		}

		// Found existing — classify by status
		for _, eid := range existingIDs {
			active, mitigated, err := e.client.GetFindingStatus(ctx, eid)
			if err != nil {
				continue
			}
			if active {
				// Already active → untouched
				result.Untouched = append(result.Untouched, eid)
			} else if mitigated {
				// Was closed → reactivate
				toReactivate = append(toReactivate, eid)
				result.Reactivated = append(result.Reactivated, eid)
			} else {
				// FP/OOS/RA → mark as duplicate
				result.Duplicates = append(result.Duplicates, f)
			}
		}
	}

	// Batch-reactivate in one gRPC call
	if len(toReactivate) > 0 {
		_ = e.client.ReactivateFinding(ctx, toReactivate)
	}

	return result, nil
}

// ComputeHashCode generates a SHA-256 fingerprint for a parsed finding.
// Algorithm mirrors Django DefectDojo dojo/utils.py::get_hash_code():
// SHA256(severity | title | cwe? | description[:256] | sorted(endpoints))
func ComputeHashCode(f *importuc.ParsedFinding) string {
	h := sha256.New()
	parts := []string{
		strings.ToLower(f.Severity),
		strings.ToLower(f.Title),
	}
	if f.CWE != 0 {
		parts = append(parts, fmt.Sprintf("%d", f.CWE))
	}
	if f.Description != "" {
		desc := f.Description
		if len(desc) > 256 {
			desc = desc[:256]
		}
		parts = append(parts, desc)
	}

	// Sort endpoints for deterministic hash regardless of discovery order
	if len(f.Endpoints) > 0 {
		eps := make([]string, len(f.Endpoints))
		copy(eps, f.Endpoints)
		sort.Strings(eps)
		parts = append(parts, eps...)
	}

	io.WriteString(h, strings.Join(parts, "|")) //nolint:errcheck
	return fmt.Sprintf("%x", h.Sum(nil))
}
