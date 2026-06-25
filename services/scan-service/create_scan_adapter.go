package scan

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	pgadapter "github.com/osv/scan-service/internal/adapters/repository/postgres"
	httpdelivery "github.com/osv/scan-service/internal/delivery/http"
	"github.com/osv/scan-service/internal/domain/entity"
)

// createScanUCAdapter wraps postgres.ScanRepo to satisfy the delivery/http.CreateScanUseCase interface.
// This avoids importing the higher-level createscan package from the delivery layer.
type createScanUCAdapter struct {
	repo *pgadapter.ScanRepo
}

// newCreateScanUCAdapter creates an adapter wrapping a real postgres ScanRepo.
func newCreateScanUCAdapter(r *pgadapter.ScanRepo) *createScanUCAdapter {
	return &createScanUCAdapter{repo: r}
}

// Execute implements delivery/http.CreateScanUseCase.
// Converts the raw delivery request into a domain entity and persists it.
func (a *createScanUCAdapter) Execute(ctx context.Context, userID uuid.UUID, req httpdelivery.CreateScanRequest) (map[string]interface{}, error) {
	scanType := httpdelivery.ParseScanType(req.Type)

	// Parse ScanOptions from generic map
	opts := entity.ScanOptions{}
	if req.Options != nil {
		if pr, ok := req.Options["port_range"].(string); ok {
			opts.Ports = pr
		}
		if t, ok := req.Options["timeout"]; ok {
			switch v := t.(type) {
			case float64:
				opts.Timeout = int(v)
			case int:
				opts.Timeout = v
			}
		}
	}

	priority := req.Priority
	if priority < 1 || priority > 10 {
		priority = 5
	}

	scan := &entity.Scan{
		ID:        uuid.New(),
		UserID:    userID,
		Targets:   req.Targets,
		ScanType:  scanType,
		Status:    entity.ScanStatusPending,
		Priority:  priority,
		Options:   opts,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := a.repo.Create(ctx, scan); err != nil {
		return nil, fmt.Errorf("failed to create scan: %w", err)
	}

	// Return explicit map with snake_case keys (entity has db: tags only, no json: tags)
	result := map[string]interface{}{
		"id":         scan.ID.String(),
		"user_id":    scan.UserID.String(),
		"targets":    scan.Targets,
		"scan_type":  string(scan.ScanType),
		"status":     string(scan.Status),
		"priority":   scan.Priority,
		"created_at": scan.CreatedAt.Format(time.RFC3339),
		"updated_at": scan.UpdatedAt.Format(time.RFC3339),
	}
	return result, nil
}
