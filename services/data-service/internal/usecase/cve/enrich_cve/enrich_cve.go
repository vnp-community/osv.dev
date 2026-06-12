// Package enrichcve provides the CVE enrichment use case.
// Fetches CVE data from OSV.dev (primary) with NVD fallback, enriches with EPSS score.
package enrichcve

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/osv/data-service/internal/domain/entity"
	"github.com/osv/data-service/internal/domain/repository"
)

// Input defines the enrichment request parameters.
type Input struct {
	CVEIDs  []string
	ScanID  uuid.UUID
	AssetID uuid.UUID
}

// Output summarises the enrichment results.
type Output struct {
	Enriched []string // successfully enriched CVE IDs
	Failed   []string // CVE IDs that could not be enriched
}

// OSVClient fetches CVE data from OSV.dev.
type OSVClient interface {
	GetVulnerability(ctx context.Context, cveID string) (*entity.CVE, error)
}

// NVDClient fetches CVE data from NVD as a fallback.
type NVDClient interface {
	GetCVE(ctx context.Context, cveID string) (*entity.CVE, error)
}

// EPSSClient fetches EPSS probability scores.
type EPSSClient interface {
	GetScore(ctx context.Context, cveID string) (score, percentile float64, err error)
}

// Publisher publishes enrichment completion events.
type Publisher interface {
	PublishCVEEnriched(ctx context.Context, cveIDs []string, severity entity.Severity, scanID, assetID uuid.UUID) error
}

// UseCase orchestrates concurrent CVE enrichment with a semaphore.
type UseCase struct {
	cveRepo  repository.CVERepository
	osvClient OSVClient
	nvdClient NVDClient
	epssClient EPSSClient
	publisher Publisher
	maxConcurrent int // semaphore size
}

// NewUseCase creates an EnrichCVE use case.
func NewUseCase(
	cveRepo repository.CVERepository,
	osv OSVClient, nvd NVDClient, epss EPSSClient,
	publisher Publisher,
) *UseCase {
	return &UseCase{
		cveRepo: cveRepo, osvClient: osv, nvdClient: nvd,
		epssClient: epss, publisher: publisher, maxConcurrent: 10,
	}
}

// Execute enriches a list of CVE IDs concurrently (max 10 goroutines).
func (uc *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
	sem := make(chan struct{}, uc.maxConcurrent)
	var mu sync.Mutex
	out := &Output{}
	var wg sync.WaitGroup

	for _, cveID := range in.CVEIDs {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := uc.enrichOne(ctx, id, in.ScanID, in.AssetID); err != nil {
				mu.Lock(); out.Failed = append(out.Failed, id); mu.Unlock()
			} else {
				mu.Lock(); out.Enriched = append(out.Enriched, id); mu.Unlock()
			}
		}(cveID)
	}
	wg.Wait()
	return out, nil
}

func (uc *UseCase) enrichOne(ctx context.Context, cveID string, scanID, assetID uuid.UUID) error {
	// 1. Check DB freshness (24h)
	existing, err := uc.cveRepo.FindByCVEID(ctx, cveID)
	if err == nil && time.Since(existing.LastFetchedAt) < 24*time.Hour {
		return nil // fresh enough
	}

	// 2. Primary: OSV.dev
	cve, err := uc.osvClient.GetVulnerability(ctx, cveID)
	if err != nil || cve == nil {
		// 3. Fallback: NVD
		cve, err = uc.nvdClient.GetCVE(ctx, cveID)
		if err != nil || cve == nil {
			return fmt.Errorf("enrich %s: both OSV and NVD failed: %w", cveID, err)
		}
	}

	// 4. EPSS score
	if uc.epssClient != nil {
		score, pct, e := uc.epssClient.GetScore(ctx, cveID)
		if e == nil {
			cve.EPSS = score
			cve.EPSSPercentile = pct
			cve.EPSSScore = score
			cve.EPSSPctile = pct
		}
	}

	// 5. Derive severity from CVSS if not set
	if cve.Severity == "" {
		cve.Severity = entity.SeverityFromCVSS(cve.CVSSv3Score)
	}

	// 6. Persist
	if err := uc.cveRepo.Upsert(ctx, cve); err != nil {
		return fmt.Errorf("upsert %s: %w", cveID, err)
	}

	// 7. Publish
	if uc.publisher != nil {
		uc.publisher.PublishCVEEnriched(ctx, []string{cveID}, cve.Severity, scanID, assetID) //nolint:errcheck
	}
	return nil
}
