package usecase

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"

    "github.com/osv/scan-service/internal/domain/entity"
    "github.com/osv/scan-service/internal/scanner/nmap"
    "github.com/osv/scan-service/internal/scanner/zap"
)

// ScanRepository defines scan storage operations.
type ScanRepository interface {
    FindByID(ctx context.Context, id uuid.UUID) (*entity.Scan, error)
    Update(ctx context.Context, scan *entity.Scan) error
    Save(ctx context.Context, scan *entity.Scan) error
}

// ProgressPublisher sends progress updates (used by SSE).
type ProgressPublisher interface {
    Publish(scanID uuid.UUID, progress int, message string)
}

// EventBus publishes domain events.
type EventBus interface {
    Publish(ctx context.Context, subject string, data interface{}) error
}

// ExecuteScanInput is the input for starting a scan.
type ExecuteScanInput struct {
    ScanID uuid.UUID
}

// ExecuteScanUseCase orchestrates nmap/ZAP scans with lifecycle management.
type ExecuteScanUseCase struct {
    scanRepo  ScanRepository
    nmapScan  *nmap.Scanner
    zapScan   *zap.Scanner
    progress  ProgressPublisher
    eventBus  EventBus
    logger    zerolog.Logger
}

// NewExecuteScan creates the use-case.
func NewExecuteScan(
    scanRepo ScanRepository,
    nmapScan *nmap.Scanner,
    zapScan *zap.Scanner,
    progress ProgressPublisher,
    eventBus EventBus,
    logger zerolog.Logger,
) *ExecuteScanUseCase {
    return &ExecuteScanUseCase{
        scanRepo: scanRepo,
        nmapScan: nmapScan,
        zapScan:  zapScan,
        progress: progress,
        eventBus: eventBus,
        logger:   logger,
    }
}

// Execute runs the scan identified by input.ScanID.
func (uc *ExecuteScanUseCase) Execute(ctx context.Context, in ExecuteScanInput) error {
    // 1. Load scan record
    scan, err := uc.scanRepo.FindByID(ctx, in.ScanID)
    if err != nil {
        return fmt.Errorf("load scan %s: %w", in.ScanID, err)
    }

    // 2. Transition to Running
    if err := uc.transitionTo(ctx, scan, entity.ScanStatusRunning); err != nil {
        return err
    }

    uc.progress.Publish(scan.ID, 5, "Scan started")

    // 3. Execute scan based on type
    var execErr error
    switch scan.ScanType {
    case entity.ScanTypeFull, entity.ScanTypeDiscovery:
        execErr = uc.runNmapScan(ctx, scan)
    case entity.ScanTypeWeb:
        execErr = uc.runZAPScan(ctx, scan)
    default:
        execErr = fmt.Errorf("unsupported scan type: %s", scan.ScanType)
    }

    // 4. Handle result
    if execErr != nil {
        return uc.failScan(ctx, scan, execErr)
    }

    return uc.completeScan(ctx, scan)
}

func (uc *ExecuteScanUseCase) runNmapScan(ctx context.Context, scan *entity.Scan) error {
    uc.progress.Publish(scan.ID, 20, "Running Nmap scan")

    var findings []*nmap.ScanFinding
    var err error

    if scan.ScanType == entity.ScanTypeDiscovery {
        hosts, e := uc.nmapScan.DiscoveryScan(ctx, scan.ID.String(), scan.Targets)
        if e != nil {
            return e
        }
        uc.logger.Info().Int("hosts_found", len(hosts)).Msg("discovery scan done")
    } else {
        findings, err = uc.nmapScan.FullScan(ctx, scan.ID.String(), scan.Targets)
        if err != nil {
            return err
        }
    }

    uc.progress.Publish(scan.ID, 90, fmt.Sprintf("Nmap completed: %d findings", len(findings)))

    // Publish findings event
    uc.eventBus.Publish(ctx, "scan.scan.completed", map[string]interface{}{
        "scan_id":       scan.ID,
        "finding_count": len(findings),
    })

    return nil
}

func (uc *ExecuteScanUseCase) runZAPScan(ctx context.Context, scan *entity.Scan) error {
    uc.progress.Publish(scan.ID, 20, "Running ZAP spider")

    if len(scan.Targets) == 0 {
        return fmt.Errorf("no targets for web scan")
    }

    alerts, err := uc.zapScan.ActiveScan(ctx, scan.ID.String(), scan.Targets[0])
    if err != nil {
        return err
    }

    uc.progress.Publish(scan.ID, 90, fmt.Sprintf("ZAP completed: %d alerts", len(alerts)))

    uc.eventBus.Publish(ctx, "scan.scan.completed", map[string]interface{}{
        "scan_id":       scan.ID,
        "finding_count": len(alerts),
    })

    return nil
}

func (uc *ExecuteScanUseCase) transitionTo(ctx context.Context, scan *entity.Scan, status entity.ScanStatus) error {
    if !scan.Status.CanTransitionTo(status) {
        return fmt.Errorf("invalid transition: %s → %s", scan.Status, status)
    }
    now := time.Now().UTC()
    scan.Status = status
    if status == entity.ScanStatusRunning {
        scan.StartedAt = &now
    }
    scan.UpdatedAt = now
    return uc.scanRepo.Update(ctx, scan)
}

func (uc *ExecuteScanUseCase) completeScan(ctx context.Context, scan *entity.Scan) error {
    now := time.Now().UTC()
    scan.Status = entity.ScanStatusCompleted
    scan.CompletedAt = &now
    scan.Progress = 100
    scan.UpdatedAt = now
    uc.progress.Publish(scan.ID, 100, "Scan complete")
    return uc.scanRepo.Update(ctx, scan)
}

func (uc *ExecuteScanUseCase) failScan(ctx context.Context, scan *entity.Scan, reason error) error {
    now := time.Now().UTC()
    scan.Status = entity.ScanStatusFailed
    scan.FailedAt = &now
    scan.ErrorMsg = reason.Error()
    scan.UpdatedAt = now
    _ = uc.scanRepo.Update(ctx, scan)
    return reason
}
