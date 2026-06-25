# SA-SHARED-01 — Shared Event Types & NATS Stream Config

## Metadata
- **Task ID**: SA-SHARED-01
- **Sprint**: A (P0 — Foundation)
- **Ước tính**: 1 giờ
- **Dependencies**: Không có
- **Spec nguồn**: `specs/solutions/enhance-cli-app/06_nats-integration.md` § "2. JetStream Streams Configuration"

---

## Context

```bash
# Kiểm tra shared package hiện có
ls services/shared/
find services/shared -name "*.go" | head -20

# Xem existing NATS types nếu có
ls services/shared/pkg/ 2>/dev/null || echo "no pkg dir"
```

---

## Goal

Tạo shared event type definitions và NATS JetStream stream config để tất cả services và apps/cli đều dùng chung.

---

## Files to Create

### File 1: `services/shared/pkg/events/types.go`

```go
// Package events defines the canonical NATS event types and subject constants
// for the OSV platform. All services and CLI tools must use these definitions
// instead of defining their own ad-hoc event structures.
package events

import "time"

// ── Subject Constants (JetStream subjects) ──────────────────────────────────

const (
	// SubjectVulnImported is published by cli/importer when a new vulnerability
	// is imported from an external source (NVD, GHSA, OSS-Fuzz, etc.)
	SubjectVulnImported = "osv.vuln.imported"

	// SubjectVulnUpdated is published by data-service when a vulnerability record
	// changes (description, severity, affected versions, etc.)
	SubjectVulnUpdated = "osv.vuln.updated"

	// SubjectVulnWithdrawn is published by data-service when a vulnerability is retracted.
	SubjectVulnWithdrawn = "osv.vuln.withdrawn"

	// SubjectAIEnrichmentCompleted is published by ai-service after enrichment.
	// search-service subscribes to reindex vector embeddings.
	SubjectAIEnrichmentCompleted = "osv.ai.enrichment.completed"

	// SubjectScanCompleted is published by scan-service when a scan job finishes.
	// finding-service subscribes to create findings.
	SubjectScanCompleted = "osv.scan.completed"

	// SubjectFindingCreated is published by finding-service when a new finding is created.
	// notification-service subscribes to match alert rules.
	SubjectFindingCreated = "defectdojo.finding.created"

	// SubjectFindingStatusChanged is published when a finding state changes.
	SubjectFindingStatusChanged = "defectdojo.finding.status_changed"

	// SubjectSLABreached is published when a finding's SLA deadline is exceeded.
	SubjectSLABreached = "finding.sla.breached"

	// SubjectRiskAccepted is published when a finding is risk-accepted.
	SubjectRiskAccepted = "finding.risk.accepted"
)

// ── Event Payloads ───────────────────────────────────────────────────────────

// VulnImportedEvent is published to SubjectVulnImported.
// Published by: apps/cli/cmd/importer (NATSPublisher)
// Consumed by: data-service (CVEImportConsumer)
type VulnImportedEvent struct {
	// ID is the OSV/CVE identifier (e.g., "CVE-2021-44228", "GHSA-xxxx-xxxx-xxxx")
	ID string `json:"id"`
	// Source identifies the upstream feed (e.g., "nvd", "ghsa", "ossfuzz", "debian")
	Source string `json:"source"`
	// ImportedAt is the UTC timestamp when the import was triggered.
	ImportedAt time.Time `json:"imported_at"`
	// OSVData is the JSON-encoded OSV schema Vulnerability record.
	OSVData []byte `json:"osv_data"`
	// TraceID for distributed tracing correlation.
	TraceID string `json:"trace_id,omitempty"`
}

// VulnUpdatedEvent is published to SubjectVulnUpdated.
// Published by: data-service after upsert
// Consumed by: search-service (reindex), ai-service (re-enrich if needed)
type VulnUpdatedEvent struct {
	CVEID              string    `json:"cve_id"`
	UpdatedAt          time.Time `json:"updated_at"`
	DescriptionChanged bool      `json:"description_changed"`
	SeverityChanged    bool      `json:"severity_changed"`
	AffectedChanged    bool      `json:"affected_changed"`
}

// VulnWithdrawnEvent is published to SubjectVulnWithdrawn.
// Consumed by: search-service (remove from index)
type VulnWithdrawnEvent struct {
	CVEID      string    `json:"cve_id"`
	WithdrawnAt time.Time `json:"withdrawn_at"`
	Reason     string    `json:"reason,omitempty"`
}

// AIEnrichmentCompletedEvent is published to SubjectAIEnrichmentCompleted.
// Published by: ai-service after EnrichCVE completes
// Consumed by: search-service (update vector index)
type AIEnrichmentCompletedEvent struct {
	CVEID       string    `json:"cve_id"`
	Provider    string    `json:"provider"` // "vertex", "openai", "ollama"
	CompletedAt time.Time `json:"completed_at"`
	// HasVector indicates a new vector embedding was generated.
	HasVector bool `json:"has_vector"`
	// EPSSScore is the EPSS probability score (0.0–1.0), if updated.
	EPSSScore float64 `json:"epss_score,omitempty"`
}

// ScanCompletedEvent is published to SubjectScanCompleted.
// Published by: scan-service after a Trivy scan finishes
// Consumed by: finding-service (create findings from scan result)
type ScanCompletedEvent struct {
	JobID      string    `json:"job_id"`
	Target     string    `json:"target"`     // image name or directory path
	ScanType   string    `json:"scan_type"`  // "image", "dir", "sbom"
	CompletedAt time.Time `json:"completed_at"`
	// SBOMData is the CycloneDX JSON SBOM, if generated.
	SBOMData []byte `json:"sbom_data,omitempty"`
	// VulnCount is the number of vulnerabilities found.
	VulnCount int `json:"vuln_count"`
	// ResultURL points to the full scan result (if stored in object storage).
	ResultURL string `json:"result_url,omitempty"`
}

// FindingCreatedEvent is published to SubjectFindingCreated.
// Consumed by: notification-service (match alert rules)
type FindingCreatedEvent struct {
	FindingID  string    `json:"finding_id"`
	Title      string    `json:"title"`
	Severity   string    `json:"severity"` // "critical", "high", "medium", "low"
	CVEID      string    `json:"cve_id,omitempty"`
	AssetID    string    `json:"asset_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// SLABreachedEvent is published to SubjectSLABreached.
// Consumed by: notification-service (send urgent alert)
type SLABreachedEvent struct {
	FindingID   string    `json:"finding_id"`
	Severity    string    `json:"severity"`
	DeadlineAt  time.Time `json:"deadline_at"`
	BreachedAt  time.Time `json:"breached_at"`
	DaysOverdue int       `json:"days_overdue"`
}
```

### File 2: `services/shared/pkg/nats/streams.go`

```go
// Package nats provides shared NATS JetStream utilities for the OSV platform.
// All services use this package to set up streams consistently.
package nats

import (
	"errors"
	"fmt"
	"time"

	natsclient "github.com/nats-io/nats.go"
)

// StreamConfig is our internal config type (wraps nats.StreamConfig for clarity).
type StreamDefinition struct {
	Name     string
	Subjects []string
	MaxAge   time.Duration
}

// AllStreams defines all JetStream streams for the OSV platform.
// Services call SetupStreams on startup to ensure streams exist.
var AllStreams = []StreamDefinition{
	{
		Name:     "OSV_VULN",
		Subjects: []string{"osv.vuln.*"},
		MaxAge:   7 * 24 * time.Hour, // 7 days
	},
	{
		Name:     "OSV_AI",
		Subjects: []string{"osv.ai.*"},
		MaxAge:   24 * time.Hour,
	},
	{
		Name:     "OSV_SCAN",
		Subjects: []string{"osv.scan.*"},
		MaxAge:   72 * time.Hour,
	},
	{
		Name:     "DEFECTDOJO",
		Subjects: []string{"defectdojo.*", "finding.*"},
		MaxAge:   30 * 24 * time.Hour, // 30 days
	},
}

// SetupStreams creates all platform streams if they don't already exist.
// Safe to call multiple times (idempotent).
func SetupStreams(js natsclient.JetStreamContext) error {
	for _, def := range AllStreams {
		cfg := &natsclient.StreamConfig{
			Name:      def.Name,
			Subjects:  def.Subjects,
			Storage:   natsclient.FileStorage,
			Retention: natsclient.LimitsPolicy,
			MaxAge:    def.MaxAge,
			Replicas:  1,
		}
		_, err := js.AddStream(cfg)
		if err != nil && !errors.Is(err, natsclient.ErrStreamNameAlreadyInUse) {
			return fmt.Errorf("setup stream %s: %w", def.Name, err)
		}
	}
	return nil
}
```

---

## Acceptance Criteria

- [ ] `services/shared/pkg/events/types.go` tồn tại với tất cả event structs và subject constants
- [ ] `services/shared/pkg/nats/streams.go` tồn tại với `SetupStreams()` function
- [ ] `go build ./...` từ `services/shared/` phải PASS
- [ ] Không xóa hay sửa bất kỳ file nào hiện có trong `services/shared/`

---

## Verification

```bash
cd services/shared
go build ./...
# Output: no errors
```

---

## ✅ Execution Status: COMPLETED ✅

**Completed**: 2026-06-13

### Files Created
- `services/shared/pkg/events/types.go` — 9 event types + 9 subject constants
- `services/shared/pkg/nats/osv_streams.go` — `SetupOSVStreams()` + `SetupAllStreams()` (additive to setup.go)

### Build Verification
```
go build ./events/...  → OK
go build ./nats/...    → OK
```
