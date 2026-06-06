package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/schedule-service/internal/domain/entity"
)

// ScheduleRepo implements repository.ScheduleRepository.
type ScheduleRepo struct{ db *pgxpool.Pool }

func NewScheduleRepo(db *pgxpool.Pool) *ScheduleRepo { return &ScheduleRepo{db: db} }

func (r *ScheduleRepo) Create(ctx context.Context, s *entity.ScheduledScan) error {
	if s.ID == uuid.Nil { s.ID = uuid.New() }
	s.CreatedAt = time.Now().UTC()
	targets, _ := json.Marshal(s.Targets)
	opts, _ := json.Marshal(s.Options)
	_, err := r.db.Exec(ctx, `
		INSERT INTO schedule.scheduled_scans
		  (id,user_id,targets,scan_type,cron_expr,interval_minutes,status,next_run_at,options,created_at)
		VALUES ($1,$2,$3::jsonb,$4,$5,$6,$7,$8,$9::jsonb,$10)`,
		s.ID, s.UserID, targets, s.ScanType,
		nilStr(s.CronExpr), nilInt(s.IntervalMinutes),
		s.Status, s.NextRunAt, opts, s.CreatedAt,
	)
	return err
}

func (r *ScheduleRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.ScheduledScan, error) {
	var s entity.ScheduledScan
	var targets, opts []byte
	err := r.db.QueryRow(ctx, `
		SELECT id,user_id,targets,scan_type,cron_expr,interval_minutes,status,next_run_at,last_run_at,options,created_at
		FROM schedule.scheduled_scans WHERE id=$1`, id).Scan(
		&s.ID, &s.UserID, &targets, &s.ScanType,
		&s.CronExpr, &s.IntervalMinutes, &s.Status,
		&s.NextRunAt, &s.LastRunAt, &opts, &s.CreatedAt,
	)
	if err != nil { return nil, err }
	json.Unmarshal(targets, &s.Targets)
	json.Unmarshal(opts, &s.Options)
	return &s, nil
}

// FindDue returns active schedules whose next_run_at is <= now.
func (r *ScheduleRepo) FindDue(ctx context.Context, now time.Time) ([]*entity.ScheduledScan, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id,user_id,targets,scan_type,cron_expr,interval_minutes,status,next_run_at,options
		FROM schedule.scheduled_scans
		WHERE status='active' AND next_run_at <= $1`, now)
	if err != nil { return nil, err }
	defer rows.Close()
	var schedules []*entity.ScheduledScan
	for rows.Next() {
		var s entity.ScheduledScan
		var targets, opts []byte
		rows.Scan(&s.ID, &s.UserID, &targets, &s.ScanType, &s.CronExpr, &s.IntervalMinutes, &s.Status, &s.NextRunAt, &opts)
		json.Unmarshal(targets, &s.Targets)
		json.Unmarshal(opts, &s.Options)
		schedules = append(schedules, &s)
	}
	return schedules, rows.Err()
}

func (r *ScheduleRepo) UpdateNextRun(ctx context.Context, id uuid.UUID, nextRun time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE schedule.scheduled_scans SET next_run_at=$2, last_run_at=NOW() WHERE id=$1`, id, nextRun)
	return err
}

func (r *ScheduleRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE schedule.scheduled_scans SET status=$2 WHERE id=$1`, id, status)
	return err
}

func (r *ScheduleRepo) List(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*entity.ScheduledScan, int64, error) {
	if page < 1 { page = 1 }; if pageSize < 1 { pageSize = 20 }
	rows, err := r.db.Query(ctx, `
		SELECT id,user_id,scan_type,status,next_run_at,last_run_at,created_at
		FROM schedule.scheduled_scans WHERE user_id=$1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, pageSize, (page-1)*pageSize)
	if err != nil { return nil, 0, err }
	defer rows.Close()
	var schedules []*entity.ScheduledScan
	for rows.Next() {
		var s entity.ScheduledScan
		rows.Scan(&s.ID, &s.UserID, &s.ScanType, &s.Status, &s.NextRunAt, &s.LastRunAt, &s.CreatedAt)
		schedules = append(schedules, &s)
	}
	return schedules, int64(len(schedules)), rows.Err()
}

func (r *ScheduleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM schedule.scheduled_scans WHERE id=$1`, id)
	return err
}

func nilStr(s string) *string {
	if s == "" { return nil }; return &s
}

func nilInt(i int) *int {
	if i == 0 { return nil }; return &i
}
