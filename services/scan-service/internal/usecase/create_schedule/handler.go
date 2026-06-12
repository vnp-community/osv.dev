package create_schedule

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/osv/scan-service/internal/domain/schedule"
	
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// Command carries the input for creating a new schedule.
type Command struct {
	Name        string
	Description string
	CronExpr    string
	Type        schedule.ScheduleType
	TargetIDs   []string
}

// Handler orchestrates the creation of a new Schedule.
type Handler struct {
	repo schedule.Repository
}

// NewHandler creates a new create_schedule handler.
func NewHandler(repo schedule.Repository) *Handler {
	return &Handler{repo: repo}
}

// Handle validates and persists a new schedule.
func (h *Handler) Handle(ctx context.Context, cmd Command) (*schedule.Schedule, error) {
	// Validate cron expression upfront.
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	cronSched, err := parser.Parse(cmd.CronExpr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression %q: %w", cmd.CronExpr, err)
	}

	nextRun := cronSched.Next(time.Now())
	now := time.Now()

	s := &schedule.Schedule{
		ID:          uuid.New(),
		Name:        cmd.Name,
		Description: cmd.Description,
		CronExpr:    cmd.CronExpr,
		Type:        cmd.Type,
		TargetIDs:   cmd.TargetIDs,
		Status:      schedule.ScheduleStatusActive,
		NextRunAt:   &nextRun,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.repo.Save(ctx, s); err != nil {
		return nil, fmt.Errorf("create_schedule: save: %w", err)
	}

	log.Info().
		Str("schedule_id", s.ID.String()).
		Str("cron", s.CronExpr).
		Time("next_run", nextRun).
		Msg("schedule created")

	return s, nil
}
