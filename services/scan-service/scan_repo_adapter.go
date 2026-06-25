package scan

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/osv/scan-service/internal/adapters/repository/postgres"
	"github.com/osv/scan-service/internal/domain/entity"
	"github.com/osv/scan-service/internal/domain/repository"
)

// scanRepoAdapter wraps postgres.ScanRepo to satisfy the delivery/http.ScanRepository
// interface (which uses raw interface{} slices for decoupling from domain types).
type scanRepoAdapter struct {
	inner *postgres.ScanRepo
}

// newScanRepoAdapter creates an adapter wrapping a real postgres ScanRepo.
func newScanRepoAdapter(r *postgres.ScanRepo) *scanRepoAdapter {
	return &scanRepoAdapter{inner: r}
}

// ListRaw implements delivery/http.ScanRepository.
func (a *scanRepoAdapter) ListRaw(ctx interface{}, page, pageSize int, status string) ([]interface{}, int64, error) {
	c, ok := ctx.(context.Context)
	if !ok {
		c = context.Background()
	}
	var statusPtr *entity.ScanStatus
	if status != "" {
		s := entity.ScanStatus(status)
		statusPtr = &s
	}
	scans, total, err := a.inner.List(c, repository.ScanFilter{
		Status:   statusPtr,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]interface{}, len(scans))
	for i, s := range scans {
		b, _ := json.Marshal(s)
		var m map[string]interface{}
		json.Unmarshal(b, &m) //nolint:errcheck
		out[i] = m
	}
	return out, total, nil
}

// FindByIDRaw implements delivery/http.ScanRepository.
func (a *scanRepoAdapter) FindByIDRaw(ctx interface{}, id string) (interface{}, error) {
	c, ok := ctx.(context.Context)
	if !ok {
		c = context.Background()
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid scan id %q: %w", id, err)
	}
	s, err := a.inner.FindByID(c, uid)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}
	b, _ := json.Marshal(s)
	var m map[string]interface{}
	json.Unmarshal(b, &m) //nolint:errcheck
	return m, nil
}

// CancelRaw implements delivery/http.ScanRepository.
func (a *scanRepoAdapter) CancelRaw(ctx interface{}, id string) error {
	c, ok := ctx.(context.Context)
	if !ok {
		c = context.Background()
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid scan id %q: %w", id, err)
	}
	return a.inner.UpdateStatus(c, uid, entity.ScanStatusCancelled)
}
