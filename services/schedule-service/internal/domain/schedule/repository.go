package schedule

import (
	"context"

	"github.com/osv/schedule-service/internal/domain/schedule"
)

// Repository defines the persistence contract for Schedule aggregates.
type Repository interface {
	Save(ctx context.Context, s *schedule.Schedule) error
	FindByID(ctx context.Context, id string) (*schedule.Schedule, error)
	FindAll(ctx context.Context) ([]*schedule.Schedule, error)
	FindActive(ctx context.Context) ([]*schedule.Schedule, error)
	Delete(ctx context.Context, id string) error
}
