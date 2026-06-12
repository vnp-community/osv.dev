// Package executescan provides the core scan execution use case.
package executescan

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/osv/scan-service/internal/adapters/scanner/nmap"
	"github.com/osv/scan-service/internal/adapters/scanner/zap"
	"github.com/osv/scan-service/internal/domain/entity"
	"github.com/osv/scan-service/internal/domain/repository"
	"github.com/rs/zerolog"
)

// AssetGRPCClient upserts finding data into the asset service.
type AssetGRPCClient interface {
	UpsertAsset(ctx context.Context, finding *entity.Finding) (assetID string, created bool, err error)
}

// CVEGRPCClient enriches CVE data for given IDs.
type CVEGRPCClient interface {
	EnrichCVEs(ctx context.Context, cveIDs []string, scanID, assetID uuid.UUID) error
}

// Publisher publishes scan lifecycle events.
type Publisher interface {
	PublishScanStarted(ctx context.Context, scanID uuid.UUID) error
	PublishScanCompleted(ctx context.Context, scanID uuid.UUID, findingCount int) error
	PublishScanFailed(ctx context.Context, scanID uuid.UUID, errMsg string) error
	PublishFindingDiscovered(ctx context.Context, finding *entity.Finding) error
	PublishProgress(ctx context.Context, scanID uuid.UUID, progress int) error
}

// UseCase orchestrates scan execution.
type UseCase struct {
	scanRepo    repository.ScanRepository
	findingRepo repository.FindingRepository
	alertRepo   repository.WebAlertRepository
	nmapScanner *nmap.NmapScanner
	zapScanner  *zap.ZAPScanner
	assetClient AssetGRPCClient
	cveClient   CVEGRPCClient
	publisher   Publisher
	log         zerolog.Logger
}

// NewUseCase creates an ExecuteScan use case.
func NewUseCase(
	scanRepo repository.ScanRepository,
	findingRepo repository.FindingRepository,
	alertRepo repository.WebAlertRepository,
	nmapScanner *nmap.NmapScanner,
	zapScanner *zap.ZAPScanner,
	assetClient AssetGRPCClient,
	cveClient CVEGRPCClient,
	publisher Publisher,
	log zerolog.Logger,
) *UseCase {
	return &UseCase{
		scanRepo: scanRepo, findingRepo: findingRepo, alertRepo: alertRepo,
		nmapScanner: nmapScanner, zapScanner: zapScanner,
		assetClient: assetClient, cveClient: cveClient,
		publisher: publisher, log: log,
	}
}

// Execute runs the scan and updates its status throughout.
func (uc *UseCase) Execute(ctx context.Context, scanID uuid.UUID) error {
	scan, err := uc.scanRepo.FindByID(ctx, scanID)
	if err != nil {
		return fmt.Errorf("load scan: %w", err)
	}

	// Mark running
	now := time.Now().UTC()
	scan.StartedAt = &now
	if err := uc.scanRepo.UpdateStatus(ctx, scanID, entity.ScanStatusRunning); err != nil {
		return err
	}
	uc.publisher.PublishScanStarted(ctx, scanID) //nolint:errcheck

	var execErr error
	switch scan.ScanType {
	case entity.ScanTypeFull, entity.ScanTypeDiscovery:
		execErr = uc.runNmapScan(ctx, scan)
	case entity.ScanTypeWeb:
		execErr = uc.runWebScan(ctx, scan)
	default:
		execErr = fmt.Errorf("unsupported scan type: %s", scan.ScanType)
	}

	if execErr != nil {
		uc.scanRepo.UpdateStatus(ctx, scanID, entity.ScanStatusFailed) //nolint:errcheck
		uc.publisher.PublishScanFailed(ctx, scanID, execErr.Error())   //nolint:errcheck
		return execErr
	}

	uc.scanRepo.UpdateStatus(ctx, scanID, entity.ScanStatusCompleted) //nolint:errcheck
	uc.scanRepo.UpdateProgress(ctx, scanID, 100)                      //nolint:errcheck
	uc.publisher.PublishScanCompleted(ctx, scanID, scan.FindingCount)  //nolint:errcheck
	return nil
}

func (uc *UseCase) runNmapScan(ctx context.Context, scan *entity.Scan) error {
	total := len(scan.Targets)
	var allFindings []*entity.Finding

	for i, target := range scan.Targets {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var findings []*entity.Finding
		var err error

		if scan.ScanType == entity.ScanTypeDiscovery {
			hosts, e := uc.nmapScanner.RunDiscovery(ctx, target)
			if e != nil {
				uc.log.Warn().Err(e).Str("target", target).Msg("discovery failed")
				continue
			}
			// Convert hosts to findings stubs
			for _, h := range hosts {
				findings = append(findings, &entity.Finding{
					ID: uuid.New(), ScanID: scan.ID,
					IPAddress: h.IPAddress, Hostname: h.Hostname,
				})
			}
		} else {
			findings, err = uc.nmapScanner.RunFull(ctx, target, scan.Options)
			if err != nil {
				uc.log.Warn().Err(err).Str("target", target).Msg("nmap scan failed")
				continue
			}
		}

		// Assign scan ID to each finding
		for _, f := range findings {
			f.ScanID = scan.ID
		}

		// Persist + enrich
		if len(findings) > 0 {
			uc.findingRepo.CreateBatch(ctx, findings) //nolint:errcheck
			allFindings = append(allFindings, findings...)
			for _, f := range findings {
				uc.enrichFinding(ctx, scan.ID, f)
			}
		}

		progress := (i + 1) * 100 / total
		uc.scanRepo.UpdateProgress(ctx, scan.ID, progress) //nolint:errcheck
		uc.publisher.PublishProgress(ctx, scan.ID, progress) //nolint:errcheck
	}

	scan.FindingCount = len(allFindings)
	return nil
}

func (uc *UseCase) runWebScan(ctx context.Context, scan *entity.Scan) error {
	for _, target := range scan.Targets {
		cfg := zap.ZAPConfig{
			SpiderTimeout:     5 * time.Minute,
			ActiveScanTimeout: 10 * time.Minute,
		}
		alerts, err := uc.zapScanner.Scan(ctx, scan.ID, target, cfg)
		if err != nil {
			return err
		}
		if len(alerts) > 0 {
			uc.alertRepo.CreateBatch(ctx, alerts) //nolint:errcheck
		}
		scan.FindingCount += len(alerts)
	}
	return nil
}

func (uc *UseCase) enrichFinding(ctx context.Context, scanID uuid.UUID, f *entity.Finding) {
	// 1. Upsert to asset-service
	assetID, _, err := uc.assetClient.UpsertAsset(ctx, f)
	if err != nil {
		uc.log.Warn().Err(err).Msg("upsert asset failed")
		return
	}
	assetUUID, _ := uuid.Parse(assetID)

	// 2. Enrich CVEs
	if len(f.CVEIDs) > 0 && uc.cveClient != nil {
		uc.cveClient.EnrichCVEs(ctx, f.CVEIDs, scanID, assetUUID) //nolint:errcheck
	}

	// 3. Publish finding event
	uc.publisher.PublishFindingDiscovered(ctx, f) //nolint:errcheck
}
