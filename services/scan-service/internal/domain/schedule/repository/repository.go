package repository

import (
	"context"
	"time"
	"github.com/google/uuid"
	"github.com/osv/scan-service/internal/domain/schedule/entity"
)

// ScheduleRepository defines persistence for scheduled scans.
type ScheduleRepository interface {
	Create(ctx context.Context, s *entity.ScheduledScan) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.ScheduledScan, error)
	// FindDue returns active schedules where next_run_at <= now.
	FindDue(ctx context.Context, now time.Time) ([]*entity.ScheduledScan, error)
	UpdateNextRun(ctx context.Context, id uuid.UUID, nextRun time.Time) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	List(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*entity.ScheduledScan, int64, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
