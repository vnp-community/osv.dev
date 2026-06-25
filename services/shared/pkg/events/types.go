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
	CVEID       string    `json:"cve_id"`
	WithdrawnAt time.Time `json:"withdrawn_at"`
	Reason      string    `json:"reason,omitempty"`
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
	JobID       string    `json:"job_id"`
	Target      string    `json:"target"`    // image name or directory path
	ScanType    string    `json:"scan_type"` // "image", "dir", "sbom"
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
	FindingID string    `json:"finding_id"`
	Title     string    `json:"title"`
	Severity  string    `json:"severity"` // "critical", "high", "medium", "low"
	CVEID     string    `json:"cve_id,omitempty"`
	AssetID   string    `json:"asset_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
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
