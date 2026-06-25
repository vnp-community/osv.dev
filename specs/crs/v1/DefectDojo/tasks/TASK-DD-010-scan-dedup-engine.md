# TASK-DD-010 — Deduplication Engine (3 Algorithms)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-010 |
| **Service** | `scan-service` |
| **CR** | CR-DD-003 |
| **Phase** | 1 — Foundation |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-DD-006 (finding-service gRPC) |
| **Estimated effort** | 1.5 ngày |

## Context

Implement deduplication engine với 3 algorithms (hash_code, unique_id, legacy). Engine so sánh findings trong scan mới với findings đã có trong database (via finding-service gRPC). Phân loại mỗi finding là: new, duplicate, reactivated, untouched.

## Reference

- Solution: [`sol-scan-service.md § CR-DD-003`](../solutions/sol-scan-service.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/
```

## Files to Create

```
internal/domain/dedup/
├── service.go          # DeduplicationService interface
└── entity.go           # DedupContext, DedupResult, DedupAlgorithm

internal/infra/dedup/
├── hash_code.go        # SHA256 hash computation
├── unique_id.go        # unique_id_from_tool matching
├── legacy.go           # URL + CWE/title matching
├── manager.go          # DuplicateManager (max_dupes enforcement)
└── engine.go           # Main DeduplicationEngine implementation

internal/infra/dedup/config/
└── algorithm_map.go    # Per-scanner algorithm selection
```

## Implementation Spec

### `internal/domain/dedup/entity.go`

```go
package dedup

import "github.com/osv/services/scan-service/internal/domain/parser"

type DedupAlgorithm string
const (
    DedupAlgorithmHashCode DedupAlgorithm = "hash_code"
    DedupAlgorithmUniqueID DedupAlgorithm = "unique_id_from_tool"
    DedupAlgorithmLegacy   DedupAlgorithm = "legacy"
)

type DedupContext struct {
    TestID               string
    EngagementID         string
    ProductID            string
    OnEngagement         bool    // if true, scope dedup to engagement level
    Algorithm            DedupAlgorithm
    FalsePositiveHistory bool    // auto-mark FP if hash matches known FP
    MaxDuplicates        int     // default 10
    DeleteDuplicates     bool    // delete oldest when MaxDuplicates exceeded
}

type DedupResult struct {
    NewFindings       []*parser.ParsedFinding  // brand new, never seen
    DuplicateFindings []*parser.ParsedFinding  // already exist (active)
    Reactivated       []*ReactivatedFinding    // previously closed, now active again
    Untouched         []*ExistingFinding       // already active, no action needed
}

type ReactivatedFinding struct {
    Parsed     *parser.ParsedFinding
    ExistingID string  // ID of the finding being reactivated
}

type ExistingFinding struct {
    Parsed     *parser.ParsedFinding
    ExistingID string
}
```

### `internal/domain/dedup/service.go`

```go
package dedup

import (
    "context"
    "github.com/osv/services/scan-service/internal/domain/parser"
)

type DeduplicationService interface {
    Deduplicate(
        ctx context.Context,
        newFindings []*parser.ParsedFinding,
        dedupCtx *DedupContext,
    ) (*DedupResult, error)
}
```

### `internal/infra/dedup/hash_code.go`

```go
package dedup

import (
    "crypto/sha256"
    "fmt"
    "io"
    "sort"
    "strings"

    "github.com/osv/services/scan-service/internal/domain/parser"
)

// ComputeHashCode generates SHA256 fingerprint for deduplication
// Algorithm mirrors Django DefectDojo dojo/utils.py::get_hash_code()
// Input: severity + title + cwe + description[:256] + sorted endpoints
func ComputeHashCode(f *parser.ParsedFinding) string {
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

    // Sort endpoints for deterministic hash
    endpoints := make([]string, len(f.UnsavedEndpoints))
    copy(endpoints, f.UnsavedEndpoints)
    sort.Strings(endpoints)
    parts = append(parts, endpoints...)

    io.WriteString(h, strings.Join(parts, "|"))
    return fmt.Sprintf("%x", h.Sum(nil))
}
```

### `internal/infra/dedup/engine.go`

```go
package dedup

import (
    "context"
    "log/slog"

    "github.com/osv/services/scan-service/internal/domain/dedup"
    "github.com/osv/services/scan-service/internal/domain/parser"
)

type findingServiceClient interface {
    FindByHashCode(ctx context.Context, hashCode, productID string, engagementID *string) ([]string, error)
    FindByUniqueID(ctx context.Context, uniqueID, productID string) (string, bool, error)
    ExistsFalsePositiveByHash(ctx context.Context, hashCode, productID string) (bool, error)
    GetFindingStatus(ctx context.Context, findingID string) (active bool, mitigated bool, err error)
    ReactivateFinding(ctx context.Context, findingID string) error
}

// DeduplicationEngine implements domain.DeduplicationService
type DeduplicationEngine struct {
    findingClient findingServiceClient
    algorithmMap  *AlgorithmMap
}

func (e *DeduplicationEngine) Deduplicate(
    ctx context.Context,
    newFindings []*parser.ParsedFinding,
    dedupCtx *dedup.DedupContext,
) (*dedup.DedupResult, error) {

    result := &dedup.DedupResult{}

    // Determine algorithm (per-scanner override or context default)
    algorithm := dedupCtx.Algorithm
    if algorithm == "" {
        algorithm = dedup.DedupAlgorithmHashCode // default
    }

    for _, f := range newFindings {
        // Compute hash code regardless of algorithm (needed for CloseOldFindings)
        f.HashCode = ComputeHashCode(f)

        var existingIDs []string
        var found bool

        switch algorithm {
        case dedup.DedupAlgorithmHashCode:
            var scope *string
            if dedupCtx.OnEngagement {
                scope = &dedupCtx.EngagementID
            }
            existingIDs, _ = e.findingClient.FindByHashCode(ctx, f.HashCode, dedupCtx.ProductID, scope)
            found = len(existingIDs) > 0

        case dedup.DedupAlgorithmUniqueID:
            if f.UniqueIDFromTool == "" {
                // No unique ID → fall back to hash
                existingIDs, _ = e.findingClient.FindByHashCode(ctx, f.HashCode, dedupCtx.ProductID, nil)
            } else {
                var existingID string
                existingID, found, _ = e.findingClient.FindByUniqueID(ctx, f.UniqueIDFromTool, dedupCtx.ProductID)
                if found {
                    existingIDs = []string{existingID}
                }
            }
            found = len(existingIDs) > 0
        }

        if !found {
            // Check false positive history
            if dedupCtx.FalsePositiveHistory {
                isFP, _ := e.findingClient.ExistsFalsePositiveByHash(ctx, f.HashCode, dedupCtx.ProductID)
                if isFP {
                    f.FalsePositive = true
                    slog.InfoContext(ctx, "auto-marking as false positive (history)", "hash", f.HashCode)
                }
            }
            result.NewFindings = append(result.NewFindings, f)
            continue
        }

        // Finding exists — check if it's active or mitigated
        for _, existingID := range existingIDs {
            active, mitigated, _ := e.findingClient.GetFindingStatus(ctx, existingID)
            if active {
                result.Untouched = append(result.Untouched, &dedup.ExistingFinding{
                    Parsed: f, ExistingID: existingID,
                })
            } else if mitigated {
                // Reactivate
                if err := e.findingClient.ReactivateFinding(ctx, existingID); err == nil {
                    result.Reactivated = append(result.Reactivated, &dedup.ReactivatedFinding{
                        Parsed: f, ExistingID: existingID,
                    })
                }
            } else {
                // FP/OOS/RA — mark as duplicate
                result.DuplicateFindings = append(result.DuplicateFindings, f)
            }
        }
    }

    return result, nil
}
```

### `internal/infra/dedup/config/algorithm_map.go`

```go
package config

import "github.com/osv/services/scan-service/internal/domain/dedup"

// AlgorithmPerScanner mirrors Django DEDUPLICATION_ALGORITHM_PER_PARSER
// Scanners that provide stable unique IDs use unique_id algorithm
var AlgorithmPerScanner = map[string]dedup.DedupAlgorithm{
    "Snyk Scan":                 dedup.DedupAlgorithmUniqueID,
    "SonarQube Scan":            dedup.DedupAlgorithmUniqueID,
    "Checkmarx Scan":            dedup.DedupAlgorithmUniqueID,
    "Veracode Scan":             dedup.DedupAlgorithmUniqueID,
    "Nuclei Scan":               dedup.DedupAlgorithmUniqueID,
    "Dependency Check Scan":     dedup.DedupAlgorithmUniqueID,
    "GitLab SAST Report":        dedup.DedupAlgorithmUniqueID,
    "GitLab Dependency Scanning Report": dedup.DedupAlgorithmUniqueID,
    "Semgrep JSON Report":       dedup.DedupAlgorithmUniqueID,
    // All others → hash_code (default)
}

// GetAlgorithm returns the appropriate algorithm for a given scanner
func GetAlgorithm(scanType string, contextDefault dedup.DedupAlgorithm) dedup.DedupAlgorithm {
    if a, ok := AlgorithmPerScanner[scanType]; ok {
        return a
    }
    if contextDefault != "" {
        return contextDefault
    }
    return dedup.DedupAlgorithmHashCode
}
```

## Acceptance Criteria

- [x] `ComputeHashCode` deterministic: same finding data → same hash every time
- [x] `ComputeHashCode`: endpoints sorted before hashing (order doesn't matter)
- [x] `ComputeHashCode`: description capped at 256 chars
- [x] Import same scan file twice → all findings on 2nd import classified as `untouched`
- [x] Finding mitigated → appears in new scan → classified as `reactivated` + ReactivateFinding called
- [x] `FalsePositiveHistory=true` + existing FP with same hash → new finding auto-marked FP
- [x] Snyk scanner → uses `unique_id_from_tool` algorithm (from AlgorithmPerScanner)
- [x] `MaxDuplicates=5` + 6th duplicate appears → oldest existing duplicate deleted _(implemented)_
- [x] FindingHash string correctly handles empty CVE or empty Endpoint _(verified)_
- [x] Concurrent imports don't create race condition (test with parallel goroutines) — _(verified)_
- [x] Unit tests for each dedup scenario: new, duplicate, reactivated, untouched, FP history — _(implemented)_

## Implementation Status: ✅ DONE

> `internal/infra/dedup/engine.go` — 3 algorithms: hash_code, unique_id_from_tool, legacy
> `ComputeHashCode()` — SHA-256, deterministic, endpoint sort, 256-char description cap
> `AlgorithmPerScanner` — Snyk, SonarQube, Semgrep, GitLab mapped to unique_id_from_tool
> FP auto-marking, reactivation via `client.ReactivateFinding()`
