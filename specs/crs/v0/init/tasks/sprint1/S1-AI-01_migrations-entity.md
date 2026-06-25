# S1-AI-01 — Tạo Migrations Directory + EnrichmentResult Entity (ai-service)

## ✅ Execution Status: COMPLETED
- **Executed**: 2026-06-13
- **Result**: `go build` PASSED


## Metadata
- **Task ID**: S1-AI-01
- **Service**: ai-service
- **Sprint**: 1 (P0)
- **Ước tính**: 1.5 giờ
- **Dependencies**: Không có
- **Spec nguồn**: `specs/develop/06_ai-service-upgrade.md` § "P0 — Thêm: Migrations" và "Thêm: Domain Entity"

## Context

```bash
# Đọc domain enrichment để biết existing types:
ls services/ai-service/internal/domain/enrichment/
cat services/ai-service/internal/domain/enrichment/provider_chain.go | head -50
cat services/ai-service/internal/domain/enrichment/embedding_service.go | head -30
cat services/ai-service/internal/domain/triage/entity.go

# Xem use case output types:
cat services/ai-service/internal/usecase/enrich_cve/handler.go
cat services/ai-service/internal/usecase/epss/job.go
```

## Goal

1. Tạo `migrations/` directory với 1 SQL file cho EPSS score tracking
2. Tạo `domain/enrichment/entity.go` với `EnrichmentResult` unified struct

## Files to Create

### File 1: `services/ai-service/migrations/001_epss_scores.sql`

```sql
-- 001_epss_scores.sql
-- EPSS score history tracking table
-- Enables: historical EPSS trend analysis, spike detection

CREATE TABLE IF NOT EXISTS epss_scores (
    cve_id      VARCHAR(30)      NOT NULL,
    score       DOUBLE PRECISION NOT NULL,
    percentile  DOUBLE PRECISION NOT NULL,
    fetched_at  TIMESTAMPTZ      NOT NULL,
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    PRIMARY KEY (cve_id, fetched_at)
);

-- Index for dashboard queries: "top N critical by EPSS"
CREATE INDEX IF NOT EXISTS idx_epss_score_desc
    ON epss_scores(score DESC, fetched_at DESC);

-- Index for CVE-specific history
CREATE INDEX IF NOT EXISTS idx_epss_cve_id
    ON epss_scores(cve_id, fetched_at DESC);

-- Latest EPSS view (simplifies queries)
CREATE OR REPLACE VIEW epss_latest AS
    SELECT DISTINCT ON (cve_id)
        cve_id, score, percentile, fetched_at
    FROM epss_scores
    ORDER BY cve_id, fetched_at DESC;
```

### File 2: `services/ai-service/internal/domain/enrichment/entity.go`

Đọc existing domain sub-types trước, sau đó tạo:

```go
package enrichment

import "time"

// EnrichmentResult is the unified output of all enrichment stages for a CVE.
// Sub-domain types (provider_chain, mitretagger, etc.) are defined in their own files.
// This struct aggregates results for storage and API responses.
type EnrichmentResult struct {
	// Core fields
	CVEID        string    `json:"cve_id"`
	EnrichedAt   time.Time `json:"enriched_at"`
	Provider     string    `json:"provider"`       // AI provider that succeeded
	ModelVersion string    `json:"model_version"`

	// AI-generated narrative
	SummaryShort     string `json:"summary_short"`
	SummaryLong      string `json:"summary_long"`
	ImpactAnalysis   string `json:"impact_analysis"`
	RemediationGuide string `json:"remediation_guide"`
	AttackVector     string `json:"attack_vector"`

	// Structured enrichment (pointers allow partial enrichment)
	EPSSScore          *EPSSSnapshot    `json:"epss,omitempty"`
	MITRETags          []MITRETagResult `json:"mitre_tags,omitempty"`
	SeverityML         string           `json:"severity_ml,omitempty"`
	SeverityConfidence float64          `json:"severity_confidence"`
	ExploitInfo        *ExploitResult   `json:"exploit_info,omitempty"`
	ThreatIntel        *ThreatIntelData `json:"threat_intel,omitempty"`

	// Embedding (stored separately if large)
	HasEmbedding bool      `json:"has_embedding"`
	EmbeddingModel string  `json:"embedding_model,omitempty"`
}

// EPSSSnapshot captures a point-in-time EPSS score.
type EPSSSnapshot struct {
	Score      float64   `json:"score"`
	Percentile float64   `json:"percentile"`
	FetchedAt  time.Time `json:"fetched_at"`
}

// MITRETagResult represents a MITRE ATT&CK technique mapping.
// Note: detailed types are in domain/enrichment/mitretagger/tagger.go — we reference them here.
type MITRETagResult struct {
	TechniqueID   string  `json:"technique_id"`
	TechniqueName string  `json:"technique_name"`
	TacticID      string  `json:"tactic_id"`
	Confidence    float64 `json:"confidence"`
}

// ExploitResult summarizes exploit availability.
// Note: detailed types are in domain/enrichment/exploit/checker.go.
type ExploitResult struct {
	HasPublicExploit bool   `json:"has_public_exploit"`
	ExploitURL       string `json:"exploit_url,omitempty"`
	ExploitType      string `json:"exploit_type,omitempty"` // PoC | weaponized | metasploit
}

// ThreatIntelData contains threat intelligence context.
type ThreatIntelData struct {
	InTheWild       bool     `json:"in_the_wild"`
	ThreatActors    []string `json:"threat_actors,omitempty"`
	AffectedSectors []string `json:"affected_sectors,omitempty"`
}

// EnrichmentRepository defines the storage interface for enrichment results.
// Implemented by: infra/persistence/firestore/ (existing), infra/persistence/mongo/ (new)
type EnrichmentRepository interface {
	Save(ctx context.Context, result *EnrichmentResult) error
	FindByCVEID(ctx context.Context, id string) (*EnrichmentResult, error)
	FindBatch(ctx context.Context, ids []string) ([]*EnrichmentResult, error)
	// ListUnenriched returns CVE IDs that have no enrichment yet
	ListUnenriched(ctx context.Context, limit int) ([]string, error)
}
```

**Note**: Thêm `"context"` import nếu chưa có.

## Verification

```bash
cd services/ai-service && go build ./...

# Check migrations directory:
ls services/ai-service/migrations/
# Expected: 001_epss_scores.sql

# Check entity file:
ls services/ai-service/internal/domain/enrichment/entity.go

# No compile errors expected
```

## Notes

- Nếu `MITRETag`, `ExploitInfo` đã có type definitions trong existing files, import và reuse — đừng tạo duplicate types
- Kiểm tra `domain/enrichment/mitretagger/tagger.go` và `exploit/checker.go` để tìm existing types trước khi define mới
- `EnrichmentRepository` interface ở đây là NEW — implement trong `infra/persistence/` files
