// Package createscan provides the use case for creating a new scan job.
package createscan

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/osv/scan-service/internal/domain/entity"
	"github.com/osv/scan-service/internal/domain/repository"
)

var (
	hostnameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-\.]{0,253}[a-zA-Z0-9]$`)
	urlRE      = regexp.MustCompile(`^https?://`)
)

// Request is input for creating a scan.
type Request struct {
	UserID   uuid.UUID
	Targets  []string
	ScanType entity.ScanType
	Priority int
	Options  entity.ScanOptions
}

// Response is returned on successful scan creation.
type Response struct {
	ScanID   uuid.UUID
	Status   entity.ScanStatus
	Targets  []string
	ScanType entity.ScanType
}

// Publisher defines the event publishing interface.
type Publisher interface {
	PublishScanCreated(ctx context.Context, scan *entity.Scan) error
}

// UseCase orchestrates scan creation.
type UseCase struct {
	scanRepo repository.ScanRepository
	publisher Publisher
}

// NewUseCase creates a CreateScan use case.
func NewUseCase(scanRepo repository.ScanRepository, publisher Publisher) *UseCase {
	return &UseCase{scanRepo: scanRepo, publisher: publisher}
}

// Execute validates, persists, and publishes a new scan.
func (uc *UseCase) Execute(ctx context.Context, req Request) (*Response, error) {
	if err := validateTargets(req.Targets, req.ScanType); err != nil {
		return nil, err
	}

	priority := req.Priority
	if priority < 1 || priority > 10 {
		priority = 5
	}

	scan := &entity.Scan{
		ID:        uuid.New(),
		UserID:    req.UserID,
		Targets:   req.Targets,
		ScanType:  req.ScanType,
		Status:    entity.ScanStatusPending,
		Priority:  priority,
		Options:   req.Options,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := uc.scanRepo.Create(ctx, scan); err != nil {
		return nil, fmt.Errorf("create scan: %w", err)
	}

	// Publish event (non-blocking best-effort)
	if uc.publisher != nil {
		uc.publisher.PublishScanCreated(ctx, scan) //nolint:errcheck
	}

	return &Response{
		ScanID:   scan.ID,
		Status:   scan.Status,
		Targets:  scan.Targets,
		ScanType: scan.ScanType,
	}, nil
}

func validateTargets(targets []string, scanType entity.ScanType) error {
	if len(targets) == 0 {
		return fmt.Errorf("at least one target is required")
	}
	if len(targets) > 100 {
		return fmt.Errorf("maximum 100 targets per scan")
	}
	for _, t := range targets {
		t = strings.TrimSpace(t)
		if t == "" {
			return fmt.Errorf("empty target not allowed")
		}
		if scanType == entity.ScanTypeWeb && !urlRE.MatchString(t) {
			return fmt.Errorf("web scan targets must be URLs (http:// or https://): %s", t)
		}
		if scanType != entity.ScanTypeWeb {
			// Validate IP, CIDR, or hostname
			if net.ParseIP(t) != nil {
				continue // valid IP
			}
			if _, _, err := net.ParseCIDR(t); err == nil {
				continue // valid CIDR
			}
			if hostnameRE.MatchString(t) {
				continue // valid hostname
			}
			return fmt.Errorf("invalid target (not IP, CIDR, or hostname): %s", t)
		}
	}
	return nil
}
